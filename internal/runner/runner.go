package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/peng225/oval/internal/object"
	"github.com/peng225/oval/internal/s3client"
	"github.com/peng225/oval/internal/stat"
	"github.com/pkg/profile"
)

const (
	maxWorkerID = 0x10000
)

type ExecutionContext struct {
	Endpoint      string   `json:"endpoint"`
	BucketNames   []string `json:"bucketNames"`
	NumObj        int      `json:"numObj"`
	NumWorker     int      `json:"numWorker"`
	MinSize       int      `json:"minSize"`
	MaxSize       int      `json:"maxSize"`
	StartWorkerID int      `json:"startWorkerID"`
	Workers       []Worker `json:"workers"`
}

type Runner struct {
	execContext     *ExecutionContext
	opeRatio        []float64
	timeInMs        int64
	profiler        bool
	loadFileName    string
	client          *s3client.S3Client
	st              stat.Stat
	runnerID        int
	multipartThresh int
	caCertFileName  string
}

func NewRunner(execContext *ExecutionContext, opeRatio []float64, timeInMs int64,
	profiler bool, loadFileName string, processID, multipartThresh int,
	caCertFileName string) *Runner {
	if len(execContext.BucketNames) == 0 {
		slog.Error("bucket list is empty.")
		os.Exit(1)
	}
	runner := &Runner{
		execContext:     execContext,
		opeRatio:        opeRatio,
		timeInMs:        timeInMs,
		profiler:        profiler,
		loadFileName:    loadFileName,
		runnerID:        processID,
		multipartThresh: multipartThresh,
		caCertFileName:  caCertFileName,
	}
	runner.init()
	return runner
}

func NewRunnerFromLoadFile(loadFileName string, opeRatio []float64, timeInMs int64,
	profiler bool, multipartThresh int, caCertFileName string) *Runner {
	if loadFileName == "" {
		log.Fatal("loadFileName is empty.")
	}
	_, err := os.Stat(loadFileName)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	ec := loadSavedContext(loadFileName)
	return NewRunner(ec, opeRatio, timeInMs, profiler, loadFileName, 0, multipartThresh, caCertFileName)
}

func loadSavedContext(loadFileName string) *ExecutionContext {
	f, err := os.Open(loadFileName)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer f.Close()
	savedContext, err := io.ReadAll(f)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	ec := &ExecutionContext{}
	err = json.Unmarshal(savedContext, ec)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	return ec
}

func (r *Runner) init() {
	r.client = s3client.NewS3Client(r.execContext.Endpoint, r.caCertFileName, r.multipartThresh)
	if r.loadFileName == "" {
		r.execContext.Workers = make([]Worker, r.execContext.NumWorker)
		r.execContext.StartWorkerID = rand.Intn(maxWorkerID)
	}
	for i := range r.execContext.Workers {
		r.execContext.Workers[i].id = (r.execContext.StartWorkerID + i) % maxWorkerID
		r.execContext.Workers[i].minSize = r.execContext.MinSize
		r.execContext.Workers[i].maxSize = r.execContext.MaxSize
		if r.loadFileName == "" {
			r.execContext.Workers[i].BucketsWithObject = make([]*BucketWithObject, len(r.execContext.BucketNames))
			for j, bucketName := range r.execContext.BucketNames {
				r.execContext.Workers[i].BucketsWithObject[j] = &BucketWithObject{
					BucketName: bucketName,
					ObjectMeta: object.NewObjectMeta(
						r.execContext.NumObj/r.execContext.NumWorker,
						(int64(r.runnerID)<<32)+(int64(i)<<24)),
				}
			}
		} else {
			for j := range r.execContext.Workers[i].BucketsWithObject {
				r.execContext.Workers[i].BucketsWithObject[j].ObjectMeta.TidyUp()
			}
		}
		r.execContext.Workers[i].client = r.client
		r.execContext.Workers[i].st = &r.st
		r.execContext.Workers[i].logger = slog.Default().With("runnerID", r.runnerID,
			"workerID", fmt.Sprintf("%#x", r.execContext.Workers[i].id))
		r.execContext.Workers[i].ShowInfo()
	}
}

func (r *Runner) InitBucket(ctx context.Context) error {
	for _, bucketName := range r.execContext.BucketNames {
		err := r.client.HeadBucket(ctx, bucketName)
		if err != nil {
			if errors.Is(err, s3client.ErrNotFound) {
				if r.loadFileName != "" {
					return fmt.Errorf(`head bucket failed despite "load" parameter was set`)
				}
				slog.Info("Bucket not found. Creating...", "bucket", bucketName)
				err = r.client.CreateBucket(ctx, bucketName)
				if err != nil {
					// Bucket creation may be executed by multiple follower processes.
					if errors.Is(err, s3client.ErrConflict) {
						slog.Info("Bucket already exists.", "bucket", bucketName)
					} else {
						return err
					}
				} else {
					slog.Info("Bucket created successfully.")
				}
			} else {
				return err
			}
		} else {
			if r.loadFileName == "" {
				slog.Info("Clearing bucket.", "bucket", bucketName)
				err = r.client.ClearBucket(ctx, bucketName, fmt.Sprintf("%s%02x", object.KeyShortPrefix, r.runnerID))
				if err != nil {
					return err
				}
				slog.Info("Bucket cleared successfully.")
			}
		}
	}
	return nil
}

func (r *Runner) Run(ctx context.Context) error {
	slog.Info("Validation start.")
	if r.profiler {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}
	wg := &sync.WaitGroup{}
	now := time.Now()
	var err error
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i := 0; i < r.execContext.NumWorker; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for err == nil && (r.timeInMs == 0 || time.Since(now).Milliseconds() < r.timeInMs) {
				select {
				case <-ctx.Done():
					slog.Warn("Workload was canceled.")
					return
				default:
				}

				operation := r.selectOperation()
				switch operation {
				case Put:
					err = r.execContext.Workers[workerID].Put(ctx)
				case Get:
					err = r.execContext.Workers[workerID].Get(ctx)
				case Delete:
					err = r.execContext.Workers[workerID].Delete(ctx)
				case List:
					err = r.execContext.Workers[workerID].List(ctx)
				}
				if err != nil {
					cancel()
					return
				}
			}
		}(i)
	}
	wg.Wait()
	slog.Info("Validation finished.")
	r.st.Report()

	if err != nil {
		return err
	}

	return nil
}

type Operation int

const (
	Put Operation = iota
	Get
	Delete
	List
	NumOperation
)

func (r *Runner) selectOperation() Operation {
	randVal := rand.Float64()
	if randVal < r.opeRatio[0] {
		return Put
	} else if randVal < r.opeRatio[0]+r.opeRatio[1] {
		return Get
	} else if randVal < r.opeRatio[0]+r.opeRatio[1]+r.opeRatio[2] {
		return Delete
	} else {
		return List
	}
}

func (r *Runner) SaveContext(saveFileName string) error {
	f, err := os.Create(saveFileName)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(r.execContext)
	if err != nil {
		return err
	}
	for {
		n, err := f.Write(data)
		if err != nil {
			if n != len(data) {
				data = data[:n]
				continue
			}
			return err
		}
		break
	}
	return nil
}
