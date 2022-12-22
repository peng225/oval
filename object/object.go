package object

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
)

const (
	MAX_BUCKET_NAME_LENGTH = 16
	MAX_KEY_LENGTH         = 12
	KeyPrefix              = "ov"
)

type Object struct {
	Key        string `json:"key"`
	Size       int    `json:"size"`
	WriteCount int    `json:"writeCount"`
}

type ObjectMeta struct {
	ObjectList        []Object `json:"objectList"`
	ExistingObjectIDs []int64  `json:"existingObjectIDs"`
	KeyIDOffset       int64    `json:"keyIDOffset"`
}

func (obj *Object) Clear() {
	obj.Size = 0
	obj.WriteCount = 0
}

func NewObject(objID int64) *Object {
	return &Object{
		Key:        generateKey(objID),
		Size:       0,
		WriteCount: 0,
	}
}

func generateKey(objID int64) string {
	return fmt.Sprintf("%s%010x", KeyPrefix, objID)
}

func NewObjectMeta(numObj, keyIDOffset int64) *ObjectMeta {
	om := &ObjectMeta{}
	om.ObjectList = make([]Object, numObj)
	for objID := int64(0); objID < numObj; objID++ {
		om.ObjectList[objID] = *NewObject(keyIDOffset + objID)
	}
	om.ExistingObjectIDs = make([]int64, 0, int(math.Sqrt(float64(numObj))))
	om.KeyIDOffset = keyIDOffset

	return om
}

func (om *ObjectMeta) GetRandomObject() *Object {
	objID := rand.Intn(len(om.ObjectList))
	return &om.ObjectList[objID]
}

// Caution: this function should be called while the object lock is acquired.
func (om *ObjectMeta) RegisterToExistingList(key string) {
	objID, err := strconv.ParseInt(key[len(KeyPrefix):], 16, 64)
	if err != nil {
		log.Fatal(err)
	}
	objID -= om.KeyIDOffset
	for _, eoID := range om.ExistingObjectIDs {
		if eoID == objID {
			// The key is already registered.
			return
		}
	}
	om.ExistingObjectIDs = append(om.ExistingObjectIDs, objID)
	if len(om.ObjectList) < len(om.ExistingObjectIDs) {
		log.Fatal("Invalid contents of existing object ID list.")
	}
}

func (om *ObjectMeta) PopExistingRandomObject() *Object {
	if len(om.ExistingObjectIDs) == 0 {
		return nil
	}
	existingObjID := rand.Intn(len(om.ExistingObjectIDs))

	objID := om.ExistingObjectIDs[existingObjID]
	// Delete the `existingObjID`-th entry from existing object ID list
	om.ExistingObjectIDs[existingObjID] = om.ExistingObjectIDs[len(om.ExistingObjectIDs)-1]
	om.ExistingObjectIDs = om.ExistingObjectIDs[:len(om.ExistingObjectIDs)-1]
	return &om.ObjectList[objID]
}

func (om *ObjectMeta) GetExistingRandomObject() *Object {
	if len(om.ExistingObjectIDs) == 0 {
		return nil
	}
	existingObjID := rand.Intn(len(om.ExistingObjectIDs))

	objID := om.ExistingObjectIDs[existingObjID]
	return &om.ObjectList[objID]
}

func (om *ObjectMeta) Exist(key string) bool {
	for _, id := range om.ExistingObjectIDs {
		if key == om.ObjectList[id].Key {
			return true
		}
	}
	return false
}

func (om *ObjectMeta) GetHeadAndTailKey() (string, string) {
	return om.ObjectList[0].Key, om.ObjectList[len(om.ObjectList)-1].Key
}
