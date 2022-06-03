package object

import (
	"fmt"
	"math/rand"
)

const (
	MAX_BUCKET_NAME_LENGTH = 12
	MAX_KEY_LENGTH         = 12
)

type Object struct {
	Key        string
	Size       int
	WriteCount int
	BucketName string
}

type ObjectList struct {
	objectList        []Object
	existingObjectIDs []int64
}

func NewObject(bucketName string, id int64) *Object {
	return &Object{
		Key:        generateKey(id),
		Size:       0,
		WriteCount: 0,
		BucketName: bucketName,
	}
}

func generateKey(id int64) string {
	currentId := id
	return fmt.Sprintf("ov%010d", currentId)
}

func (ol *ObjectList) Init(bucketName string, numObj int64) {
	for objId := int64(0); objId < numObj; objId++ {
		ol.objectList = append(ol.objectList, *NewObject(bucketName, objId))
	}
}

// TODO: thread safety
func (ol *ObjectList) GetRandomObject() *Object {
	return &ol.objectList[rand.Intn(len(ol.objectList))]
}

func (ol *ObjectList) PopExistingRandomObject() *Object {
	if len(ol.existingObjectIDs) == 0 {
		return nil
	}
	existingObjId := rand.Intn(len(ol.existingObjectIDs))
	objId := ol.existingObjectIDs[existingObjId]
	// Delete the `existingObjId`-th entry from existing object id list
	ol.existingObjectIDs[existingObjId] = ol.existingObjectIDs[len(ol.existingObjectIDs)-1]
	ol.existingObjectIDs = ol.existingObjectIDs[:len(ol.existingObjectIDs)-1]

	return &ol.objectList[objId]
}

func (ol *ObjectList) Exist(key string) bool {
	for _, id := range ol.existingObjectIDs {
		if key == ol.objectList[id].Key {
			return true
		}
	}
	return false
}
