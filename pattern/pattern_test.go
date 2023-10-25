package pattern

import (
	"io"
	"testing"

	"github.com/dsnet/golib/memfile"
	"github.com/peng225/oval/object"
	"github.com/stretchr/testify/suite"
)

const (
	testBucketName     = "test-bucket"
	testLongBucketName = "test-long-long-long-bucket"
	testKeyName        = "test-key"
)

/*******************************/
/* Test set up                 */
/*******************************/
type PatternSuite struct {
	suite.Suite
	f io.ReadWriteSeeker
}

func (suite *PatternSuite) SetupTest() {
	suite.f = memfile.New([]byte{})
}

/*******************************/
/* Test cases                  */
/*******************************/
func (suite *PatternSuite) TestGenerateDataUnitSuccess() {
	obj := &object.Object{
		Key:        testKeyName,
		Size:       256,
		WriteCount: 300,
	}
	workerID := 100

	suite.Equal(nil, generateDataUnit(4, workerID, testBucketName, obj, suite.f))
	_, err := suite.f.Seek(0, 0)
	suite.NoError(err)
	data, err := io.ReadAll(suite.f)
	suite.NoError(err)
	suite.Equal(dataUnitSize, len(data))

	// bucketName
	suite.Equal(append([]byte(testBucketName), 0x20, 0x20, 0x20, 0x20, 0x20),
		data[0:object.MaxBucketNameLength])
	current := object.MaxBucketNameLength
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MaxKeyLength])
	current += object.MaxKeyLength
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x04, 0x00, 0x00}, data[current:current+4]) // hex(256*4) = 0x400
	current += 4
	// Check worker ID
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64
}

func (suite *PatternSuite) TestGenerateSuccess() {
	obj := &object.Object{
		Key:        testKeyName,
		Size:       256,
		WriteCount: 300,
	}
	workerID := 100

	size := 512
	readSeeker, err := Generate(size, workerID, 0, testBucketName, obj)
	suite.NoError(err)
	suite.Equal(512, size)
	data, err := io.ReadAll(readSeeker)
	suite.NoError(err)

	// 1st data unit
	// bucketName
	suite.Equal(append([]byte(testBucketName), 0x20, 0x20, 0x20, 0x20, 0x20),
		data[0:object.MaxBucketNameLength])
	current := object.MaxBucketNameLength
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MaxKeyLength])
	current += object.MaxKeyLength
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x00, 0x00, 0x00}, data[current:current+4])
	current += 4
	// Check worker ID
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64

	// 2nd data unit
	current = dataUnitSize
	// bucketName
	suite.Equal(append([]byte(testBucketName), 0x20, 0x20, 0x20, 0x20, 0x20),
		data[current:current+object.MaxBucketNameLength])
	current += object.MaxBucketNameLength
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MaxKeyLength])
	current += object.MaxKeyLength
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(256*1) = 0x100
	current += 4
	// Check worker ID
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64
}

func (suite *PatternSuite) TestGenerateLongBucketName() {
	obj := &object.Object{
		Key:        testKeyName,
		Size:       256,
		WriteCount: 300,
	}
	workerID := 100

	size := 512
	readSeeker, err := Generate(size, workerID, 0, testLongBucketName, obj)
	suite.NoError(err)
	suite.Equal(512, size)
	data, err := io.ReadAll(readSeeker)
	suite.NoError(err)

	// 1st data unit
	// bucketName
	suite.Equal([]byte(testLongBucketName[:object.MaxBucketNameLength]),
		data[0:object.MaxBucketNameLength])
	current := object.MaxBucketNameLength
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MaxKeyLength])
	current += object.MaxKeyLength
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x00, 0x00, 0x00}, data[current:current+4])
	current += 4
	// Check worker ID
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64

	// 2nd data unit
	current = dataUnitSize
	// bucketName
	suite.Equal([]byte(testLongBucketName[:object.MaxBucketNameLength]),
		data[current:current+object.MaxBucketNameLength])
	current += object.MaxBucketNameLength
	// keyName
	suite.Equal(append([]byte(testKeyName), 0x20, 0x20, 0x20, 0x20), data[current:current+object.MaxKeyLength])
	current += object.MaxKeyLength
	// Check write count
	suite.Equal([]byte{0x2c, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(300) = 0x12c
	current += 4
	// Check offset
	suite.Equal([]byte{0x00, 0x01, 0x00, 0x00}, data[current:current+4]) // hex(256*1) = 0x100
	current += 4
	// Check worker ID
	suite.Equal([]byte{0x64, 0x00, 0x00, 0x00}, data[current:current+4]) // hex(100) = 0x64
}

func (suite *PatternSuite) TestValidDataUnitSuccess() {
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

func (suite *PatternSuite) TestValidSuccess() {
	obj := &object.Object{
		Key:        testKeyName,
		WriteCount: 300,
	}
	workerID := 100

	size := 1024
	readSeeker, err := Generate(size, workerID, 0, testBucketName, obj)
	suite.NoError(err)
	obj.Size = size

	err = Valid(workerID, testBucketName, obj, readSeeker)
	suite.NoError(err)
}

func (suite *PatternSuite) TestValidLongBucketName() {
	obj := &object.Object{
		Key:        testKeyName,
		WriteCount: 300,
	}
	workerID := 100

	size := 1024
	readSeeker, err := Generate(size, workerID, 0, testLongBucketName, obj)
	suite.NoError(err)
	obj.Size = size

	err = Valid(workerID, testLongBucketName, obj, readSeeker)
	suite.NoError(err)
}

func (suite *PatternSuite) TestDecideSize() {
	type testCase struct {
		minSize     int
		maxSize     int
		expectedErr bool
	}
	testCases := []testCase{
		{
			minSize:     512,
			maxSize:     512,
			expectedErr: false,
		},
		{
			minSize:     1024,
			maxSize:     5 * 1024 * 1024,
			expectedErr: false,
		},
		{
			minSize:     256,
			maxSize:     1024,
			expectedErr: false,
		},
		{
			minSize:     513,
			maxSize:     1024,
			expectedErr: true,
		},
		{
			minSize:     512,
			maxSize:     513,
			expectedErr: true,
		},
		{
			minSize:     0,
			maxSize:     512,
			expectedErr: true,
		},
		{
			minSize:     1024,
			maxSize:     512,
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		size, err := DecideSize(tc.minSize, tc.maxSize)
		if tc.expectedErr {
			suite.Errorf(err, "tc.minSize: %d, tc.maxSize: %d", tc.minSize, tc.maxSize)
			continue
		}
		suite.GreaterOrEqual(size, tc.minSize)
		suite.LessOrEqual(size, tc.maxSize)
	}
}

/*******************************/
/* Run tests                   */
/*******************************/
func TestPatternSuite(t *testing.T) {
	suite.Run(t, new(PatternSuite))
}
