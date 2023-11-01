package multiprocess

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/peng225/oval/runner"
)

type State int

var (
	run            *runner.Runner
	runnerErr      error
	done           chan struct{}
	cancel         chan struct{}
	state          State
	mu             sync.Mutex
	watchDog       int
	caCertFileName string
)

const (
	stopped State = iota
	running
	cancelling
)

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
	runnerErr = nil
	done = make(chan struct{})
	cancel = make(chan struct{})
	watchDog = 0

	go func() {
		run = runner.NewRunner(&param.Context, param.OpeRatio, param.TimeInMs, false, "",
			param.ID, param.MultipartThresh, caCertFileName)
		runnerErr = run.Run(cancel)
		mu.Lock()
		defer mu.Unlock()
		state = stopped
		close(done)
	}()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		previousWatchDog := watchDog
		for {
			select {
			case <-ticker.C:
				if previousWatchDog == watchDog {
					log.Println("Could not receive requests from the leader for some time period.")
					cancelWorkload()
					return
				}
				previousWatchDog = watchDog
			case <-done:
				return
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
	if runnerErr != nil {
		data = []byte(runnerErr.Error())
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

	close(cancel)
	state = cancelling
	log.Println("Canceled the workload.")
}
