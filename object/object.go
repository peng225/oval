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
}

type ObjectMeta struct {
	ObjectList        []Object
	ExistingObjectIDs []int
	KeyIDOffset       int
}

func (obj *Object) Clear() {
	obj.Size = 0
	obj.WriteCount = 0
}

func NewObject(id int) *Object {
	return &Object{
		Key:        generateKey(id),
		Size:       0,
		WriteCount: 0,
	}
}

func generateKey(id int) string {
	currentId := id
	return fmt.Sprintf("ov%010d", currentId)
}

func (ol *ObjectMeta) Init(numObj, keyIDOffset int) {
	ol.ObjectList = make([]Object, numObj)
	for objId := 0; objId < numObj; objId++ {
		ol.ObjectList[objId] = *NewObject(keyIDOffset + objId)
	}
	ol.ExistingObjectIDs = make([]int, 0, int(math.Sqrt(float64(numObj))))
	ol.KeyIDOffset = keyIDOffset
}

func (ol *ObjectMeta) GetRandomObject() *Object {
	objId := rand.Intn(len(ol.ObjectList))
	return &ol.ObjectList[objId]
}

// Caution: this function should be called while the for the object lock is acquired.
func (ol *ObjectMeta) RegisterToExistingList(key string) {
	objId, err := strconv.Atoi(key[2:])
	if err != nil {
		log.Fatal(err)
	}
	objId -= ol.KeyIDOffset
	for _, eoId := range ol.ExistingObjectIDs {
		if eoId == objId {
			// The key is already registered.
			return
		}
	}
	ol.ExistingObjectIDs = append(ol.ExistingObjectIDs, objId)
	if len(ol.ObjectList) < len(ol.ExistingObjectIDs) {
		log.Fatal("Invalid contents of existing object ID list.")
	}
}

func (ol *ObjectMeta) PopExistingRandomObject() *Object {
	if len(ol.ExistingObjectIDs) == 0 {
		return nil
	}
	existingObjId := rand.Intn(len(ol.ExistingObjectIDs))

	objId := ol.ExistingObjectIDs[existingObjId]
	// Delete the `existingObjId`-th entry from existing object id list
	ol.ExistingObjectIDs[existingObjId] = ol.ExistingObjectIDs[len(ol.ExistingObjectIDs)-1]
	ol.ExistingObjectIDs = ol.ExistingObjectIDs[:len(ol.ExistingObjectIDs)-1]
	return &ol.ObjectList[objId]
}

func (ol *ObjectMeta) GetExistingRandomObject() *Object {
	if len(ol.ExistingObjectIDs) == 0 {
		return nil
	}
	existingObjId := rand.Intn(len(ol.ExistingObjectIDs))

	objId := ol.ExistingObjectIDs[existingObjId]
	return &ol.ObjectList[objId]
}

func (ol *ObjectMeta) Exist(key string) bool {
	for _, id := range ol.ExistingObjectIDs {
		if key == ol.ObjectList[id].Key {
			return true
		}
	}
	return false
}

func (ol *ObjectMeta) GetHeadAndTailKey() (string, string) {
	return ol.ObjectList[0].Key, ol.ObjectList[len(ol.ObjectList)-1].Key
}
