package object

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
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

// TODO: create the instance for each goroutine
type ObjectList struct {
	objectList        []Object
	existingObjectIDs []int
	keyIDOffset       int
}

func (obj *Object) Clear() {
	obj.Size = 0
	obj.WriteCount = 0
}

func NewObject(bucketName string, id int) *Object {
	return &Object{
		Key:        generateKey(id),
		Size:       0,
		WriteCount: 0,
		BucketName: bucketName,
	}
}

func generateKey(id int) string {
	currentId := id
	return fmt.Sprintf("ov%010d", currentId)
}

func (ol *ObjectList) Init(bucketName string, numObj, keyIDOffset int) {
	ol.objectList = make([]Object, numObj)
	for objId := 0; objId < numObj; objId++ {
		ol.objectList[objId] = *NewObject(bucketName, keyIDOffset+objId)
	}
	ol.existingObjectIDs = make([]int, 0, int(math.Sqrt(float64(numObj))))
	ol.keyIDOffset = keyIDOffset
}

func (ol *ObjectList) GetRandomObject() *Object {
	objId := rand.Intn(len(ol.objectList))
	return &ol.objectList[objId]
}

// Caution: this function should be called while the for the object lock is acquired.
func (ol *ObjectList) RegisterToExistingList(key string) {
	objId, err := strconv.Atoi(key[2:])
	if err != nil {
		log.Fatal(err)
	}
	objId -= ol.keyIDOffset
	for _, eoId := range ol.existingObjectIDs {
		if eoId == objId {
			// The key is already registered.
			return
		}
	}
	ol.existingObjectIDs = append(ol.existingObjectIDs, objId)
	if len(ol.objectList) < len(ol.existingObjectIDs) {
		log.Fatal("Invalid contents of existing object ID list.")
	}
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
