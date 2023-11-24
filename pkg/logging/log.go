package logging

import (
	"github.com/charmbracelet/log"
	"log/slog"
	"os"
)

var LogHandler = log.New(os.Stderr)
var Log = slog.New(LogHandler)

func Fatal(msg string, err error, args ...interface{}) {
	Log.Error(msg, "error", err, args)
	os.Exit(1)
}
