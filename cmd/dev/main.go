package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/SouichiroTsujimoto/unagi/cmd/dev/internal/tui"
	"github.com/SouichiroTsujimoto/unagi/internal/config"
	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
)

// Development launcher: optional Bubble Tea TUI, or plain Air with streaming logs.
// Precedence for logo/tui/db: CLI flag > UNIGO_* env > .env > .unigo.toml > defaults.
// Production bin/server does not load .env.
func main() {
	loadDotEnv()

	file := config.Load()
	logo := file.Logo()
	useTUI := file.TUIEnabled()
	dbCfg := db.Config{Driver: file.DB.Driver, DSN: file.DB.DSN}.WithDefaults()

	if v, ok := envBool("UNIGO_LOGO"); ok {
		logo = v
	}
	if v, ok := envBool("UNIGO_TUI"); ok {
		useTUI = v
	}
	if v := strings.TrimSpace(os.Getenv("UNIGO_DB_DRIVER")); v != "" {
		dbCfg.Driver = v
	}
	if v := strings.TrimSpace(os.Getenv("UNIGO_DB_DSN")); v != "" {
		dbCfg.DSN = v
	}

	logoFlag := flag.String("logo", "", "include ASCII logo in the startup banner (true|false)")
	tuiFlag := flag.String("tui", "", "use the Bubble Tea development TUI (true|false)")
	addr := flag.String("addr", ":8080", "listen address shown in the banner")
	dbDriverFlag := flag.String("db-driver", "", "database driver shown/passed to the server (sqlite|postgres)")
	dbDSNFlag := flag.String("db", "", "database DSN shown/passed to the server")
	flag.Parse()

	if v, ok := parseBoolFlag("logo", *logoFlag); ok {
		logo = v
	}
	if v, ok := parseBoolFlag("tui", *tuiFlag); ok {
		useTUI = v
	}
	if v := strings.TrimSpace(*dbDriverFlag); v != "" {
		dbCfg.Driver = v
	}
	if v := strings.TrimSpace(*dbDSNFlag); v != "" {
		dbCfg.DSN = v
	}
	dbCfg = dbCfg.WithDefaults()

	if err := terminal.EnsureBannerLogo(); err != nil {
		// Non-fatal: fall back to the embedded default logo.
		fmt.Fprintf(os.Stderr, "dev: banner logo: %v\n", err)
	}

	if err := tui.RunDev(tui.DevConfig{
		BannerStyle: terminal.BannerStyleFromLogo(logo),
		TUI:         useTUI,
		Version:     "0.1.0",
		Address:     *addr,
		DB:          dbCfg,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "dev: %v\n", err)
		os.Exit(1)
	}
}

func envBool(key string) (bool, bool) {
	return parseBoolFlag(key, os.Getenv(key))
}

func parseBoolFlag(name, raw string) (bool, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return false, false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		switch strings.ToLower(v) {
		case "on", "yes":
			return true, true
		case "off", "no":
			return false, true
		default:
			fmt.Fprintf(os.Stderr, "invalid %s %q (use true or false)\n", name, raw)
			os.Exit(2)
			return false, false
		}
	}
	return b, true
}
