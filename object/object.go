package object

import (
	"fmt"
	"sync"
)

const (
	MAX_BUCKET_NAME_LENGTH = 12
	MAX_KEY_LENGTH         = 16
)

type Object struct {
	Key        string
	Size       int
	WriteCount int
	Worker     int
	BucketName string
}

type ObjectFactory struct {
	id   int64
	idMu sync.Mutex
}

func (of *ObjectFactory) NewObject(bucketName string) *Object {
	return &Object{
		Key:        of.generateKey(),
		Size:       0,
		WriteCount: 0,
		BucketName: bucketName,
	}
}

func (of *ObjectFactory) generateKey() string {
	of.idMu.Lock()
	currentId := of.id
	of.id++
	of.idMu.Unlock()
	return fmt.Sprintf("ov%010d", currentId)
}
