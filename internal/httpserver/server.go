package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
)

type Config struct {
	Address          string
	Domains          []string
	ACMEEmail        string
	CertMagicStorage string
	DBPath           string
	Version          string
	Banner           string // "full" (ASCII logo + box) or "compact" (box only)
}

func Run(handler http.Handler, config Config, log *slog.Logger) error {
	if len(config.Domains) > 0 {
		urls := make([]string, 0, len(config.Domains))
		for _, domain := range config.Domains {
			urls = append(urls, "https://"+domain)
		}
		pid := strconv.Itoa(os.Getpid())
		if terminal.ShouldPrintListenBanner() {
			terminal.PrintListenBanner(os.Stdout, config.Banner, terminal.BannerInfo{
				Version: config.Version,
				DBPath:  config.DBPath,
				Mode:    "https",
				PID:     pid,
				URLs:    urls,
			})
		}
		log.Info("listening (HTTPS via CertMagic)", "url", urls, "pid", pid)
		terminal.NotifyDevReload()
		if err := runHTTPS(handler, config); err != nil {
			return fmt.Errorf("serve HTTPS: %w", err)
		}
		return nil
	}

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
		BaseContext: func(net.Listener) context.Context {
			terminal.NotifyDevReload()
			return context.Background()
		},
	}
	if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
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
