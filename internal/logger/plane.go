package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type planeHandler struct {
	mu sync.Mutex
}

func newPlaneHandler() slog.Handler {
	return &planeHandler{}
}

func (h *planeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}
func (h *planeHandler) Handle(ctx context.Context, r slog.Record) error {
	frames := runtime.CallersFrames([]uintptr{r.PC})
	frame, _ := frames.Next()
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := fmt.Fprintf(os.Stdout, "%s %s %s:%d %s",
		r.Time.Format("2006-01-02T15:04:05.999Z07:00"), r.Level, filepath.Base(frame.File), frame.Line, r.Message)
	if err != nil {
		return err
	}

	count := 0
	r.Attrs(func(a slog.Attr) bool {
		if count == 0 {
			_, err = fmt.Fprintf(os.Stdout, " (%v:%v", a.Key, a.Value)
		} else {
			_, err = fmt.Fprintf(os.Stdout, ", %v:%v", a.Key, a.Value)
		}
		if err != nil {
			fmt.Println(err)
			return false
		}
		count += 1
		if count == r.NumAttrs() {
			_, err = fmt.Fprint(os.Stdout, ")")
			if err != nil {
				fmt.Println(err)
				return false
			}
		}
		return true
	})
	fmt.Fprintf(os.Stdout, "\n")
	return nil
}

func (h *planeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *planeHandler) WithGroup(name string) slog.Handler {
	return h
}
