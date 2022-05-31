package datasource

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/peng225/oval/object"
)

const (
	DATA_UNIT_SIZE = 256
)

func Generate(minSize, maxSize int, obj *object.Object) (io.ReadSeeker, int, error) {
	if minSize%DATA_UNIT_SIZE != 0 {
		return nil, 0, fmt.Errorf("minSize should be a multiple of %v.", DATA_UNIT_SIZE)
	}
	if maxSize != 0 && maxSize%DATA_UNIT_SIZE != 0 {
		return nil, 0, fmt.Errorf("maxSize should be a multiple of %v.", DATA_UNIT_SIZE)
	}
	if maxSize != 0 && maxSize < minSize {
		return nil, 0, errors.New("maxSize should be larger than minSize.")
	}

	var dataSize int
	if maxSize == 0 {
		dataSize = minSize
	} else {
		// TODO: implement
		dataSize = minSize
	}

	buf := bytes.NewBuffer(make([]byte, 0))
	for i := 0; i < dataSize/DATA_UNIT_SIZE; i++ {
		generateDataUnit(i, obj, buf)
	}

	if len(buf.Bytes()) != dataSize {
		log.Fatal("Generated data size is wrong.")
	}

	// TODO: would not like to write to tmp file
	file, err := os.CreateTemp("", "ov")
	if err != nil {
		log.Fatal(err)
	}
	file.Write(buf.Bytes())
	file.Seek(0, 0)

	return file, dataSize, nil
}

func generateDataUnit(unitCount int, obj *object.Object, buf *bytes.Buffer) {
	bucketKeyformat := fmt.Sprintf("%%-%vs%%-%vs", object.MAX_BUCKET_NAME_LENGTH, object.MAX_KEY_LENGTH)
	offsetInObject := unitCount * DATA_UNIT_SIZE
	n, err := buf.WriteString(fmt.Sprintf(bucketKeyformat, obj.BucketName, obj.Key))
	if err != nil {
		log.Fatal(err)
	}
	if n != object.MAX_BUCKET_NAME_LENGTH+object.MAX_KEY_LENGTH {
		log.Fatal("bucket name and key was not written correctly.")
	}

	const DATA_UNIT_HEADER_SIZE_WITHOUT_BUCKET_AND_KEY = 10
	numBinBuf := make([]byte, DATA_UNIT_HEADER_SIZE_WITHOUT_BUCKET_AND_KEY)
	binary.LittleEndian.PutUint16(numBinBuf[:2], uint16(obj.Worker))
	binary.LittleEndian.PutUint32(numBinBuf[2:6], uint32(obj.WriteCount))
	binary.LittleEndian.PutUint32(numBinBuf[6:], uint32(offsetInObject))
	buf.Write(numBinBuf)

	unitBodyStartPos := object.MAX_BUCKET_NAME_LENGTH + object.MAX_KEY_LENGTH + DATA_UNIT_HEADER_SIZE_WITHOUT_BUCKET_AND_KEY
	for i := unitBodyStartPos; i < DATA_UNIT_SIZE; i++ {
		buf.WriteByte(byte(i % 256))
	}
}

func Validate(data io.Reader) (bool, error) {
	return true, nil
}
