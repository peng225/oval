package s3_client

import (
	"context"
	"errors"
	"io"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Client struct {
	client *s3.Client
}

type NoSuchKey struct {
	errorMessage string
}

func (nsk *NoSuchKey) Error() string {
	return nsk.errorMessage
}

type NotFound struct {
	errorMessage string
}

func (nf *NotFound) Error() string {
	return nf.errorMessage
}

func NewS3Client(endpoint string) *S3Client {
	s := &S3Client{}
	var cfg aws.Config
	var err error
	if endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               endpoint,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		})
		cfg, err = config.LoadDefaultConfig(context.Background(), config.WithEndpointResolverWithOptions(customResolver))
		if err != nil {
			log.Fatal(err)
		}
	} else {
		cfg, err = config.LoadDefaultConfig(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create an Amazon S3 service client
	s.client = s3.NewFromConfig(cfg)

	return s
}

func (s *S3Client) CreateBucket(bucketName string) error {
	_, err := s.client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3Client) ClearBucket(bucketName, prefix string) error {
	var continuationToken *string = nil
	for {
		listRes, err := s.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket:            &bucketName,
			ContinuationToken: continuationToken,
			Prefix:            &prefix,
		})
		if err != nil {
			return err
		}
		if len(listRes.Contents) == 0 {
			break
		}
		for _, obj := range listRes.Contents {
			_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: &bucketName,
				Key:    obj.Key,
			})
			if err != nil {
				return err
			}
		}
		continuationToken = listRes.NextContinuationToken
	}
	return nil
}

func (s *S3Client) PutObject(bucketName, key string, body io.ReadSeeker) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &bucketName,
		Key:    &key,
		Body:   body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3Client) GetObject(bucketName, key string) (io.ReadCloser, error) {
	res, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			err = &NoSuchKey{
				errorMessage: err.Error(),
			}
		}
		return nil, err
	}
	return res.Body, err
}

func (s *S3Client) DeleteObject(bucketName, key string) error {
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3Client) HeadBucket(bucketName string) error {
	_, err := s.client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		var nf *types.NotFound
		if errors.As(err, &nf) {
			err = &NotFound{
				errorMessage: err.Error(),
			}
		}
		return err
	}
	return nil
}
