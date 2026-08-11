package terminal

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Dev log socket contract between the development TUI (listener) and the
// app logger (dialer). Kept in terminal so the runtime logger can honor it
// without importing cmd/dev.
const (
	DevLogSockEnv  = "UNIGO_DEV_LOG_SOCK"
	DevLogSockPath = "tmp/unigo-dev.log.sock"
)

func devLogSockFromEnv() string {
	if v := os.Getenv(DevLogSockEnv); v != "" {
		return v
	}
	return ""
}

// ListenDevLogSock creates the Unix socket the app logger dials in TUI mode.
func ListenDevLogSock(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", path, err)
	}
	return ln, nil
}

func dialDevLogSock(path string) (net.Conn, error) {
	var last error
	for range 20 {
		conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
		if err == nil {
			return conn, nil
		}
		last = err
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("dial %s: %w", path, last)
}
