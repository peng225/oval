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
	"github.com/peng225/oval/datasource"
	"github.com/peng225/oval/object"
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

type Validator struct {
	MinSize    int
	MaxSize    int
	BucketName string
	client     *s3.Client
	objectList object.ObjectList
	st         *stat.Stat
}

func (r *Runner) Init() {
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

func (v *Validator) put() {
	obj := v.objectList.GetRandomObject()

	// Validation before write
	getBeforeRes, err := v.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			if v.objectList.Exist(obj.Key) {
				// expect: exists, actual: does not exist
				log.Fatalf("An object has been lost. (key = %s)", obj.Key)
			}
		}
	} else {
		defer getBeforeRes.Body.Close()
		if !v.objectList.Exist(obj.Key) {
			// expect: does not exist, actual: exists
			log.Fatalf("An unexpected object was found. (key = %s)", obj.Key)
		}
		err := datasource.Valid(obj, getBeforeRes.Body)
		if err != nil {
			log.Fatal(err)
		}
	}

	v.objectList.RegisterToExistingList(obj.Key)
	obj.WriteCount++
	body, size, err := datasource.Generate(v.MinSize, v.MaxSize, obj)
	obj.Size = size
	if err != nil {
		log.Fatal(err)
	}
	_, err = v.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
		Body:   body,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Validation after write
	getAfterRes, err := v.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer getAfterRes.Body.Close()
	err = datasource.Valid(obj, getAfterRes.Body)
	if err != nil {
		log.Fatal(err)
	}
	v.st.AddPutCount()
}

func (v *Validator) delete() {
	obj := v.objectList.PopExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation before delete
	getBeforeRes, err := v.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer getBeforeRes.Body.Close()
	err = datasource.Valid(obj, getBeforeRes.Body)
	if err != nil {
		log.Fatal(err)
	}

	_, err = v.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Validation after delete
	getAfterRes, err := v.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if !errors.As(err, &nsk) {
			log.Fatalf("Unexpected error occurred. (err = %w)", err)
		}
	} else {
		defer getAfterRes.Body.Close()
		log.Fatalf("expected: object not found, actual: object found. (obj = %v)", *obj)
	}
	obj.Clear()
	v.st.AddDeleteCount()
}
