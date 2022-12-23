package multiprocess

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/peng225/oval/runner"
)

var (
	run   *runner.Runner
	errCh chan error
)

func init() {
	errCh = make(chan error)
}

func StartServer(port int) {
	portStr := strconv.Itoa(port)

	http.HandleFunc("/start", startHandler)
	http.HandleFunc("/result", resultHandler)
	http.HandleFunc("/stop", stopHandler)
	log.Printf("Start server. port = %s\n", portStr)
	log.Println(http.ListenAndServe(":"+portStr, nil))
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
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
	log.Println(param)

	go func() {
		run = runner.NewRunner(&param.Context, param.OpeRatios, param.TimeInMs, false, "", param.ID)
		errCh <- run.Run()
	}()
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Invalid method: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	select {
	case err := <-errCh:
		var writtenLength int
		data := []byte(successMessage)
		if err != nil {
			data = []byte(err.Error())
		}
		writtenLength, err = w.Write(data)
		if err != nil {
			log.Fatal(err)
		} else if writtenLength != len(data) {
			log.Fatalf("Invalied writtern length. writtenLength = %v", writtenLength)
		}
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
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

	log.Println("Received stop request.")
	go func() {
		time.Sleep(3)
		log.Println("Stop.")
		os.Exit(0)
	}()
}
