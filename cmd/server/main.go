package main

import (
	"encoding/hex"
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/app"
	"github.com/SouichiroTsujimoto/unagi/internal/config"
	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/adminauth"
	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
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

	site := layoutSite(file)
	authCfg := adminAuthConfig(site)
	mediaBackend := envOr("UNIGO_MEDIA_BACKEND", "local")
	mediaDir := envOr("UNIGO_MEDIA_DIR", "data/media")
	certStorage := envOr("UNIGO_CERTMAGIC_STORAGE", "data/certmagic")
	_ = app.EnsureDataDirs("data", mediaDir, certStorage)

	if err := app.Run(app.Config{
		Address: *addr,
		DB: db.Config{
			Driver: *dbDriverFlag,
			DSN:    *dbDSNFlag,
		},
		Domains:          splitCSV(*domain),
		ACMEEmail:        *email,
		CertMagicStorage: certStorage,
		Version:          version,
		Banner:           *banner,
		Site:             site,
		Auth:             authCfg,
		MediaBackend:     mediaBackend,
		MediaLocalDir:    mediaDir,
		GCSBucket:        os.Getenv("UNIGO_GCS_BUCKET"),
		GCSPrefix:        os.Getenv("UNIGO_GCS_PREFIX"),
	}); err != nil {
		slog.Error("application stopped", "err", err)
		os.Exit(1)
	}
}

func adminAuthConfig(site layout.Site) adminauth.Config {
	origins := splitCSV(os.Getenv("UNIGO_WEBAUTHN_ORIGINS"))
	if len(origins) == 0 && site.BaseURL != "" {
		origins = []string{strings.TrimRight(site.BaseURL, "/")}
	}
	rpid := strings.TrimSpace(os.Getenv("UNIGO_WEBAUTHN_RPID"))
	if rpid == "" {
		rpid = hostFromBaseURL(site.BaseURL)
	}
	secure := strings.HasPrefix(site.BaseURL, "https://")
	if v, ok := envBool("UNIGO_SECURE_COOKIES"); ok {
		secure = v
	}
	ttl := 7 * 24 * time.Hour
	if v := strings.TrimSpace(os.Getenv("UNIGO_SESSION_TTL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ttl = d
		}
	}
	return adminauth.Config{
		RPDisplayName:      envOr("UNIGO_WEBAUTHN_RP_NAME", site.Name),
		RPID:               rpid,
		RPOrigins:          origins,
		BootstrapTokenHash: decodeBootstrapHash(os.Getenv("UNIGO_BOOTSTRAP_TOKEN_HASH")),
		SessionTTL:         ttl,
		SecureCookies:      secure,
	}
}

func decodeBootstrapHash(raw string) []byte {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
		return b
	}
	// Accept raw token for local convenience; hash it.
	return adminauth.HashToken(raw)
}

func hostFromBaseURL(base string) string {
	base = strings.TrimSpace(base)
	base = strings.TrimPrefix(base, "https://")
	base = strings.TrimPrefix(base, "http://")
	if i := strings.IndexByte(base, '/'); i >= 0 {
		base = base[:i]
	}
	if i := strings.IndexByte(base, ':'); i >= 0 {
		// keep host for localhost:8080 — WebAuthn RPID should be hostname without port
		base = base[:i]
	}
	return base
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
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
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func layoutSite(file config.File) layout.Site {
	return layout.Site{
		Name:        file.Site.Name,
		Description: file.Site.Description,
		BaseURL:     file.Site.BaseURL,
		Author:      file.Site.Author,
	}
}
