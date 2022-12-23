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
	BucketName string             `json:"bucketName"`
	ObjectMata *object.ObjectMeta `json:"objectMeta"`
}

func (v *Worker) ShowInfo() {
	// Only show the key range of the first bucket
	// because key range is the same for all buckets.
	head, tail := v.BucketsWithObject[0].ObjectMata.GetHeadAndTailKey()
	fmt.Printf("Worker ID = %#x, Key = [%s, %s]\n", v.id, head, tail)
}

func (v *Worker) Put() error {
	bucketWithObj := v.selectBucketWithObject()
	obj := bucketWithObj.ObjectMata.GetRandomObject()

	// Validation before write
	getBeforeBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			if bucketWithObj.ObjectMata.Exist(obj.Key) {
				// expect: exists, actual: does not exist
				err = fmt.Errorf("An object has been lost. (key = %s)", obj.Key)
				log.Println(err.Error())
				return err
			}
		} else {
			log.Println(err.Error())
			return err
		}
	} else {
		defer getBeforeBody.Close()
		if !bucketWithObj.ObjectMata.Exist(obj.Key) {
			// expect: does not exist, actual: exists
			err = fmt.Errorf("An unexpected object was found. (key = %s)", obj.Key)
			log.Println(err.Error())
			return err
		}
		err := pattern.Valid(v.id, bucketWithObj.BucketName, obj, getBeforeBody)
		if err != nil {
			err = fmt.Errorf("Data validation error occurred before put.\n%w", err)
			log.Println(err.Error())
			return err
		}
		v.st.AddGetForValidCount()
	}

	bucketWithObj.ObjectMata.RegisterToExistingList(obj.Key)
	obj.WriteCount++
	body, size, err := pattern.Generate(v.minSize, v.maxSize, v.id, bucketWithObj.BucketName, obj)
	obj.Size = size
	if err != nil {
		log.Println(err.Error())
		return err
	}
	err = v.client.PutObject(bucketWithObj.BucketName, obj.Key, body)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	v.st.AddPutCount()

	// Validation after write
	getAfterBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			err = fmt.Errorf("Object lost after put.\nerr: %w\nobj: %v", err, obj)
			log.Println(err.Error())
			return err
		} else {
			log.Println(err.Error())
			return err
		}
	}
	defer getAfterBody.Close()
	err = pattern.Valid(v.id, bucketWithObj.BucketName, obj, getAfterBody)
	if err != nil {
		err = fmt.Errorf("Data validation error occurred after put.\n%w", err)
		log.Println(err.Error())
		return err
	}
	v.st.AddGetForValidCount()
	return nil
}

func (v *Worker) Get() error {
	bucketWithObj := v.selectBucketWithObject()
	obj := bucketWithObj.ObjectMata.GetExistingRandomObject()
	if obj == nil {
		return nil
	}

	// Validation on get
	body, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			err = fmt.Errorf("Object lost before get.\nerr: %w\nobj: %v", err, obj)
			log.Println(err.Error())
			return err
		} else {
			log.Println(err.Error())
			return err
		}
	}
	defer body.Close()
	err = pattern.Valid(v.id, bucketWithObj.BucketName, obj, body)
	if err != nil {
		err = fmt.Errorf("Data validation error occurred at get operation.\n%w", err)
		log.Println(err.Error())
		return err
	}
	v.st.AddGetCount()
	return nil
}

func (v *Worker) Delete() error {
	bucketWithObj := v.selectBucketWithObject()
	obj := bucketWithObj.ObjectMata.PopExistingRandomObject()
	if obj == nil {
		return nil
	}

	// Validation before delete
	getBeforeBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			err = fmt.Errorf("Object lost before delete.\nerr: %w\nobj: %v", err, obj)
			log.Println(err.Error())
			return err
		} else {
			log.Println(err.Error())
			return err
		}
	}
	defer getBeforeBody.Close()
	err = pattern.Valid(v.id, bucketWithObj.BucketName, obj, getBeforeBody)
	if err != nil {
		err = fmt.Errorf("Data validation error occurred before delete.\n%w", err)
		log.Println(err.Error())
		return err
	}
	v.st.AddGetForValidCount()

	err = v.client.DeleteObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	v.st.AddDeleteCount()

	// Validation after delete
	getAfterBody, err := v.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if !errors.As(err, &nsk) {
			err = fmt.Errorf("Unexpected error occurred. (err = %w)", err)
			log.Println(err.Error())
			return err
		}
	} else {
		defer getAfterBody.Close()
		err = fmt.Errorf("expected: object not found, actual: object found. (obj = %v)", *obj)
		log.Println(err.Error())
		return err
	}
	obj.Clear()
	return nil
}

func (v *Worker) selectBucketWithObject() *BucketWithObject {
	return v.BucketsWithObject[rand.Intn(len(v.BucketsWithObject))]
}
