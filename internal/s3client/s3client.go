package s3client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Client struct {
	client          *s3.Client
	multipartThresh int
}

var (
	NotFound  = errors.New("not found")
	NoSuchKey = errors.New("no such key")
	Conflict  = errors.New("conflict")
)

func getTLSClient(caCertFileName string) (*http.Client, error) {
	cert, err := os.ReadFile(caCertFileName)
	if err != nil {
		return nil, err
	}

	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if !caCertPool.AppendCertsFromPEM(cert) {
		return nil, fmt.Errorf("failed to add ca cert: cert=%v", cert)
	}

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("invalid default transport")
	}

	transport := defaultTransport.Clone()

	transport.TLSClientConfig = &tls.Config{
		RootCAs: caCertPool,
	}

	client := &http.Client{
		Transport: transport,
	}

	return client, nil
}

func NewS3Client(endpoint, caCertFileName string, multipartThresh int) *S3Client {
	s := &S3Client{
		multipartThresh: multipartThresh,
	}
	var cfg aws.Config
	var err error
	client := http.DefaultClient

	if caCertFileName != "" {
		client, err = getTLSClient(caCertFileName)
		if err != nil {
			log.Fatal(err)
		}
	}
	if endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               endpoint,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		})
		cfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithEndpointResolverWithOptions(customResolver),
			config.WithHTTPClient(client))
		if err != nil {
			log.Fatal(err)
		}
	} else {
		cfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithHTTPClient(client))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create an Amazon S3 service client
	s.client = s3.NewFromConfig(cfg)

	return s
}

func (s *S3Client) CreateBucket(ctx context.Context, bucketName string) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		var baoby *types.BucketAlreadyOwnedByYou
		if errors.As(err, &baoby) {
			err = errors.Join(err, Conflict)
		}
		return err
	}
	return nil
}

func (s *S3Client) ClearBucket(ctx context.Context, bucketName, prefix string) error {
	for {
		listRes, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: &bucketName,
			Prefix: &prefix,
		})
		if err != nil {
			return err
		}
		if len(listRes.Contents) == 0 {
			break
		}
		for _, obj := range listRes.Contents {
			_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: &bucketName,
				Key:    obj.Key,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *S3Client) PutObject(ctx context.Context, bucketName, key string, body []byte) (int, error) {
	var err error
	var partCount int
	if len(body) <= s.multipartThresh {
		_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucketName,
			Key:    &key,
			Body:   bytes.NewReader(body),
		})
		if err != nil {
			return 0, err
		}
		partCount = 1
	} else {
		partCount, err = s.multipartUpload(ctx, bucketName, key, body)
	}
	return partCount, err
}

func (s *S3Client) multipartUpload(ctx context.Context, bucketName, key string, body []byte) (int, error) {
	cmuOutput, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		return 0, err
	}

	partList := make([]types.CompletedPart, 0)
	remainingSize := int64(len(body))
	partNumber := int32(1)
	for remainingSize > 0 {
		pn := partNumber
		partSize := remainingSize
		if int64(s.multipartThresh) < partSize {
			partSize = int64(s.multipartThresh)
		}
		upOutput, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:        &bucketName,
			Key:           &key,
			Body:          bytes.NewReader(body[:partSize]),
			PartNumber:    &pn,
			UploadId:      cmuOutput.UploadId,
			ContentLength: &partSize,
		})
		if err != nil {
			// `ctx` cannot be used for AbortMultipartUpload()
			// because UploadPart() may have failed due to
			// the cancellation of `ctx`.
			_, abortErr := s.client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
				Bucket:   &bucketName,
				Key:      &key,
				UploadId: cmuOutput.UploadId,
			})
			if abortErr != nil {
				log.Fatalf("UploadPart err: %v, AbortMultipartUpload err: %v", err, abortErr)
			}
			return 0, err
		}
		body = body[partSize:]
		partList = append(partList, types.CompletedPart{
			PartNumber: &pn,
			ETag:       upOutput.ETag,
		})
		partNumber++
		remainingSize -= partSize
	}

	_, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   &bucketName,
		Key:      &key,
		UploadId: cmuOutput.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: partList,
		},
	})
	if err != nil {
		// `ctx` cannot be used for AbortMultipartUpload()
		// because UploadPart() may have failed due to
		// the cancellation of `ctx`.
		_, abortErr := s.client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
			Bucket:   &bucketName,
			Key:      &key,
			UploadId: cmuOutput.UploadId,
		})
		if abortErr != nil {
			log.Fatalf("CompleteMultipartUpload err: %v, AbortMultipartUpload err: %v", err, abortErr)
		}
		return 0, err
	}
	return int(partNumber - 1), nil
}

func (s *S3Client) GetObject(ctx context.Context, bucketName, key string) (io.ReadCloser, error) {
	res, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			err = errors.Join(err, NoSuchKey)
		}
		return nil, err
	}
	return res.Body, err
}

func (s *S3Client) ListObjects(ctx context.Context, bucketName, prefix string) ([]string, error) {
	var continuationToken *string = nil
	objectNames := make([]string, 0)
	for {
		listRes, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &bucketName,
			ContinuationToken: continuationToken,
			Prefix:            &prefix,
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range listRes.Contents {
			objectNames = append(objectNames, *obj.Key)
		}

		if listRes.NextContinuationToken == nil {
			break
		}
		continuationToken = listRes.NextContinuationToken
	}
	return objectNames, nil
}

func (s *S3Client) DeleteObject(ctx context.Context, bucketName, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3Client) HeadBucket(ctx context.Context, bucketName string) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		var nf *types.NotFound
		if errors.As(err, &nf) {
			err = errors.Join(err, NotFound)
		}
		return err
	}
	return nil
}
