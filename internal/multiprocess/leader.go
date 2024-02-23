package multiprocess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/peng225/oval/internal/runner"
)

const (
	successMessage = "OK"
)

type StartFollowerParameter struct {
	ID              int
	Context         runner.ExecutionContext
	OpeRatio        []float64
	TimeInMs        int64
	MultipartThresh int
}

func StartFollower(followerList []string,
	context *runner.ExecutionContext,
	opeRatio []float64, timeInMs int64, multipartThresh int) error {
	for i, follower := range followerList {
		param := StartFollowerParameter{
			ID:              i,
			Context:         *context,
			OpeRatio:        opeRatio,
			TimeInMs:        timeInMs,
			MultipartThresh: multipartThresh,
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

type followerReport struct {
	follower string
	report   string
}

func GetResultFromAllFollower(followerList []string) (bool, map[string]string, error) {
	var returnedErr error
	frCh := make(chan followerReport, len(followerList))
	successAll := true
	canceled := false
	wg := &sync.WaitGroup{}
	wg.Add(len(followerList))
	for _, follower := range followerList {
		go func(follower string) {
			defer wg.Done()
			success, report, err := getResultFromFollower(follower)
			if err != nil {
				if !canceled {
					canceled = true
					returnedErr = err
					successAll = false
					cancelErr := CancelFollowerWorkload(followerList)
					if cancelErr != nil {
						slog.Error("Failed to cancel followers' workload.", "err", cancelErr)
					}
				}
				return
			}
			if !success {
				if !canceled {
					canceled = true
					successAll = false
					cancelErr := CancelFollowerWorkload(followerList)
					if cancelErr != nil {
						slog.Error("Failed to cancel followers' workload.", "err", cancelErr)
					}
				}
			}
			frCh <- followerReport{
				follower: follower,
				report:   report,
			}
		}(follower)
	}
	wg.Wait()

	close(frCh)
	report := make(map[string]string)
	for fr := range frCh {
		report[fr.follower] = fr.report
	}
	return successAll, report, returnedErr
}

func getResultFromFollower(follower string) (bool, string, error) {
	var report string
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
		time.Sleep(500 * time.Millisecond)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	report = string(body)
	return (string(body) == successMessage), report, nil
}

func CancelFollowerWorkload(followerList []string) error {
	var returnedErr error
	for _, follower := range followerList {
		path, err := url.JoinPath(follower, "cancel")
		if err != nil {
			slog.Error(err.Error())
			returnedErr = err
			continue
		}
		resp, err := http.Post(path, "application/octet-stream", nil)
		if err != nil {
			slog.Error(err.Error())
			returnedErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			returnedErr = fmt.Errorf("invalid status code. StatusCode = %d", resp.StatusCode)
			slog.Error(returnedErr.Error())
			continue
		}
	}
	return returnedErr
}
