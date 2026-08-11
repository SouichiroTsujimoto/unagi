package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/SouichiroTsujimoto/unigo-template/internal/app"
	"github.com/SouichiroTsujimoto/unigo-template/internal/config"
	"github.com/SouichiroTsujimoto/unigo-template/internal/db"
	"github.com/SouichiroTsujimoto/unigo-template/internal/terminal"
)

// Set at build time with:
// go build -ldflags "-X main.version=1.2.3" ./cmd/server
var version = "0.1.0"

func main() {
	file := config.Load()
	logo := file.Logo()
	if v, ok := envBool("UNIGO_LOGO"); ok {
		logo = v
	}
	bannerDefault := terminal.BannerStyleFromLogo(logo)
	if v := strings.TrimSpace(os.Getenv("UNIGO_BANNER")); v != "" {
		bannerDefault = terminal.NormalizeBanner(v)
	}

	dbDriver := file.DB.Driver
	if v := strings.TrimSpace(os.Getenv("UNIGO_DB_DRIVER")); v != "" {
		dbDriver = v
	}
	dbDSN := file.DB.DSN
	if v := strings.TrimSpace(os.Getenv("UNIGO_DB_DSN")); v != "" {
		dbDSN = v
	}

	addr := flag.String("addr", ":8080", "listen address for development HTTP mode")
	dbDriverFlag := flag.String("db-driver", dbDriver, "database driver: sqlite or postgres")
	dbDSNFlag := flag.String("db", dbDSN, "database DSN (sqlite path or postgres URL)")
	domain := flag.String("domain", "", "comma-separated domains for CertMagic HTTPS (empty = HTTP only)")
	email := flag.String("email", "", "ACME contact email for CertMagic")
	banner := flag.String("banner", bannerDefault, "startup banner style: full or compact")
	flag.Parse()

	if err := app.Run(app.Config{
		Address: *addr,
		DB: db.Config{
			Driver: *dbDriverFlag,
			DSN:    *dbDSNFlag,
		},
		Domains:   splitCSV(*domain),
		ACMEEmail: *email,
		Version:   version,
		Banner:    *banner,
	}); err != nil {
		slog.Error("application stopped", "err", err)
		os.Exit(1)
	}
}

func envBool(key string) (bool, bool) {
	v := strings.TrimSpace(os.Getenv(key))
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
			return false, false
		}
	}
	return b, true
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	domains := make([]string, 0, len(parts))
	for _, part := range parts {
		if domain := strings.TrimSpace(part); domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains
}
