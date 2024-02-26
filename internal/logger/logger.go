package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	Plane = "plane"
	JSON  = "json"
)

var logFormat string

func init() {
	logFormat = Plane
}

func SetLogFormat(f string) error {
	var l *slog.Logger
	switch f {
	case Plane:
		l = slog.New(newPlaneHandler())
	case JSON:
		l = slog.New(slog.NewJSONHandler(os.Stdout,
			&slog.HandlerOptions{
				AddSource: true,
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					switch a.Key {
					case "function", "file":
						a.Value = slog.StringValue(filepath.Base(a.Value.String()))
					}
					return a
				},
			}))
	default:
		return fmt.Errorf("invalid log format: %s", f)
	}
	logFormat = f
	slog.SetDefault(l)
	return nil
}

func LogFormat() string {
	return logFormat
}
