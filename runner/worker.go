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
	ObjectMeta *object.ObjectMeta `json:"objectMeta"`
}

func (w *Worker) ShowInfo() {
	// Only show the key range of the first bucket
	// because key range is the same for all buckets.
	head, tail := w.BucketsWithObject[0].ObjectMeta.GetHeadAndTailKey()
	log.Printf("Worker ID = %#x, Key = [%s, %s]\n", w.id, head, tail)
}

func (w *Worker) Put(multipartThresh int) error {
	bucketWithObj := w.selectBucketWithObject()
	obj := bucketWithObj.ObjectMeta.GetRandomObject()

	// Validation before write
	getBeforeBody, err := w.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			if bucketWithObj.ObjectMeta.Exist(obj.Key) {
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
		if !bucketWithObj.ObjectMeta.Exist(obj.Key) {
			// expect: does not exist, actual: exists
			err = fmt.Errorf("An unexpected object was found. (key = %s)", obj.Key)
			log.Println(err.Error())
			return err
		}
		err := pattern.Valid(w.id, bucketWithObj.BucketName, obj, getBeforeBody)
		if err != nil {
			err = fmt.Errorf("Data validation error occurred before put.\n%w", err)
			log.Println(err.Error())
			return err
		}
		w.st.AddGetForValidCount()
	}

	size, err := pattern.DecideSize(w.minSize, w.maxSize)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	obj.Size = size
	bucketWithObj.ObjectMeta.RegisterToExistingList(obj.Key)
	obj.WriteCount++
	if size <= multipartThresh {
		body, err := pattern.Generate(size, w.id, 0, bucketWithObj.BucketName, obj)
		if err != nil {
			log.Println(err.Error())
			return err
		}
		err = w.client.PutObject(bucketWithObj.BucketName, obj.Key, body)
		if err != nil {
			log.Println(err.Error())
			return err
		}
	} else {
		// multipart upload
		partBodies := make([]s3_client.PartBody, 0)
		remainingSize := size
		for remainingSize > 0 {
			partSize := remainingSize
			if multipartThresh < partSize {
				partSize = multipartThresh
			}
			body, err := pattern.Generate(partSize, w.id, size-remainingSize, bucketWithObj.BucketName, obj)
			if err != nil {
				log.Println(err.Error())
				return err
			}
			partBodies = append(partBodies, s3_client.PartBody{
				Body: body,
				Size: partSize,
			})
			remainingSize -= partSize
		}
		err = w.client.MultipartUpload(bucketWithObj.BucketName, obj.Key, partBodies)
		if err != nil {
			log.Println(err.Error())
			return err
		}
	}
	w.st.AddPutCount()

	// Validation after write
	getAfterBody, err := w.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			err = fmt.Errorf("Object lost after put.\nerr: %w\nobj: %v", err, obj)
		}
		log.Println(err.Error())
		return err
	}
	defer getAfterBody.Close()
	err = pattern.Valid(w.id, bucketWithObj.BucketName, obj, getAfterBody)
	if err != nil {
		err = fmt.Errorf("Data validation error occurred after put.\n%w", err)
		log.Println(err.Error())
		return err
	}
	w.st.AddGetForValidCount()
	return nil
}

func (w *Worker) Get() error {
	bucketWithObj := w.selectBucketWithObject()
	obj := bucketWithObj.ObjectMeta.GetExistingRandomObject()
	if obj == nil {
		return nil
	}

	// Validation on get
	body, err := w.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			err = fmt.Errorf("Object lost before get.\nerr: %w\nobj: %v", err, obj)
		}
		log.Println(err.Error())
		return err
	}
	defer body.Close()
	err = pattern.Valid(w.id, bucketWithObj.BucketName, obj, body)
	if err != nil {
		err = fmt.Errorf("Data validation error occurred at get operation.\n%w", err)
		log.Println(err.Error())
		return err
	}
	w.st.AddGetCount()
	return nil
}

func (w *Worker) List() error {
	bucketWithObj := w.selectBucketWithObject()

	objectNames, err := w.client.ListObjects(bucketWithObj.BucketName, bucketWithObj.ObjectMeta.KeyPrefix)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	if len(bucketWithObj.ObjectMeta.ExistingObjectIDs) != len(objectNames) {
		err = fmt.Errorf("Invalid number of objects found as a result of the LIST operation. expected = %d, actual = %d",
			len(bucketWithObj.ObjectMeta.ExistingObjectIDs), len(objectNames))
		log.Println(err.Error())
		return err
	}

	for _, objName := range objectNames {
		if !bucketWithObj.ObjectMeta.Exist(objName) {
			err = fmt.Errorf("Invalid object key '%s' found in the result of the LIST operation. workerID = 0x%x",
				objName, w.id)
			log.Println(err.Error())
			return err
		}
	}

	w.st.AddListCount()
	return nil
}

func (w *Worker) Delete() error {
	bucketWithObj := w.selectBucketWithObject()
	obj := bucketWithObj.ObjectMeta.PopExistingRandomObject()
	if obj == nil {
		return nil
	}

	// Validation before delete
	getBeforeBody, err := w.client.GetObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		var nsk *s3_client.NoSuchKey
		if errors.As(err, &nsk) {
			err = fmt.Errorf("Object lost before delete.\nerr: %w\nobj: %v", err, obj)
		}
		log.Println(err.Error())
		return err
	}
	defer getBeforeBody.Close()
	err = pattern.Valid(w.id, bucketWithObj.BucketName, obj, getBeforeBody)
	if err != nil {
		err = fmt.Errorf("Data validation error occurred before delete.\n%w", err)
		log.Println(err.Error())
		return err
	}
	w.st.AddGetForValidCount()

	err = w.client.DeleteObject(bucketWithObj.BucketName, obj.Key)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	w.st.AddDeleteCount()

	// Validation after delete
	getAfterBody, err := w.client.GetObject(bucketWithObj.BucketName, obj.Key)
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

func (w *Worker) selectBucketWithObject() *BucketWithObject {
	return w.BucketsWithObject[rand.Intn(len(w.BucketsWithObject))]
}
