package object

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
)

const (
	MaxBucketNameLength = 16
	MaxKeyLength        = 12
	KeyShortPrefix      = "ov"
	// len(KeyShortPrefix) + 2 (for process ID) + 2 (for worker ID)
	KeyPrefixLength = 6
)

type Object struct {
	Key        string `json:"key"`
	Size       int    `json:"size"`
	WriteCount int    `json:"writeCount"`
}

type ObjectMeta struct {
	ObjectList          []*Object `json:"objectList"`
	ExistingObjectIDs   []int64   `json:"existingObjectIDs"`
	existingObjectIDMap map[int64]struct{}
	KeyIDOffset         int64 `json:"keyIDOffset"`
	KeyPrefix           string
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
	return fmt.Sprintf("%s%010x", KeyShortPrefix, objID)
}

func getObjIDFromKey(key string) (int64, error) {
	return strconv.ParseInt(key[KeyPrefixLength:], 16, 64)

}

func NewObjectMeta(numObj int, keyIDOffset int64) *ObjectMeta {
	om := &ObjectMeta{}
	om.ObjectList = make([]*Object, numObj)
	for objID := 0; objID < numObj; objID++ {
		om.ObjectList[objID] = NewObject(keyIDOffset + int64(objID))
	}
	om.ExistingObjectIDs = make([]int64, 0, int(math.Sqrt(float64(numObj))))
	om.existingObjectIDMap = make(map[int64]struct{})
	om.KeyIDOffset = keyIDOffset
	om.KeyPrefix = generateKey(keyIDOffset)[:KeyPrefixLength]

	return om
}

func (om *ObjectMeta) GetRandomObject() *Object {
	objID := rand.Intn(len(om.ObjectList))
	return om.ObjectList[objID]
}

func (om *ObjectMeta) RegisterToExistingList(key string) {
	objID, err := getObjIDFromKey(key)
	if err != nil {
		log.Fatal(err)
	}
	if _, ok := om.existingObjectIDMap[objID]; ok {
		// The key is already registered.
		return
	}
	om.ExistingObjectIDs = append(om.ExistingObjectIDs, objID)
	om.existingObjectIDMap[objID] = struct{}{}
	if len(om.ObjectList) < len(om.ExistingObjectIDs) {
		log.Fatal("Invalid contents of existing object ID list.")
	}
}

func (om *ObjectMeta) PopExistingRandomObject() *Object {
	if len(om.ExistingObjectIDs) == 0 {
		return nil
	}
	eoIDIndex := rand.Intn(len(om.ExistingObjectIDs))

	objID := om.ExistingObjectIDs[eoIDIndex]
	// Delete the `existingObjID`-th entry from existing object ID list
	om.ExistingObjectIDs[eoIDIndex] = om.ExistingObjectIDs[len(om.ExistingObjectIDs)-1]
	om.ExistingObjectIDs = om.ExistingObjectIDs[:len(om.ExistingObjectIDs)-1]
	if _, ok := om.existingObjectIDMap[objID]; !ok {
		log.Fatalf("objID 0x%x found in ExistingObjectIDs, but not in existingObjectIDMap.", objID)
	}
	delete(om.existingObjectIDMap, objID)
	return om.ObjectList[objID]
}

func (om *ObjectMeta) GetExistingRandomObject() *Object {
	if len(om.ExistingObjectIDs) == 0 {
		return nil
	}
	eoIDIndex := rand.Intn(len(om.ExistingObjectIDs))

	objID := om.ExistingObjectIDs[eoIDIndex]
	return om.ObjectList[objID]
}

func (om *ObjectMeta) Exist(key string) bool {
	objID, err := getObjIDFromKey(key)
	if err != nil {
		log.Fatal(err)
	}
	_, ok := om.existingObjectIDMap[objID]
	return ok
}

func (om *ObjectMeta) GetHeadAndTailKey() (string, string) {
	return om.ObjectList[0].Key, om.ObjectList[len(om.ObjectList)-1].Key
}

func (om *ObjectMeta) TidyUp() {
	om.existingObjectIDMap = make(map[int64]struct{})
	for _, objID := range om.ExistingObjectIDs {
		om.existingObjectIDMap[objID] = struct{}{}
	}
}
