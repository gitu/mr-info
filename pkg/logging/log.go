package logging

import (
	"github.com/charmbracelet/log"
	"log/slog"
	"os"
)

var LogHandler = log.New(os.Stderr)
var Log = slog.New(LogHandler)
