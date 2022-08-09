package validator

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
	maxValidatorID = 0x10000
)

type ExecutionContext struct {
	Endpoint         string      `json:"endpoint"`
	BucketNames      []string    `json:"bucketNames"`
	NumObj           int         `json:"numObj"`
	NumWorker        int         `json:"numWorker"`
	MinSize          int         `json:"minSize"`
	MaxSize          int         `json:"maxSize"`
	StartValidatorID int         `json:"startValidatorID"`
	Validators       []Validator `json:"validators"`
}

type Runner struct {
	execContext  *ExecutionContext
	opeRatios    []float64
	timeInMs     int64
	profiler     bool
	loadFileName string
	client       *s3_client.S3Client
	st           stat.Stat
}

func NewRunner(execContext *ExecutionContext, opeRatios []float64, timeInMs int64, profiler bool, loadFileName string) *Runner {
	runner := &Runner{
		execContext:  execContext,
		opeRatios:    opeRatios,
		timeInMs:     timeInMs,
		profiler:     profiler,
		loadFileName: loadFileName,
	}
	runner.init()
	return runner
}

func NewRunnerFromLoadFile(loadFileName string, opeRatios []float64, timeInMs int64, profiler bool) *Runner {
	if loadFileName == "" {
		log.Fatal("loadFileName is empty.")
	}
	_, err := os.Stat(loadFileName)
	if err != nil {
		log.Fatal(err)
	}
	ec := loadSavedContext(loadFileName)
	return NewRunner(ec, opeRatios, timeInMs, profiler, loadFileName)
}

func loadSavedContext(loadFileName string) *ExecutionContext {
	f, err := os.Open(loadFileName)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	savedContext, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}
	ec := &ExecutionContext{}
	json.Unmarshal(savedContext, ec)
	return ec
}

func (r *Runner) init() {
	r.client = &s3_client.S3Client{}
	r.client.Init(r.execContext.Endpoint)
	for _, bucketName := range r.execContext.BucketNames {
		err := r.client.HeadBucket(bucketName)
		if err != nil {
			var nf *s3_client.NotFound
			if errors.As(err, &nf) {
				if r.loadFileName != "" {
					log.Fatal("HeadBucket failed despite \"load\" parameter was set.")
				}
				fmt.Println("Bucket \"" + bucketName + "\" not found. Creating...")
				err = r.client.CreateBucket(bucketName)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Println("Bucket created successfully.")
			} else {
				log.Fatal(err)
			}
		} else {
			if r.loadFileName == "" {
				err = r.client.ClearBucket(bucketName)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	if r.loadFileName == "" {
		r.execContext.Validators = make([]Validator, r.execContext.NumWorker)
		rand.Seed(time.Now().UnixNano())
		startID := rand.Intn(maxValidatorID)
		r.execContext.StartValidatorID = startID
		for i, _ := range r.execContext.Validators {
			r.execContext.Validators[i].ID = (startID + i) % maxValidatorID
			r.execContext.Validators[i].MinSize = r.execContext.MinSize
			r.execContext.Validators[i].MaxSize = r.execContext.MaxSize
			r.execContext.Validators[i].BucketsWithObject = make([]*BucketWithObject, len(r.execContext.BucketNames))
			for j, bucketName := range r.execContext.BucketNames {
				r.execContext.Validators[i].BucketsWithObject[j] = &BucketWithObject{
					BucketName: bucketName,
					ObjectMata: object.ObjectMeta{},
				}
				r.execContext.Validators[i].BucketsWithObject[j].ObjectMata.Init(
					r.execContext.NumObj/r.execContext.NumWorker,
					r.execContext.NumObj/r.execContext.NumWorker*i,
				)
			}
			r.execContext.Validators[i].client = r.client
			r.execContext.Validators[i].st = &r.st
			r.execContext.Validators[i].ShowInfo()
		}
	} else {
		for i, _ := range r.execContext.Validators {
			r.execContext.Validators[i].ID = (r.execContext.StartValidatorID + i) % maxValidatorID
			r.execContext.Validators[i].MinSize = r.execContext.MinSize
			r.execContext.Validators[i].MaxSize = r.execContext.MaxSize
			r.execContext.Validators[i].client = r.client
			r.execContext.Validators[i].st = &r.st
			r.execContext.Validators[i].ShowInfo()
		}
	}
}

func (r *Runner) Run() {
	fmt.Println("Validation start.")
	if r.profiler {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}
	wg := &sync.WaitGroup{}
	now := time.Now()
	for i := 0; i < r.execContext.NumWorker; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for time.Since(now).Milliseconds() < r.timeInMs {
				operation := r.selectOperation()
				switch operation {
				case Put:
					r.execContext.Validators[workerId].Put()
				case Get:
					r.execContext.Validators[workerId].Get()
				case Delete:
					r.execContext.Validators[workerId].Delete()
				}
			}
		}(i)
	}
	wg.Wait()
	fmt.Println("Validation finished.")
	r.st.Report()
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
	if randVal < r.opeRatios[0] {
		return Put
	} else if randVal < r.opeRatios[0]+r.opeRatios[1] {
		return Get
	} else {
		return Delete
	}
}

func (r *Runner) SaveContext(saveFileName string) error {
	// Check if a file with the name "saveFileName" exists.
	_, err := os.Stat(saveFileName)
	if err == nil {
		fmt.Println(`A file "` + saveFileName + `" already exists. Are you sure to overwrite it? (y/N)`)
		var userInput string
		_, err = fmt.Scan(&userInput)
		if err != nil {
			return err
		}
		if userInput != "y" {
			fmt.Println("Saving file canceled.")
			return nil
		}
	}
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
