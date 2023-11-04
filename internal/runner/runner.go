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
	processID       int
	multipartThresh int
	caCertFileName  string
}

func NewRunner(execContext *ExecutionContext, opeRatio []float64, timeInMs int64,
	profiler bool, loadFileName string, processID, multipartThresh int,
	caCertFileName string) *Runner {
	runner := &Runner{
		execContext:     execContext,
		opeRatio:        opeRatio,
		timeInMs:        timeInMs,
		profiler:        profiler,
		loadFileName:    loadFileName,
		processID:       processID,
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
		log.Fatal(err)
	}
	ec := loadSavedContext(loadFileName)
	return NewRunner(ec, opeRatio, timeInMs, profiler, loadFileName, 0, multipartThresh, caCertFileName)
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
	err = json.Unmarshal(savedContext, ec)
	if err != nil {
		log.Fatal(err)
	}
	return ec
}

func (r *Runner) init() {
	r.client = s3client.NewS3Client(r.execContext.Endpoint, r.caCertFileName, r.multipartThresh)
	err := r.initBucket()
	if err != nil {
		log.Fatal(err)
	}
	if r.loadFileName == "" {
		r.execContext.Workers = make([]Worker, r.execContext.NumWorker)
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
					ObjectMeta: object.NewObjectMeta(
						r.execContext.NumObj/r.execContext.NumWorker,
						(int64(r.processID)<<32)+(int64(i)<<24)),
				}
			}
			r.execContext.Workers[i].client = r.client
			r.execContext.Workers[i].st = &r.st
			r.execContext.Workers[i].ShowInfo()
		}
	} else {
		for i := range r.execContext.Workers {
			r.execContext.Workers[i].id = (r.execContext.StartWorkerID + i) % maxWorkerID
			r.execContext.Workers[i].minSize = r.execContext.MinSize
			r.execContext.Workers[i].maxSize = r.execContext.MaxSize
			r.execContext.Workers[i].client = r.client
			r.execContext.Workers[i].st = &r.st
			for j := range r.execContext.Workers[i].BucketsWithObject {
				r.execContext.Workers[i].BucketsWithObject[j].ObjectMeta.TidyUp()
			}
			r.execContext.Workers[i].ShowInfo()
		}
	}
}

func (r *Runner) initBucket() error {
	for _, bucketName := range r.execContext.BucketNames {
		err := r.client.HeadBucket(bucketName)
		if err != nil {
			var nf *s3client.NotFound
			if errors.As(err, &nf) {
				if r.loadFileName != "" {
					return fmt.Errorf(`HeadBucket failed despite "load" parameter was set.`)
				}
				log.Printf(`Bucket "%s" not found. Creating...`, bucketName)
				err = r.client.CreateBucket(bucketName)
				if err != nil {
					// Bucket creation may be executed by multiple follower processes.
					var cf *s3client.Conflict
					if errors.As(err, &cf) {
						log.Printf(`Bucket "%s" already exists.`, bucketName)
					} else {
						return err
					}
				} else {
					log.Println("Bucket created successfully.")
				}
			} else {
				return err
			}
		} else {
			if r.loadFileName == "" {
				log.Printf("Clearing bucket '%s'.\n", bucketName)
				err = r.client.ClearBucket(bucketName, fmt.Sprintf("%s%02x", object.KeyShortPrefix, r.processID))
				if err != nil {
					return err
				}
				log.Println("Bucket cleared successfully.")
			}
		}
	}
	return nil
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
				case List:
					err = r.execContext.Workers[workerID].List()
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