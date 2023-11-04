package multiprocess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

func InitFollower(followerList []string) error {
	for _, follower := range followerList {
		path, err := url.JoinPath(follower, "init")
		if err != nil {
			return err
		}
		resp, err := http.Post(path, "application/octet-stream", nil)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("invalid status code. StatusCode = %d", resp.StatusCode)
		}
	}
	return nil
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

func GetResultFromAllFollower(followerList []string) (bool, string, error) {
	var returnedErr error
	reportCh := make(chan string, len(followerList))
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
					cancelErr := cancelFollowerWorkload(followerList)
					if cancelErr != nil {
						log.Printf("Failed to cancel followers' workload. err: %v\n", cancelErr)
					}
				}
				return
			}
			if !success {
				if !canceled {
					canceled = true
					successAll = false
					cancelErr := cancelFollowerWorkload(followerList)
					if cancelErr != nil {
						log.Printf("Failed to cancel followers' workload. err: %v\n", cancelErr)
					}
				}
			}
			reportCh <- report
		}(follower)
	}
	wg.Wait()

	close(reportCh)
	report := ""
	for r := range reportCh {
		report += r + "\n"
	}
	return successAll, report, returnedErr
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
		time.Sleep(500 * time.Millisecond)
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

func cancelFollowerWorkload(followerList []string) error {
	var returnedErr error
	for _, follower := range followerList {
		path, err := url.JoinPath(follower, "cancel")
		if err != nil {
			log.Println(err.Error())
			returnedErr = err
			continue
		}
		resp, err := http.Post(path, "application/octet-stream", nil)
		if err != nil {
			log.Println(err.Error())
			returnedErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			returnedErr = fmt.Errorf("invalid status code. StatusCode = %d", resp.StatusCode)
			log.Println(returnedErr.Error())
			continue
		}
	}
	return returnedErr
}
