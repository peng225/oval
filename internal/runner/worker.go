package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"

	"github.com/peng225/oval/internal/object"
	"github.com/peng225/oval/internal/pattern"
	"github.com/peng225/oval/internal/s3client"
	"github.com/peng225/oval/internal/stat"
)

type Worker struct {
	id                int
	minSize           int
	maxSize           int
	BucketsWithObject []*BucketWithObject `json:"bucketsWithObject"`
	client            *s3client.S3Client
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

func (w *Worker) Put(ctx context.Context) error {
	bucketWithObj := w.selectBucketWithObject()
	obj := bucketWithObj.ObjectMeta.GetRandomObject()

	// Validation before write
	getBeforeBody, err := w.client.GetObject(ctx, bucketWithObj.BucketName, obj.Key)
	if err != nil {
		if errors.Is(err, s3client.ErrNoSuchKey) {
			if bucketWithObj.ObjectMeta.Exist(obj.Key) {
				// expect: exists, actual: does not exist
				err = fmt.Errorf("an object has been lost. (key = %s)", obj.Key)
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
			err = fmt.Errorf("an unexpected object was found. (key = %s)", obj.Key)
			log.Println(err.Error())
			return err
		}
		err := pattern.Valid(w.id, bucketWithObj.BucketName, obj, getBeforeBody)
		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Println("Detected the canceled context.")
				return nil
			}
			err = fmt.Errorf("data validation error occurred before put.\n%w", err)
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
	body, err := pattern.Generate(size, w.id, bucketWithObj.BucketName, obj)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	partCount, err := w.client.PutObject(ctx, bucketWithObj.BucketName, obj.Key, body)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	w.st.AddUploadedPartCount(int64(partCount))
	w.st.AddPutCount()

	// Validation after write
	getAfterBody, err := w.client.GetObject(ctx, bucketWithObj.BucketName, obj.Key)
	if err != nil {
		if errors.Is(err, s3client.ErrNoSuchKey) {
			err = fmt.Errorf("object lost after put.\nerr: %w\nobj: %v", err, obj)
		}
		log.Println(err.Error())
		return err
	}
	defer getAfterBody.Close()
	err = pattern.Valid(w.id, bucketWithObj.BucketName, obj, getAfterBody)
	if err != nil {
		if ctx.Err() == context.Canceled {
			log.Println("Detected the canceled context.")
			return nil
		}
		err = fmt.Errorf("data validation error occurred after put.\n%w", err)
		log.Println(err.Error())
		return err
	}
	w.st.AddGetForValidCount()
	return nil
}

func (w *Worker) Get(ctx context.Context) error {
	bucketWithObj := w.selectBucketWithObject()
	obj := bucketWithObj.ObjectMeta.GetExistingRandomObject()
	if obj == nil {
		return nil
	}

	// Validation on get
	body, err := w.client.GetObject(ctx, bucketWithObj.BucketName, obj.Key)
	if err != nil {
		if errors.Is(err, s3client.ErrNoSuchKey) {
			err = fmt.Errorf("object lost before get.\nerr: %w\nobj: %v", err, obj)
		}
		log.Println(err.Error())
		return err
	}
	defer body.Close()
	err = pattern.Valid(w.id, bucketWithObj.BucketName, obj, body)
	if err != nil {
		if ctx.Err() == context.Canceled {
			log.Println("Detected the canceled context.")
			return nil
		}
		err = fmt.Errorf("data validation error occurred at get operation.\n%w", err)
		log.Println(err.Error())
		return err
	}
	w.st.AddGetCount()
	return nil
}

func (w *Worker) List(ctx context.Context) error {
	bucketWithObj := w.selectBucketWithObject()

	objectNames, err := w.client.ListObjects(ctx, bucketWithObj.BucketName, bucketWithObj.ObjectMeta.KeyPrefix)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	if len(bucketWithObj.ObjectMeta.ExistingObjectIDs) != len(objectNames) {
		err = fmt.Errorf("invalid number of objects found as a result of the LIST operation. expected = %d, actual = %d",
			len(bucketWithObj.ObjectMeta.ExistingObjectIDs), len(objectNames))
		log.Println(err.Error())
		return err
	}

	for _, objName := range objectNames {
		if !bucketWithObj.ObjectMeta.Exist(objName) {
			err = fmt.Errorf("invalid object key '%s' found in the result of the LIST operation. workerID = 0x%x",
				objName, w.id)
			log.Println(err.Error())
			return err
		}
	}

	w.st.AddListCount()
	return nil
}

func (w *Worker) Delete(ctx context.Context) error {
	bucketWithObj := w.selectBucketWithObject()
	obj := bucketWithObj.ObjectMeta.PopExistingRandomObject()
	if obj == nil {
		return nil
	}

	// Validation before delete
	getBeforeBody, err := w.client.GetObject(ctx, bucketWithObj.BucketName, obj.Key)
	if err != nil {
		if errors.Is(err, s3client.ErrNoSuchKey) {
			err = fmt.Errorf("object lost before delete.\nerr: %w\nobj: %v", err, obj)
		}
		log.Println(err.Error())
		return err
	}
	defer getBeforeBody.Close()
	err = pattern.Valid(w.id, bucketWithObj.BucketName, obj, getBeforeBody)
	if err != nil {
		if ctx.Err() == context.Canceled {
			log.Println("Detected the canceled context.")
			return nil
		}
		err = fmt.Errorf("data validation error occurred before delete.\n%w", err)
		log.Println(err.Error())
		return err
	}
	w.st.AddGetForValidCount()

	err = w.client.DeleteObject(ctx, bucketWithObj.BucketName, obj.Key)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	w.st.AddDeleteCount()

	// Validation after delete
	getAfterBody, err := w.client.GetObject(ctx, bucketWithObj.BucketName, obj.Key)
	if err != nil {
		if !errors.Is(err, s3client.ErrNoSuchKey) {
			err = fmt.Errorf("unexpected error occurred. (err = %w)", err)
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
