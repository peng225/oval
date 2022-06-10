package validator

import (
	"errors"
	"log"

	"github.com/peng225/oval/datasource"
	"github.com/peng225/oval/object"
	"github.com/peng225/oval/s3_client"
	"github.com/peng225/oval/stat"
)

type Validator struct {
	MinSize    int
	MaxSize    int
	BucketName string
	client     *s3_client.S3Client
	objectList object.ObjectList
	st         *stat.Stat
}

func (v *Validator) put() {
	obj := v.objectList.GetRandomObject()

	// Validation before write
	getBeforeBody, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			if v.objectList.Exist(obj.Key) {
				// expect: exists, actual: does not exist
				log.Fatalf("An object has been lost. (key = %s)", obj.Key)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		defer getBeforeBody.Close()
		if !v.objectList.Exist(obj.Key) {
			// expect: does not exist, actual: exists
			log.Fatalf("An unexpected object was found. (key = %s)", obj.Key)
		}
		err := datasource.Valid(obj, getBeforeBody)
		if err != nil {
			log.Fatalf("Data validation error occurred before put.\n%v", err)
		}
		v.st.AddGetForValidCount()
	}

	v.objectList.RegisterToExistingList(obj.Key)
	obj.WriteCount++
	body, size, err := datasource.Generate(v.MinSize, v.MaxSize, obj)
	obj.Size = size
	if err != nil {
		log.Fatal(err)
	}
	err = v.client.PutObject(v.BucketName, obj.Key, body)
	if err != nil {
		log.Fatal(err)
	}
	v.st.AddPutCount()

	// Validation after write
	getAfterBody, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		log.Fatal(err)
	}
	defer getAfterBody.Close()
	err = datasource.Valid(obj, getAfterBody)
	if err != nil {
		log.Fatalf("Data validation error occurred after put.\n%v", err)
	}
	v.st.AddGetForValidCount()
}

func (v *Validator) get() {
	obj := v.objectList.GetExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation on get
	body, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		log.Fatal(err)
	}
	defer body.Close()
	err = datasource.Valid(obj, body)
	if err != nil {
		log.Fatalf("Data validation error occurred at get operation.\n%v", err)
	}
	v.st.AddGetCount()
}

func (v *Validator) delete() {
	obj := v.objectList.PopExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation before delete
	getBeforeBody, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		log.Fatal(err)
	}
	defer getBeforeBody.Close()
	err = datasource.Valid(obj, getBeforeBody)
	if err != nil {
		log.Fatalf("Data validation error occurred before delete.\n%v", err)
	}
	v.st.AddGetForValidCount()

	err = v.client.DeleteObject(v.BucketName, obj.Key)
	if err != nil {
		log.Fatal(err)
	}
	v.st.AddDeleteCount()

	// Validation after delete
	getAfterBody, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if !errors.As(err, &nsk) {
			log.Fatalf("Unexpected error occurred. (err = %v)", err)
		}
	} else {
		defer getAfterBody.Close()
		log.Fatalf("expected: object not found, actual: object found. (obj = %v)", *obj)
	}
	obj.Clear()
}
