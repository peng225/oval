package validator

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/peng225/oval/datasource"
	"github.com/peng225/oval/object"
)

type Validator struct {
	NumObj     int64
	NumWorker  int
	MinSize    int
	MaxSize    int
	NumRound   int
	BucketName string
	client     *s3.Client
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
}

func (v *Validator) Run() {
	for round := 0; round < v.NumRound; round++ {
		fmt.Printf("round: %v\n", round)
		// Create phase
		wg := &sync.WaitGroup{}
		for i := 0; i < v.NumWorker; i++ {
			wg.Add(1)
			go func() {
				v.create()
				wg.Done()
			}()
		}
		wg.Wait()

		// Update phase
		for i := 0; i < v.NumWorker; i++ {
			wg.Add(1)
			go func() {
				v.update()
				wg.Done()
			}()
		}
		wg.Wait()

		// Delete phase
		for i := 0; i < v.NumWorker; i++ {
			wg.Add(1)
			go func() {
				v.delete()
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func (v *Validator) create() {
	numObj := v.NumObj

	of := new(object.ObjectFactory)
	for numObj != 0 {
		obj := of.NewObject(v.BucketName)
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
		numObj--
		getRes, err := v.client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: &v.BucketName,
			Key:    &obj.Key,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer getRes.Body.Close()

		data, err := io.ReadAll(getRes.Body)
		if err != nil {
			log.Fatal(err)
		}
		file, err := os.Create("hoge.txt")
		if err != nil {
			log.Fatal(err)
		}
		file.Write(data)
		file.Close()

		datasource.Validate(getRes.Body)
	}
}

func (v *Validator) update() {
	// TODO: implement
}

func (v *Validator) delete() {
	// TODO: implement
}
