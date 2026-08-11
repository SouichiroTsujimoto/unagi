package terminal

import (
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// NewLogger returns a tint-colored slog logger.
// When UNIGO_DEV_LOG_SOCK is set (dev TUI), logs go to that Unix socket
// instead of stdout so Air's stream stays separate from app logs.
func NewLogger() *slog.Logger {
	w := io.Writer(os.Stdout)
	if path := devLogSockFromEnv(); path != "" {
		if conn, err := dialDevLogSock(path); err == nil {
			w = conn
		}
	}
	return slog.New(tint.NewHandler(w, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.Kitchen,
		NoColor:    false,
	}))
}
