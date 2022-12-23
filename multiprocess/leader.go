package multiprocess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/peng225/oval/runner"
)

const (
	successMessage = "OK"
)

type StartFollowerParameter struct {
	ID        int
	Context   runner.ExecutionContext
	OpeRatios []float64
	TimeInMs  int64
}

func StartFollower(followerList []string,
	context *runner.ExecutionContext,
	opeRatios []float64, timeInMs int64) error {
	for i, follower := range followerList {
		param := StartFollowerParameter{
			ID:        i + 1,
			Context:   *context,
			OpeRatios: opeRatios,
			TimeInMs:  timeInMs,
		}
		data, err := json.Marshal(param)
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer(data)

		path, err := url.JoinPath(follower, "start")
		if err != nil {
			return err
		}
		resp, err := http.Post(path, "application/json", buf)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("invalid status code. StatusCode = %d", resp.StatusCode)
		}
	}
	return nil
}

func GetResultFromAllFollower(followerList []string) (bool, string, error) {
	report := ""
	successAll := true
	for _, follower := range followerList {
		s, r, err := getResultFromFollower(follower)
		if err != nil {
			return false, "", err
		}
		successAll = successAll && s
		report += r + "\n"
	}
	return successAll, report, nil
}

func getResultFromFollower(follower string) (bool, string, error) {
	report := ""
	var resp *http.Response
	var err error
	for {
		path, err := url.JoinPath(follower, "result")
		if err != nil {
			return false, "", err
		}
		resp, err = http.Get(path)
		if err != nil {
			return false, "", err
		}
		if resp.StatusCode == http.StatusOK {
			break
		} else if resp.StatusCode != http.StatusNoContent {
			return false, "", fmt.Errorf("invalid status code. StatusCode = %d", resp.StatusCode)
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	report += fmt.Sprintf("follower: %s\n", follower)
	report += string(body)
	return (string(body) == successMessage), report, nil
}

func StopFollower(followerList []string) error {
	for _, follower := range followerList {
		path, err := url.JoinPath(follower, "stop")
		if err != nil {
			return err
		}
		resp, err := http.Post(path, "application/json", nil)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("invalid status code. StatusCode = %d", resp.StatusCode)
		}
	}
	return nil
}
