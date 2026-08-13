package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/SouichiroTsujimoto/unagi/internal/app"
)

// Set at build time with:
// go build -ldflags "-X main.version=1.2.3" ./cmd/server
var version = "0.1.0"

func main() {
	cfg := app.LoadConfig(version)
	flag.StringVar(&cfg.Address, "addr", cfg.Address, "listen address (PORT env overrides)")
	flag.StringVar(&cfg.DB.DSN, "db", cfg.DB.DSN, "database DSN (Postgres URL)")
	flag.StringVar(&cfg.Banner, "banner", cfg.Banner, "startup banner style: full or compact")
	flag.Parse()

	cfg.Address = app.AddressFromPort(cfg.Address)
	if err := app.Run(cfg); err != nil {
		slog.Error("application stopped", "err", err)
		os.Exit(1)
	}
}
