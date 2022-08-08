package validator

import (
	"errors"
	"fmt"
	"log"

	"github.com/peng225/oval/datasource"
	"github.com/peng225/oval/object"
	"github.com/peng225/oval/s3_client"
	"github.com/peng225/oval/stat"
)

type Validator struct {
	ID         int
	MinSize    int
	MaxSize    int
	BucketName string
	ObjectMata object.ObjectMeta `json:"objectMeta"`
	client     *s3_client.S3Client
	st         *stat.Stat
}

func (v *Validator) ShowInfo() {
	head, tail := v.ObjectMata.GetHeadAndTailKey()
	fmt.Printf("Worker ID = %#x, Key = [%s, %s]\n", v.ID, head, tail)
}

func (v *Validator) Put() {
	obj := v.ObjectMata.GetRandomObject()

	// Validation before write
	getBeforeBody, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			if v.ObjectMata.Exist(obj.Key) {
				// expect: exists, actual: does not exist
				log.Fatalf("An object has been lost. (key = %s)", obj.Key)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		defer getBeforeBody.Close()
		if !v.ObjectMata.Exist(obj.Key) {
			// expect: does not exist, actual: exists
			log.Fatalf("An unexpected object was found. (key = %s)", obj.Key)
		}
		err := datasource.Valid(v.ID, obj, getBeforeBody)
		if err != nil {
			log.Fatalf("Data validation error occurred before put.\n%v", err)
		}
		v.st.AddGetForValidCount()
	}

	v.ObjectMata.RegisterToExistingList(obj.Key)
	obj.WriteCount++
	body, size, err := datasource.Generate(v.MinSize, v.MaxSize, v.ID, obj)
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
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			log.Fatalf("Object lost after put.\nerr: %v\nobj: %v", err, obj)
		} else {
			log.Fatal(err)
		}
	}
	defer getAfterBody.Close()
	err = datasource.Valid(v.ID, obj, getAfterBody)
	if err != nil {
		log.Fatalf("Data validation error occurred after put.\n%v", err)
	}
	v.st.AddGetForValidCount()
}

func (v *Validator) Get() {
	obj := v.ObjectMata.GetExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation on get
	body, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			log.Fatalf("Object lost before get.\nerr: %v\nobj: %v", err, obj)
		} else {
			log.Fatal(err)
		}
	}
	defer body.Close()
	err = datasource.Valid(v.ID, obj, body)
	if err != nil {
		log.Fatalf("Data validation error occurred at get operation.\n%v", err)
	}
	v.st.AddGetCount()
}

func (v *Validator) Delete() {
	obj := v.ObjectMata.PopExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation before delete
	getBeforeBody, err := v.client.GetObject(v.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			log.Fatalf("Object lost before delete.\nerr: %v\nobj: %v", err, obj)
		} else {
			log.Fatal(err)
		}
	}
	defer getBeforeBody.Close()
	err = datasource.Valid(v.ID, obj, getBeforeBody)
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
