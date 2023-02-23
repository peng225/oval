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
	run       *runner.Runner
	runnerErr error
	cancel    chan struct{}
	done      chan struct{}
	state     State
	mu        sync.Mutex
	watchDog  int
)

const (
	initial State = iota
	running
	canceled
	finished
)

func StartServer(port int) {
	portStr := strconv.Itoa(port)

	http.HandleFunc("/init", initHandler)
	http.HandleFunc("/start", startHandler)
	http.HandleFunc("/result", resultHandler)
	http.HandleFunc("/cancel", cancelHandler)
	log.Printf("Start server. port = %s\n", portStr)
	log.Println(http.ListenAndServe(":"+portStr, nil))
}

func initHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a init request.")
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	state = initial
	runnerErr = nil
	cancel = make(chan struct{})
	done = make(chan struct{})
	watchDog = 0
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a start request.")
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if state != initial {
		log.Printf("Invalid state %d.\n", state)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	var param StartFollowerParameter
	err = json.Unmarshal(body, &param)
	if err != nil {
		log.Fatal(err)
	}
	printStartFollowerParameter(&param)

	state = running

	go func() {
		run = runner.NewRunner(&param.Context, param.OpeRatio, param.TimeInMs, false, "", param.ID, param.MultipartThresh)
		runnerErr = run.Run(cancel)
		mu.Lock()
		defer mu.Unlock()
		state = finished
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
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	watchDog += 1

	if state != finished {
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
		log.Fatal(err)
	} else if writtenLength != len(data) {
		log.Fatalf("Invalid written length. writtenLength = %v", writtenLength)
	}
}

func cancelHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a cancel request.")
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	_, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	cancelWorkload()
}

func cancelWorkload() {
	mu.Lock()
	defer mu.Unlock()
	if state != running {
		log.Printf("Invalid state %d.\n", state)
		return
	}

	close(cancel)
	if state != finished {
		state = canceled
	}
	log.Println("Canceled the workload.")
}
