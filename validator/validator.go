package validator

import (
	"context"
	"errors"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/peng225/oval/datasource"
	"github.com/peng225/oval/object"
	"github.com/peng225/oval/stat"
)

type Validator struct {
	MinSize    int
	MaxSize    int
	BucketName string
	client     *s3.Client
	objectList object.ObjectList
	st         *stat.Stat
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
		v.st.AddGetCount()
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
	v.st.AddPutCount()

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
	v.st.AddGetCount()
}

func (v *Validator) get() {
	obj := v.objectList.GetExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation on get
	res, err := v.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	err = datasource.Valid(obj, res.Body)
	if err != nil {
		log.Fatal(err)
	}
	v.st.AddGetCount()
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
	v.st.AddGetCount()

	_, err = v.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: &v.BucketName,
		Key:    &obj.Key,
	})
	if err != nil {
		log.Fatal(err)
	}
	v.st.AddDeleteCount()

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
}
