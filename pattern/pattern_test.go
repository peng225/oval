package pattern

import (
	"io"
	"testing"

	"github.com/dsnet/golib/memfile"
	"github.com/peng225/oval/object"
	"github.com/stretchr/testify/suite"
)

const (
	testBucketName = "test-bucket"
	testKeyName    = "test-key"
)

/*******************************/
/* Test set up                 */
/*******************************/
type GeneratorSuite struct {
	suite.Suite
	f io.ReadWriteSeeker
}

func (suite *GeneratorSuite) SetupTest() {
	suite.f = memfile.New([]byte{})
}

/*******************************/
/* Test cases                  */
/*******************************/
func (suite *GeneratorSuite) TestGenerateDataUnitSuccess() {
	obj := &object.Object{
		Key:        testKeyName,
		Size:       256,
		WriteCount: 300,
	}
	workerID := 100

	suite.Equal(nil, generateDataUnit(4, workerID, testBucketName, obj, suite.f))
	suite.f.Seek(0, 0)
	data, err := io.ReadAll(suite.f)
	suite.NoError(err)
	suite.Equal(dataUnitSize, len(data))

	// bucketName
	suite.Equal(append([]byte(testBucketName), 0x20), data[0:object.MAX_BUCKET_NAME_LENGTH])
	current := object.MAX_BUCKET_NAME_LENGTH
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MAX_KEY_LENGTH])
	current += object.MAX_KEY_LENGTH
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x04, 0x00, 0x00}, data[current:current+4]) // hex(256*4) = 0x400
	current += 4
	current += 8 // Skip unix time area
	// Check offset
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64
}

func (suite *GeneratorSuite) TestGenerateSuccess() {
	obj := &object.Object{
		Key:        testKeyName,
		Size:       256,
		WriteCount: 300,
	}
	workerID := 100

	readSeeker, size, err := Generate(512, 512, workerID, testBucketName, obj)
	suite.NoError(err)
	suite.Equal(512, size)
	data, err := io.ReadAll(readSeeker)
	suite.NoError(err)

	// 1st data unit
	// bucketName
	suite.Equal(append([]byte(testBucketName), 0x20), data[0:object.MAX_BUCKET_NAME_LENGTH])
	current := object.MAX_BUCKET_NAME_LENGTH
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MAX_KEY_LENGTH])
	current += object.MAX_KEY_LENGTH
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x00, 0x00, 0x00}, data[current:current+4])
	current += 4
	current += 8 // Skip unix time area
	// Check offset
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64

	// 2nd data unit
	current = dataUnitSize
	// bucketName
	suite.Equal(append([]byte(testBucketName), 0x20), data[current:current+object.MAX_BUCKET_NAME_LENGTH])
	current += object.MAX_BUCKET_NAME_LENGTH
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MAX_KEY_LENGTH])
	current += object.MAX_KEY_LENGTH
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(256*1) = 0x100
	current += 4
	current += 8 // Skip unix time area
	// Check offset
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64
}

func (suite *GeneratorSuite) TestValidDataUnitSuccess() {
	obj := &object.Object{
		Key:        testKeyName,
		Size:       256,
		WriteCount: 300,
	}
	workerID := 100

	err := generateDataUnit(4, workerID, testBucketName, obj, suite.f)
	suite.NoError(err)
	_, err = suite.f.Seek(0, 0)
	suite.NoError(err)
	data, err := io.ReadAll(suite.f)
	suite.NoError(err)
	suite.Equal(nil, validDataUnit(4, workerID, testBucketName, obj, data))
}

func (suite *GeneratorSuite) TestValidSuccess() {
	obj := &object.Object{
		Key:        testKeyName,
		WriteCount: 300,
	}
	workerID := 100

	readSeeker, size, err := Generate(1024, 1024, workerID, testBucketName, obj)
	suite.NoError(err)
	obj.Size = size

	err = Valid(workerID, testBucketName, obj, readSeeker)
	suite.NoError(err)
}

/*******************************/
/* Run tests                   */
/*******************************/
func TestGenerateSuite(t *testing.T) {
	suite.Run(t, new(GeneratorSuite))
}
