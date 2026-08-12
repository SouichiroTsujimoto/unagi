package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
)

type Config struct {
	Address string
	DBPath  string
	Version string
	Banner  string // "full" (ASCII logo + box) or "compact" (box only)
}

func Run(handler http.Handler, config Config, log *slog.Logger) error {
	ln, err := net.Listen("tcp", config.Address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	url := httpDisplayURL(ln.Addr().String())
	pid := strconv.Itoa(os.Getpid())
	if terminal.ShouldPrintListenBanner() {
		terminal.PrintListenBanner(os.Stdout, config.Banner, terminal.BannerInfo{
			Version: config.Version,
			DBPath:  config.DBPath,
			Mode:    "http",
			PID:     pid,
			URLs:    []string{url},
		})
	}
	log.Info("listening (HTTP)", "url", url, "pid", pid)

	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(ln)
	}()
	// Notify after Serve has started so the Accept loop is ready for the
	// browser reload that follows. Air sends SIGINT on rebuild; Shutdown
	// releases :8080 before the next binary binds.
	terminal.NotifyDevReload()

	select {
	case <-ctx.Done():
		// Air kills via process-group SIGINT (sh -c ./tmp/main). Free :8080
		// immediately with Close — Shutdown's connection drain races the next bind.
		_ = httpServer.Close()
		<-errCh
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
}

func httpDisplayURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}
