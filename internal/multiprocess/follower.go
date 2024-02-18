package multiprocess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/peng225/oval/internal/runner"
)

type State int

var (
	run               *runner.Runner
	resultErr         error
	stopWithCause     context.CancelCauseFunc
	state             State
	mu                sync.Mutex
	watchDog          int
	caCertFileName    string
	workloadCancelErr error
)

const (
	stopped State = iota
	running
	cancelling
)

func init() {
	workloadCancelErr = errors.New("workload cancel requested")
}

func StartServer(port int, cert string) {
	portStr := strconv.Itoa(port)
	caCertFileName = cert

	http.HandleFunc("/start", startHandler)
	http.HandleFunc("/result", resultHandler)
	http.HandleFunc("/cancel", cancelHandler)
	log.Printf("Start server. port = %s\n", portStr)
	log.Println(http.ListenAndServe(":"+portStr, nil))
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a start request.")
	defer func() {
		if r.Body != nil {
			r.Body.Close()
		}
	}()
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if state == running {
		// This block exists to make startHandler idempotent.
		log.Println("Workload is already running.")
		return
	} else if state != stopped {
		log.Printf("Invalid state. (state=%v)", state)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var param StartFollowerParameter
	err = json.Unmarshal(body, &param)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	printStartFollowerParameter(&param)

	state = running
	resultErr = nil
	watchDog = 0

	var ctx context.Context
	var stop context.CancelFunc
	ctx, stop = signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	ctx, stopWithCause = context.WithCancelCause(ctx)
	go func() {
		defer stop()
		run = runner.NewRunner(&param.Context, param.OpeRatio, param.TimeInMs, false, "",
			param.ID, param.MultipartThresh, caCertFileName)
		err := run.InitBucket(ctx)
		if err != nil {
			resultErr = fmt.Errorf("run.InitBucket() failed. %w", err)
			log.Println(resultErr)
		} else {
			resultErr = run.Run(ctx)
		}
		// When the workload has been stopped by signals,
		// exit gracefully.
		// FIXME: I want to check the condition
		//        in a more specific way.
		if context.Cause(ctx) == context.Canceled {
			log.Println("Follower is going to stop.")
			os.Exit(0)
		}
		mu.Lock()
		defer mu.Unlock()
		state = stopped
	}()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		previousWatchDog := watchDog
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if previousWatchDog == watchDog {
					log.Println("Could not receive requests from the leader for some time period.")
					cancelWorkload()
					return
				}
				previousWatchDog = watchDog
			}
		}
	}()
}

func printStartFollowerParameter(param *StartFollowerParameter) {
	log.Printf("ID: %d\n", param.ID)
	log.Printf("Context: %v\n", param.Context)
	log.Printf("OpeRatio: %v\n", param.OpeRatio)
	log.Printf("TimeInMs: %v\n", param.TimeInMs)
	log.Printf("MultipartThresh: %v\n", param.MultipartThresh)
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	watchDog += 1

	if state != stopped {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var writtenLength int
	data := []byte(successMessage)
	if resultErr != nil {
		data = []byte(resultErr.Error())
	}
	writtenLength, err := w.Write(data)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	} else if writtenLength != len(data) {
		log.Printf("Invalid written length. writtenLength = %d\n", writtenLength)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func cancelHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a cancel request.")
	defer func() {
		if r.Body != nil {
			r.Body.Close()
		}
	}()
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	_, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	cancelWorkload()
}

func cancelWorkload() {
	mu.Lock()
	defer mu.Unlock()
	if state != running {
		log.Printf("Workload is not running. (state = %v)\n", state)
		return
	}

	stopWithCause(workloadCancelErr)
	state = cancelling
	log.Println("Canceled the workload.")
}
