package multiprocess

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/peng225/oval/internal/runner"
)

type State int

var (
	run            *runner.Runner
	resultErr      error
	stop           context.CancelFunc
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

func init() {
	stop = func() {}
}

func StartServer(port int, cert string) {
	portStr := strconv.Itoa(port)
	caCertFileName = cert
	serverCtx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	http.HandleFunc("/start", startHandler)
	http.HandleFunc("/result", resultHandler)
	http.HandleFunc("/cancel", cancelHandler)

	server := &http.Server{
		Addr:    ":" + portStr,
		Handler: nil,
	}

	go func() {
		slog.Info("Start server.", "port", portStr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server stopped in a erroneous way. %v", err)
		}
	}()

	<-serverCtx.Done()
	err := server.Shutdown(serverCtx)
	if err != nil {
		log.Fatalf("server.Shutdown failed. %v", err)
	}
	slog.Info("HTTP server stopped successfully.")
	cancelWorkload()
	for state != stopped {
		time.Sleep(time.Millisecond * 100)
	}
	slog.Info("Bye!")
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received a start request.")
	defer func() {
		if r.Body != nil {
			r.Body.Close()
		}
	}()
	if r.Method != http.MethodPost {
		slog.Error("Invalid method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if state == running {
		// This block exists to make startHandler idempotent.
		slog.Info("Workload is already running.")
		return
	} else if state != stopped {
		// FIXME: When the status is canceling,
		//        returning "InternalServerError" is not good.
		slog.Error("Invalid state.", "state", state)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var param StartFollowerParameter
	err = json.Unmarshal(body, &param)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	printStartFollowerParameter(&param)

	state = running
	resultErr = nil
	watchDog = 0

	var ctx context.Context
	ctx, stop = context.WithCancel(context.Background())
	go func() {
		run = runner.NewRunner(&param.Context, param.OpeRatio, param.TimeInMs, false, "",
			param.ID, param.MultipartThresh, caCertFileName)
		err := run.InitBucket(ctx)
		if err != nil {
			resultErr = fmt.Errorf("run.InitBucket() failed. %w", err)
			slog.Error(resultErr.Error())
		} else {
			resultErr = run.Run(ctx)
		}
		mu.Lock()
		defer mu.Unlock()
		stop()
		stop = func() {}
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
					slog.Error("Could not receive requests from the leader for some time period.")
					cancelWorkload()
					return
				}
				previousWatchDog = watchDog
			}
		}
	}()
}

func printStartFollowerParameter(param *StartFollowerParameter) {
	slog.Info("Start follower params.", "ID", param.ID,
		"Context", param.Context,
		"OpeRatio", param.OpeRatio,
		"TimeInMs", param.TimeInMs,
		"MultipartThresh", param.MultipartThresh)
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		slog.Error("Invalid method", "method", r.Method)
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
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	} else if writtenLength != len(data) {
		slog.Error("Invalid written length.", "writtenLength", writtenLength)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func cancelHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received a cancel request.")
	defer func() {
		if r.Body != nil {
			r.Body.Close()
		}
	}()
	if r.Method != http.MethodPost {
		slog.Error("Invalid method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	_, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	cancelWorkload()
}

func cancelWorkload() {
	mu.Lock()
	defer mu.Unlock()
	if state != running {
		slog.Info("Workload is not running.", "state", state)
		return
	}

	stop()
	state = cancelling
	slog.Info("Requested to cancel the workload.")
}
