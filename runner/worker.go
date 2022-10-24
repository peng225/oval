package runner

import (
	"errors"
	"fmt"
	"log"
	"math/rand"

	"github.com/peng225/oval/object"
	"github.com/peng225/oval/pattern"
	"github.com/peng225/oval/s3_client"
	"github.com/peng225/oval/stat"
)

type Worker struct {
	id                int
	minSize           int
	maxSize           int
	BucketsWithObject []*BucketWithObject `json:"bucketsWithObject"`
	client            *s3_client.S3Client
	st                *stat.Stat
}

type BucketWithObject struct {
	BucketName string            `json:"bucketName"`
	ObjectMata object.ObjectMeta `json:"objectMata"`
}

func (v *Worker) ShowInfo() {
	// Only show the key range of the first bucket
	// because key range is the same for all buckets.
	head, tail := v.BucketsWithObject[0].ObjectMata.GetHeadAndTailKey()
	fmt.Printf("Worker ID = %#x, Key = [%s, %s]\n", v.id, head, tail)
}

func (v *Worker) Put() {
	bucketWithObj := v.selectBucketWithObject()
	obj := bucketWithObj.ObjectMata.GetRandomObject()

	// Validation before write
	getBeforeBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			if bucketWithObj.ObjectMata.Exist(obj.Key) {
				// expect: exists, actual: does not exist
				log.Fatalf("An object has been lost. (key = %s)", obj.Key)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		defer getBeforeBody.Close()
		if !bucketWithObj.ObjectMata.Exist(obj.Key) {
			// expect: does not exist, actual: exists
			log.Fatalf("An unexpected object was found. (key = %s)", obj.Key)
		}
		err := pattern.Valid(v.id, bucketWithObj.BucketName, obj, getBeforeBody)
		if err != nil {
			log.Fatalf("Data validation error occurred before put.\n%v", err)
		}
		v.st.AddGetForValidCount()
	}

	bucketWithObj.ObjectMata.RegisterToExistingList(obj.Key)
	obj.WriteCount++
	body, size, err := pattern.Generate(v.minSize, v.maxSize, v.id, bucketWithObj.BucketName, obj)
	obj.Size = size
	if err != nil {
		log.Fatal(err)
	}
	err = v.client.PutObject(bucketWithObj.BucketName, obj.Key, body)
	if err != nil {
		log.Fatal(err)
	}
	v.st.AddPutCount()

	// Validation after write
	getAfterBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			log.Fatalf("Object lost after put.\nerr: %v\nobj: %v", err, obj)
		} else {
			log.Fatal(err)
		}
	}
	defer getAfterBody.Close()
	err = pattern.Valid(v.id, bucketWithObj.BucketName, obj, getAfterBody)
	if err != nil {
		log.Fatalf("Data validation error occurred after put.\n%v", err)
	}
	v.st.AddGetForValidCount()
}

func (v *Worker) Get() {
	bucketWithObj := v.selectBucketWithObject()
	obj := bucketWithObj.ObjectMata.GetExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation on get
	body, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			log.Fatalf("Object lost before get.\nerr: %v\nobj: %v", err, obj)
		} else {
			log.Fatal(err)
		}
	}
	defer body.Close()
	err = pattern.Valid(v.id, bucketWithObj.BucketName, obj, body)
	if err != nil {
		log.Fatalf("Data validation error occurred at get operation.\n%v", err)
	}
	v.st.AddGetCount()
}

func (v *Worker) Delete() {
	bucketWithObj := v.selectBucketWithObject()
	obj := bucketWithObj.ObjectMata.PopExistingRandomObject()
	if obj == nil {
		return
	}

	// Validation before delete
	getBeforeBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			log.Fatalf("Object lost before delete.\nerr: %v\nobj: %v", err, obj)
		} else {
			log.Fatal(err)
		}
	}
	defer getBeforeBody.Close()
	err = pattern.Valid(v.id, bucketWithObj.BucketName, obj, getBeforeBody)
	if err != nil {
		log.Fatalf("Data validation error occurred before delete.\n%v", err)
	}
	v.st.AddGetForValidCount()

	err = v.client.DeleteObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		log.Fatal(err)
	}
	v.st.AddDeleteCount()

	// Validation after delete
	getAfterBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
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

func (v *Worker) selectBucketWithObject() *BucketWithObject {
	return v.BucketsWithObject[rand.Intn(len(v.BucketsWithObject))]
}
