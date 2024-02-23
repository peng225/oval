package pattern

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/peng225/oval/internal/object"
)

const (
	dataUnitSize                          = 256
	dataUnitHeaderSizeWithoutBucketAndKey = 20
)

func DecideSize(minSize, maxSize int) (int, error) {
	if minSize < dataUnitSize {
		return 0, fmt.Errorf("minSize should be larger than or equal to %v", dataUnitSize)
	}
	if minSize%dataUnitSize != 0 {
		return 0, fmt.Errorf("minSize should be a multiple of %v", dataUnitSize)
	}
	if maxSize%dataUnitSize != 0 {
		return 0, fmt.Errorf("maxSize should be a multiple of %v", dataUnitSize)
	}
	if maxSize < minSize {
		return 0, errors.New("maxSize should be larger than or equal to minSize")
	}

	/*
		It would be nice if the returned value of this function is
		a random variable whose distribution is something like
		an exponential distribution.

		Because the returned value has both an upper and lower bound,
		the domain of the density function is finite,
		thus it cannot be an exponential distribution.

		However, we can use a similar distribution.

		Suppose X is a random variable whose density function is
			f_X(x) = 2^{1-x}  (0 \le x \lt 1)
		and the cumulative distribution function is
			F_X(x) = 2(1-2^{-x})  (0 \le x \lt 1)

		We can generate samples from this distribution
		by inverse transform sampling.
		cf. https://en.wikipedia.org/wiki/Inverse_transform_sampling

		The inverse function of F_X(x) is
			F_X^{-1}(y) = -log2(1-0.5*y)

		Now generating y from the uniform distribution Unif[0, 1)
		and getting the value of F_X^{-1}(y)
		is equivalent to getting a sample from f_X(x).
	*/
	y := rand.Float64()
	x := -math.Log2(1 - 0.5*y)

	/*
		x is in [0, 1) whose density function is f_X(x).
		We should transform it to the integer value in [minSize, maxSize].
	*/
	return minSize + dataUnitSize*int(float64((maxSize-minSize)/dataUnitSize+1)*x), nil
}

func Generate(dataSize, workerID int, bucketName string, obj *object.Object) ([]byte, error) {
	if len(bucketName) > object.MaxBucketNameLength {
		bucketName = bucketName[:object.MaxBucketNameLength]
	}

	data := make([]byte, 0, dataSize)
	for i := 0; i < dataSize/dataUnitSize; i++ {
		dataUnit, err := generateDataUnit(i, workerID, bucketName, obj)
		if err != nil {
			return nil, err
		}
		data = append(data, dataUnit...)
	}

	if len(data) != dataSize {
		return nil, fmt.Errorf("generated data size is wrong. (expected: %v, actual: %v)", dataSize, len(data))
	}

	return data, nil
}

func generateDataUnit(unitCount, workerID int, bucketName string, obj *object.Object) ([]byte, error) {
	bucketKeyformat := fmt.Sprintf("%%-%vs%%-%vs", object.MaxBucketNameLength, object.MaxKeyLength)
	offsetInObject := unitCount * dataUnitSize
	dataUnit := make([]byte, 0, dataUnitSize)
	dataUnit = append(dataUnit, []byte(fmt.Sprintf(bucketKeyformat, bucketName, obj.Key))...)

	numBinBuf := make([]byte, dataUnitHeaderSizeWithoutBucketAndKey)
	binary.LittleEndian.PutUint32(numBinBuf[0:4], uint32(obj.WriteCount))
	binary.LittleEndian.PutUint32(numBinBuf[4:8], uint32(offsetInObject))
	dt := time.Now()
	unixTime := dt.UnixMicro()
	binary.LittleEndian.PutUint32(numBinBuf[8:], uint32(workerID))
	binary.LittleEndian.PutUint64(numBinBuf[12:], uint64(unixTime))
	dataUnit = append(dataUnit, numBinBuf...)

	unitBodyStartPos := object.MaxBucketNameLength + object.MaxKeyLength + dataUnitHeaderSizeWithoutBucketAndKey
	for i := unitBodyStartPos; i < dataUnitSize; i += 4 {
		dataUnit = append(dataUnit,
			byte(i), byte(i+1), byte(i+2), byte(i+3),
		)
	}
	return dataUnit, nil
}

func Valid(workerID int, expectedBucketName string, obj *object.Object, reader io.Reader) error {
	if len(expectedBucketName) > object.MaxBucketNameLength {
		expectedBucketName = expectedBucketName[:object.MaxBucketNameLength]
	}
	data := make([]byte, dataUnitSize)
	for i := 0; i < obj.Size/dataUnitSize; i++ {
		n, _ := io.ReadFull(reader, data)
		if n != dataUnitSize {
			return fmt.Errorf("could not read some data. (expected: %vbyte, actual: %vbyte)\n%v", dataUnitSize, n, dump(hex.Dump(data[0:n])))
		}
		err := validDataUnit(i, workerID, expectedBucketName, obj, data)
		if err != nil {
			return err
		}
	}
	return nil
}

func validDataUnit(unitCount, workerID int, expectedBucketName string, obj *object.Object, data []byte) error {
	bucketName := data[0:object.MaxBucketNameLength]
	current := object.MaxBucketNameLength
	errMsg := ""
	if expectedBucketName != strings.TrimSpace(string(bucketName)) {
		errMsg += fmt.Sprintf("- Bucket name is wrong. (expected = \"%s\", actual = \"%s\")\n",
			expectedBucketName, strings.TrimSpace(string(bucketName)))
	}

	key := data[current : current+object.MaxKeyLength]
	current = current + object.MaxKeyLength
	if obj.Key != strings.TrimSpace(string(key)) {
		errMsg += fmt.Sprintf("- Key name is wrong. (expected = \"%s\", actual = \"%s\")\n",
			obj.Key, strings.TrimSpace(string(key)))
	}

	writeCount := binary.LittleEndian.Uint32(data[current : current+4])
	current += 4
	if uint32(obj.WriteCount) != writeCount {
		errMsg += fmt.Sprintf("- WriteCount is wrong. (expected = \"%d\", actual = \"%d\")\n",
			obj.WriteCount, writeCount)
	}

	offsetInObject := binary.LittleEndian.Uint32(data[current : current+4])
	current += 4
	if uint32(unitCount*dataUnitSize) != offsetInObject {
		errMsg += fmt.Sprintf("- OffsetInObject is wrong. (expected = \"%d\", actual = \"%d\")\n",
			unitCount*dataUnitSize, offsetInObject)
	}

	actualWorkerID := int(binary.LittleEndian.Uint32(data[current : current+4]))
	if workerID != actualWorkerID {
		errMsg += fmt.Sprintf("- WorkerID is wrong. (expected = \"%d\", actual = \"%d\")\n",
			workerID, actualWorkerID)
	}

	// Skip the unix time area.

	if errMsg != "" {
		errMsg += dump(hex.Dump(data))
		return errors.New(errMsg)
	}

	return nil
}

func dump(data string) string {
	if len(data) == 0 {
		return ""
	}
	const lineSize = 79
	output := ""
	byteExplanation := []string{
		"          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ bucket name\n",
		"          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ key name\n                                               ^^^^^^^^^^^ write count\n",
		"          ^^^^^^^^^^^ byte offset in this object\n                      ^^^^^^^^^^^ worker ID\n                                   ^^^^^^^^^^^^^^^^^^^^^^^ unix time (micro sec)\n",
	}

	for _, exp := range byteExplanation {
		if len(data) < lineSize {
			output += data
			output += exp
			return output
		}
		output += data[0:lineSize]
		output += exp
		data = data[lineSize:]
	}
	output += data
	return output
}
