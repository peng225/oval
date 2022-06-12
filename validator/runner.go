package validator

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/peng225/oval/s3_client"
	"github.com/peng225/oval/stat"
	"github.com/pkg/profile"
)

type Runner struct {
	NumObj        int
	NumWorker     int
	MinSize       int
	MaxSize       int
	TimeInMs      int64
	OpeRatios     []float64
	Profiler      bool
	validatorList []Validator
	client        *s3_client.S3Client
	st            stat.Stat
}

func (r *Runner) Init(bucketName, endpoint string) {
	r.client = &s3_client.S3Client{}
	r.client.Init(endpoint)
	err := r.client.HeadBucket(bucketName)
	if err != nil {
		var nf *s3_client.NotFound
		if errors.As(err, &nf) {
			err = r.client.CreateBucket(bucketName)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		err = r.client.ClearBucket(bucketName)
		if err != nil {
			log.Fatal(err)
		}
	}

	r.validatorList = make([]Validator, r.NumWorker)
	for i, _ := range r.validatorList {
		r.validatorList[i].MinSize = r.MinSize
		r.validatorList[i].MaxSize = r.MaxSize
		r.validatorList[i].BucketName = bucketName
		r.validatorList[i].client = r.client
		r.validatorList[i].objectList.Init(bucketName, r.NumObj/r.NumWorker, r.NumObj/r.NumWorker*i)
		r.validatorList[i].st = &r.st
	}
}

func (r *Runner) Run() {
	fmt.Println("Validation start.")
	if r.Profiler {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}
	wg := &sync.WaitGroup{}
	now := time.Now()
	for i := 0; i < r.NumWorker; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for time.Since(now).Milliseconds() < r.TimeInMs {
				operation := r.selectOperation()
				switch operation {
				case Put:
					r.validatorList[workerId].put()
				case Get:
					r.validatorList[workerId].get()
				case Delete:
					r.validatorList[workerId].delete()
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
	if randVal < r.OpeRatios[0] {
		return Put
	} else if randVal < r.OpeRatios[0]+r.OpeRatios[1] {
		return Get
	} else {
		return Delete
	}
}
