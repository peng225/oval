package object

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"sync"
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
	existingObjectIDs []int
	objMu             []sync.Mutex
	eoiMu             sync.Mutex
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

func (ol *ObjectList) Init(bucketName string, numObj int) {
	for objId := 0; objId < numObj; objId++ {
		ol.objectList = append(ol.objectList, *NewObject(bucketName, objId))
	}
	ol.objMu = make([]sync.Mutex, int(math.Sqrt(float64(numObj))))
}

func (ol *ObjectList) GetRandomObject() (*Object, *sync.Mutex) {
	for {
		objId := rand.Intn(len(ol.objectList))
		mu := &ol.objMu[objId%len(ol.objMu)]
		if mu.TryLock() {
			return &ol.objectList[objId], mu
		}
	}
}

// Caution: this function should be called while the for the object lock is acquired.
func (ol *ObjectList) RegisterToExistingList(key string) {
	objId, err := strconv.Atoi(key[2:])
	if err != nil {
		log.Fatal(err)
	}
	ol.eoiMu.Lock()
	defer ol.eoiMu.Unlock()
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

func (ol *ObjectList) PopExistingRandomObject() (*Object, *sync.Mutex) {
	for {
		ol.eoiMu.Lock()
		if len(ol.existingObjectIDs) == 0 {
			ol.eoiMu.Unlock()
			return nil, nil
		}
		existingObjId := rand.Intn(len(ol.existingObjectIDs))
		objId := ol.existingObjectIDs[existingObjId]
		mu := &ol.objMu[objId%len(ol.objMu)]
		if mu.TryLock() {
			// Delete the `existingObjId`-th entry from existing object id list
			ol.existingObjectIDs[existingObjId] = ol.existingObjectIDs[len(ol.existingObjectIDs)-1]
			ol.existingObjectIDs = ol.existingObjectIDs[:len(ol.existingObjectIDs)-1]
			ol.eoiMu.Unlock()
			return &ol.objectList[objId], mu
		}
		ol.eoiMu.Unlock()
	}
}

func (ol *ObjectList) Exist(key string) bool {
	ol.eoiMu.Lock()
	defer ol.eoiMu.Unlock()
	for _, id := range ol.existingObjectIDs {
		if key == ol.objectList[id].Key {
			return true
		}
	}
	return false
}
