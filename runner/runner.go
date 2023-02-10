package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/peng225/oval/object"
	"github.com/peng225/oval/s3_client"
	"github.com/peng225/oval/stat"
	"github.com/pkg/profile"
)

const (
	maxWorkerID = 0x10000
)

type ExecutionContext struct {
	Endpoint      string   `json:"endpoint"`
	BucketNames   []string `json:"bucketNames"`
	NumObj        int64    `json:"numObj"`
	NumWorker     int      `json:"numWorker"`
	MinSize       int      `json:"minSize"`
	MaxSize       int      `json:"maxSize"`
	StartWorkerID int      `json:"startWorkerID"`
	Workers       []Worker `json:"workers"`
}

type Runner struct {
	execContext  *ExecutionContext
	opeRatio     []float64
	timeInMs     int64
	profiler     bool
	loadFileName string
	client       *s3_client.S3Client
	st           stat.Stat
	processID    int
}

func NewRunner(execContext *ExecutionContext, opeRatio []float64, timeInMs int64,
	profiler bool, loadFileName string, processID int) *Runner {
	runner := &Runner{
		execContext:  execContext,
		opeRatio:     opeRatio,
		timeInMs:     timeInMs,
		profiler:     profiler,
		loadFileName: loadFileName,
		processID:    processID,
	}
	runner.init()
	return runner
}

func NewRunnerFromLoadFile(loadFileName string, opeRatio []float64, timeInMs int64, profiler bool) *Runner {
	if loadFileName == "" {
		log.Fatal("loadFileName is empty.")
	}
	_, err := os.Stat(loadFileName)
	if err != nil {
		log.Fatal(err)
	}
	ec := loadSavedContext(loadFileName)
	return NewRunner(ec, opeRatio, timeInMs, profiler, loadFileName, 0)
}

func loadSavedContext(loadFileName string) *ExecutionContext {
	f, err := os.Open(loadFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	savedContext, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	ec := &ExecutionContext{}
	json.Unmarshal(savedContext, ec)
	return ec
}

func (r *Runner) init() {
	r.client = s3_client.NewS3Client(r.execContext.Endpoint)
	for _, bucketName := range r.execContext.BucketNames {
		err := r.client.HeadBucket(bucketName)
		if err != nil {
			var nf *s3_client.NotFound
			if errors.As(err, &nf) {
				if r.loadFileName != "" {
					log.Fatal("HeadBucket failed despite \"load\" parameter was set.")
				}
				log.Println("Bucket \"" + bucketName + "\" not found. Creating...")
				err = r.client.CreateBucket(bucketName)
				if err != nil {
					log.Fatal(err)
				}
				log.Println("Bucket created successfully.")
			} else {
				log.Fatal(err)
			}
		} else {
			if r.loadFileName == "" {
				err = r.client.ClearBucket(bucketName, fmt.Sprintf("%s%02x", object.KeyPrefix, r.processID))
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	if r.loadFileName == "" {
		r.execContext.Workers = make([]Worker, r.execContext.NumWorker)
		rand.Seed(time.Now().UnixNano())
		startID := rand.Intn(maxWorkerID)
		r.execContext.StartWorkerID = startID
		for i := range r.execContext.Workers {
			r.execContext.Workers[i].id = (startID + i) % maxWorkerID
			r.execContext.Workers[i].minSize = r.execContext.MinSize
			r.execContext.Workers[i].maxSize = r.execContext.MaxSize
			r.execContext.Workers[i].BucketsWithObject = make([]*BucketWithObject, len(r.execContext.BucketNames))
			for j, bucketName := range r.execContext.BucketNames {
				r.execContext.Workers[i].BucketsWithObject[j] = &BucketWithObject{
					BucketName: bucketName,
					ObjectMata: object.NewObjectMeta(
						r.execContext.NumObj/int64(r.execContext.NumWorker),
						(int64(i)<<24)+(int64(r.processID)<<32)),
				}
			}
			r.execContext.Workers[i].client = r.client
			r.execContext.Workers[i].st = &r.st
			r.execContext.Workers[i].ShowInfo()
		}
	} else {
		for i, _ := range r.execContext.Workers {
			r.execContext.Workers[i].id = (r.execContext.StartWorkerID + i) % maxWorkerID
			r.execContext.Workers[i].minSize = r.execContext.MinSize
			r.execContext.Workers[i].maxSize = r.execContext.MaxSize
			r.execContext.Workers[i].client = r.client
			r.execContext.Workers[i].st = &r.st
			r.execContext.Workers[i].ShowInfo()
		}
	}
}

func (r *Runner) Run(cancel chan struct{}) error {
	log.Println("Validation start.")
	if r.profiler {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}
	wg := &sync.WaitGroup{}
	now := time.Now()
	errCh := make(chan error, r.execContext.NumWorker)
	errOccurred := false
	for i := 0; i < r.execContext.NumWorker; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			var err error
			for !errOccurred && time.Since(now).Milliseconds() < r.timeInMs {
				select {
				case <-cancel:
					errMsg := "Workload was canceled."
					log.Println(errMsg)
					errCh <- errors.New(errMsg)
					errOccurred = true
					return
				default:
				}

				operation := r.selectOperation()
				switch operation {
				case Put:
					err = r.execContext.Workers[workerID].Put()
				case Get:
					err = r.execContext.Workers[workerID].Get()
				case Delete:
					err = r.execContext.Workers[workerID].Delete()
				}
				if err != nil {
					errCh <- err
					errOccurred = true
					return
				}
			}
		}(i)
	}
	wg.Wait()
	log.Println("Validation finished.")
	r.st.Report()

	// If there are some errors, get only the first one for simplicity.
	select {
	case err := <-errCh:
		return err
	default:
	}

	return nil
}

type Operation int

const (
	Put Operation = iota
	Get
	Delete
	NumOperation
)

func (r *Runner) selectOperation() Operation {
	rand.Seed(time.Now().UnixNano())
	randVal := rand.Float64()
	if randVal < r.opeRatio[0] {
		return Put
	} else if randVal < r.opeRatio[0]+r.opeRatio[1] {
		return Get
	} else {
		return Delete
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
