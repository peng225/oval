package datasource

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/dsnet/golib/memfile"
	"github.com/peng225/oval/object"
)

const (
	dataUnitSize                          = 256
	dataUnitHeaderSizeWithoutBucketAndKey = 16
)

func Generate(minSize, maxSize int, obj *object.Object) (io.ReadSeeker, int, error) {
	if minSize%dataUnitSize != 0 {
		return nil, 0, fmt.Errorf("minSize should be a multiple of %v.", dataUnitSize)
	}
	if maxSize != 0 && maxSize%dataUnitSize != 0 {
		return nil, 0, fmt.Errorf("maxSize should be a multiple of %v.", dataUnitSize)
	}
	if maxSize < minSize {
		return nil, 0, errors.New("maxSize should be larger than minSize.")
	}

	var dataSize int
	dataSize = minSize + dataUnitSize*rand.Intn((maxSize-minSize)/dataUnitSize+1)

	f := memfile.New([]byte{})
	// memfile does not implement io.Closer interface.

	for i := 0; i < dataSize/dataUnitSize; i++ {
		err := generateDataUnit(i, obj, f)
		if err != nil {
			return nil, 0, err
		}
	}

	if len(f.Bytes()) != dataSize {
		return nil, 0, fmt.Errorf("Generated data size is wrong. (expected: %v, actual: %v)", dataSize, f.Bytes())
	}

	_, err := f.Seek(0, 0)
	if err != nil {
		return nil, 0, err
	}

	return f, dataSize, nil
}

func generateDataUnit(unitCount int, obj *object.Object, writer io.Writer) error {
	bucketKeyformat := fmt.Sprintf("%%-%vs%%-%vs", object.MAX_BUCKET_NAME_LENGTH, object.MAX_KEY_LENGTH)
	offsetInObject := unitCount * dataUnitSize
	n, err := writer.Write([]byte(fmt.Sprintf(bucketKeyformat, obj.BucketName, obj.Key)))
	if err != nil {
		return err
	}
	if n != object.MAX_BUCKET_NAME_LENGTH+object.MAX_KEY_LENGTH {
		return errors.New("bucket name and key was not written correctly.")
	}

	numBinBuf := make([]byte, dataUnitHeaderSizeWithoutBucketAndKey)
	binary.LittleEndian.PutUint32(numBinBuf[0:4], uint32(obj.WriteCount))
	binary.LittleEndian.PutUint32(numBinBuf[4:8], uint32(offsetInObject))
	dt := time.Now()
	unixTime := dt.Unix()
	binary.LittleEndian.PutUint64(numBinBuf[8:], uint64(unixTime))
	writer.Write(numBinBuf)

	unitBodyStartPos := object.MAX_BUCKET_NAME_LENGTH + object.MAX_KEY_LENGTH + dataUnitHeaderSizeWithoutBucketAndKey
	tmpData := make([]byte, 4)
	for i := unitBodyStartPos; i < dataUnitSize; i += 4 {
		tmpData[0] = byte(i)
		tmpData[1] = byte(i + 1)
		tmpData[2] = byte(i + 2)
		tmpData[3] = byte(i + 3)
		_, err := writer.Write(tmpData)
		if err != nil {
			return err
		}
	}
	return nil
}

func Valid(obj *object.Object, reader io.Reader) error {
	data := make([]byte, dataUnitSize)
	for i := 0; i < obj.Size/dataUnitSize; i++ {
		readSum := 0
		for readSum != len(data) {
			n, err := reader.Read(data[readSum:])
			if err != nil && err != io.EOF {
				return err
			}
			readSum += n
		}
		if readSum != dataUnitSize {
			return fmt.Errorf("Could not read some data. (expected: %vbyte, actual: %vbyte)\n%v", dataUnitSize, readSum, hex.Dump(data[0:readSum]))
		}
		err := validDataUnit(i, obj, data)
		if err != nil {
			return err
		}
	}
	return nil
}

func validDataUnit(unitCount int, obj *object.Object, data []byte) error {
	bucketName := data[0:object.MAX_BUCKET_NAME_LENGTH]
	current := object.MAX_BUCKET_NAME_LENGTH
	if obj.BucketName != strings.TrimSpace(string(bucketName)) {
		return fmt.Errorf("Bucket name is wrong. (expected = \"%s\", actual = \"%s\")\n%s\n",
			obj.BucketName, strings.TrimSpace(string(bucketName)), hex.Dump(data))
	}

	key := data[current : current+object.MAX_KEY_LENGTH]
	current = current + object.MAX_KEY_LENGTH
	if obj.Key != strings.TrimSpace(string(key)) {
		return fmt.Errorf("Key name is wrong. (expected = \"%s\", actual = \"%s\")\n%s\n",
			obj.Key, strings.TrimSpace(string(key)), hex.Dump(data))
	}

	writeCount := binary.LittleEndian.Uint32(data[current : current+4])
	current = current + 4
	if uint32(obj.WriteCount) != writeCount {
		return fmt.Errorf("WriteCount is wrong. (expected = \"%d\", actual = \"%d\")\n%s\n",
			obj.WriteCount, writeCount, hex.Dump(data))
	}

	offsetInObject := binary.LittleEndian.Uint32(data[current : current+4])
	current = current + 4
	if uint32(unitCount*dataUnitSize) != offsetInObject {
		return fmt.Errorf("OffsetInObject is wrong. (expected = \"%d\", actual = \"%d\")\n%s\n",
			unitCount*dataUnitSize, offsetInObject, hex.Dump(data))
	}

	return nil
}
