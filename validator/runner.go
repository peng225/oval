package validator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/peng225/oval/stat"
	"github.com/pkg/profile"
)

type Runner struct {
	NumObj        int
	NumWorker     int
	MinSize       int
	MaxSize       int
	TimeInMs      int64
	BucketName    string
	validatorList []Validator
	client        *s3.Client
	st            stat.Stat
}

func (r *Runner) Init() {
	// TODO: remove the dependency on MinIO
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:       "aws",
			URL:               "http://127.0.0.1:9000",
			SigningRegion:     "jp-tokyo-test",
			HostnameImmutable: true,
		}, nil
	})
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithEndpointResolverWithOptions(customResolver))
	if err != nil {
		log.Fatal(err)
	}

	// Create an Amazon S3 service client
	r.client = s3.NewFromConfig(cfg)

	r.validatorList = make([]Validator, r.NumWorker)
	for i, _ := range r.validatorList {
		r.validatorList[i].MinSize = r.MinSize
		r.validatorList[i].MaxSize = r.MaxSize
		r.validatorList[i].BucketName = r.BucketName
		r.validatorList[i].client = r.client
		r.validatorList[i].objectList.Init(r.BucketName, r.NumObj/r.NumWorker, r.NumObj/r.NumWorker*i)
		r.validatorList[i].st = &r.st
	}

	_, err = r.client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: &r.BucketName,
	})
	if err != nil {
		var nf *types.NotFound
		if errors.As(err, &nf) {
			_, err = r.client.CreateBucket(context.Background(), &s3.CreateBucketInput{
				Bucket: &r.BucketName,
			})
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		r.clearBucket()
	}
}

func (r *Runner) clearBucket() {
	for {
		var continuationToken *string = nil
		listRes, err := r.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket:            &r.BucketName,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			log.Fatal(err)
		}
		if len(listRes.Contents) == 0 {
			break
		}
		for _, obj := range listRes.Contents {
			_, err := r.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: &r.BucketName,
				Key:    obj.Key,
			})
			if err != nil {
				log.Fatal(err)
			}
		}
		continuationToken = listRes.NextContinuationToken
	}
}

func (r *Runner) Run() {
	fmt.Println("Validation start.")
	defer profile.Start(profile.ProfilePath(".")).Stop()
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
				case Read:
					// TODO: implement
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
	Read
	Delete
	NumOperation
)

func (r *Runner) selectOperation() Operation {
	rand.Seed(time.Now().UnixNano())
	return Operation(rand.Intn(int(NumOperation)))
}
