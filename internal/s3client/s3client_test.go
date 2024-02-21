package s3client

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dataDir = "/tmp/oval-test"

func startMinIO(t *testing.T) {
	t.Helper()

	err := os.Mkdir(dataDir, 0777)
	if err != nil && !os.IsExist(err) {
		require.NoError(t, err)
	}

	ret, err := exec.Command("id", "-u").Output()
	require.NoError(t, err)
	uid := strings.TrimSpace(string(ret))

	ret, err = exec.Command("id", "-g").Output()
	require.NoError(t, err)
	gid := strings.TrimSpace(string(ret))

	cmd := exec.Command("docker", "run", "--user", fmt.Sprintf("%s:%s", uid, gid),
		"-p", "9000:9000",
		"-p", "9090:9090", "--name", "minio",
		"-v", dataDir+":/data",
		"--rm", "-d", "quay.io/minio/minio", "server", "/data",
		"--console-address", ":9090")
	err = cmd.Run()
	require.NoError(t, err)
}

func stopMinIO(t *testing.T) {
	t.Helper()

	cmd := exec.Command("docker", "stop", "minio")
	err := cmd.Run()
	require.NoError(t, err)

	err = os.RemoveAll(dataDir)
	require.NoError(t, err)
}

func TestSuccessCase(t *testing.T) {
	startMinIO(t)
	defer stopMinIO(t)

	client := NewS3Client("http://localhost:9000", "", 1024*1024)
	require.NotNil(t, client)

	ctx := context.Background()
	bucketName := "bucket1"
	err := client.CreateBucket(ctx, bucketName)
	require.NoError(t, err)
	err = client.HeadBucket(ctx, bucketName)
	require.NoError(t, err)

	key := "test-key1"
	partCount, err := client.PutObject(ctx, bucketName, key, []byte("test-data"))
	require.NoError(t, err)
	assert.Equal(t, 1, partCount)

	data, err := client.GetObject(ctx, bucketName, key)
	require.NoError(t, err)
	dataStr, err := io.ReadAll(data)
	require.NoError(t, err)
	assert.Equal(t, []byte("test-data"), dataStr)

	objectNames, err := client.ListObjects(ctx, bucketName, "test")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{key}, objectNames)

	err = client.DeleteObject(ctx, bucketName, key)
	require.NoError(t, err)

	err = client.DeleteObject(ctx, bucketName, key)
	require.NoError(t, err)
}

func TestFailureCase(t *testing.T) {
	startMinIO(t)
	defer stopMinIO(t)

	client := NewS3Client("http://localhost:9000", "", 1024*1024)
	require.NotNil(t, client)

	ctx := context.Background()
	bucketName := "bucket1"
	err := client.HeadBucket(ctx, bucketName)
	assert.ErrorIs(t, err, ErrNotFound)

	err = client.CreateBucket(ctx, bucketName)
	require.NoError(t, err)

	err = client.CreateBucket(ctx, bucketName)
	require.ErrorIs(t, err, ErrConflict)

	key := "test-key1"
	_, err = client.GetObject(ctx, bucketName, key)
	assert.ErrorIs(t, err, ErrNoSuchKey)
}
