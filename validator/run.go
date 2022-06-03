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
)

type Validator struct {
	NumObj     int64
	NumWorker  int
	MinSize    int
	MaxSize    int
	TimeInMs   int64
	BucketName string
	client     *s3.Client
	objectList object.ObjectList
}

func (v *Validator) Init() {
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
	v.client = s3.NewFromConfig(cfg)

	v.objectList.Init(v.BucketName, v.NumObj)

	v.clearBucket()

	_, err = v.client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: &v.BucketName,
	})
	if err != nil {
		var nsb *types.NoSuchBucket
		if errors.As(err, &nsb) {
			_, err = v.client.CreateBucket(context.Background(), &s3.CreateBucketInput{
				Bucket: &v.BucketName,
			})
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

func (v *Validator) clearBucket() {
	for {
		var continuationToken *string = nil
		listRes, err := v.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket:            &v.BucketName,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			log.Fatal(err)
		}
		if len(listRes.Contents) == 0 {
			break
		}
		for _, obj := range listRes.Contents {
			_, err := v.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: &v.BucketName,
				Key:    obj.Key,
			})
			if err != nil {
				log.Fatal(err)
			}
		}
		continuationToken = listRes.NextContinuationToken
	}
}

func (v *Validator) Run() {
	fmt.Println("Validation start.")
	wg := &sync.WaitGroup{}
	now := time.Now()
	for i := 0; i < v.NumWorker; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Since(now).Milliseconds() < v.TimeInMs {
				operation := v.selectOperation()
				switch operation {
				case Put:
					v.create()
				case Read:
					// Do nothing
				case Delete:
					v.delete()
				}
			}
		}()
	}
	wg.Wait()
	fmt.Println("Validation finished.")
}

type Operation int

const (
	Put Operation = iota
	Read
	Delete
	NumOperation
)

func (v *Validator) selectOperation() Operation {
	rand.Seed(time.Now().UnixNano())
	return Operation(rand.Intn(int(NumOperation)))
}

func (v *Validator) create() {
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
				// expect: exist, actual: does not exist
				log.Fatalf("Object lost. (key = %s)", obj.Key)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		defer getBeforeRes.Body.Close()
		datasource.Valid(obj, getBeforeRes.Body)
	}

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

	datasource.Valid(obj, getAfterRes.Body)
}

func (v *Validator) delete() {
	obj := v.objectList.PopExistingRandomObject()
	if obj == nil {
		return
	}
	_, err := v.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		log.Fatal(err)
	}
}
