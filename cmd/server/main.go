package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/app"
	"github.com/SouichiroTsujimoto/unagi/internal/config"
	"github.com/SouichiroTsujimoto/unagi/internal/db"
	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
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

	addrDefault := ":8080"
	if v := strings.TrimSpace(os.Getenv("PORT")); v != "" {
		if strings.HasPrefix(v, ":") {
			addrDefault = v
		} else {
			addrDefault = ":" + v
		}
	}

	addr := flag.String("addr", addrDefault, "listen address (PORT env overrides default)")
	dbDriverFlag := flag.String("db-driver", dbDriver, "database driver (postgres)")
	dbDSNFlag := flag.String("db", dbDSN, "database DSN (postgres URL)")
	banner := flag.String("banner", bannerDefault, "startup banner style: full or compact")
	flag.Parse()

	listenAddr := *addr
	if v := strings.TrimSpace(os.Getenv("PORT")); v != "" {
		if strings.HasPrefix(v, ":") {
			listenAddr = v
		} else {
			listenAddr = ":" + v
		}
	}

	site := layoutSite(file)
	authCfg := authConfig(site)
	mediaBackend := envOr("UNIGO_MEDIA_BACKEND", "supabase")
	mediaDir := envOr("UNIGO_MEDIA_DIR", "data/media")
	if mediaBackend == "local" {
		_ = app.EnsureDataDirs("data", mediaDir)
	}

	if err := app.Run(app.Config{
		Address: listenAddr,
		DB: db.Config{
			Driver: *dbDriverFlag,
			DSN:    *dbDSNFlag,
		},
		Version:            version,
		Banner:             *banner,
		Site:               site,
		Auth:               authCfg,
		MediaBackend:       mediaBackend,
		MediaLocalDir:      mediaDir,
		MediaPublicBase:    os.Getenv("UNIGO_MEDIA_PUBLIC_BASE"),
		MediaBucket:        envOr("UNIGO_MEDIA_BUCKET", "images"),
		SupabaseURL:        envOr("UNIGO_SUPABASE_URL", authCfg.SupabaseURL),
		SupabaseServiceKey: os.Getenv("UNIGO_SUPABASE_SERVICE_ROLE_KEY"),
	}); err != nil {
		slog.Error("application stopped", "err", err)
		os.Exit(1)
	}
}

func authConfig(site layout.Site) featureauth.Config {
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
	origins := splitCSV(os.Getenv("UNIGO_ALLOWED_ORIGINS"))
	if len(origins) == 0 && site.BaseURL != "" {
		origins = []string{strings.TrimRight(site.BaseURL, "/")}
	}
	return featureauth.Config{
		SupabaseURL:    os.Getenv("UNIGO_SUPABASE_URL"),
		AnonKey:        os.Getenv("UNIGO_SUPABASE_ANON_KEY"),
		JWTSecret:      os.Getenv("UNIGO_SUPABASE_JWT_SECRET"),
		AdminUserIDs:   splitCSV(os.Getenv("UNIGO_ADMIN_USER_IDS")),
		AllowedOrigins: origins,
		SiteBaseURL:    site.BaseURL,
		SessionTTL:     ttl,
		SecureCookies:  secure,
	}
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
	site := layout.Site{
		Name:        file.Site.Name,
		Description: file.Site.Description,
		BaseURL:     file.Site.BaseURL,
		Author:      file.Site.Author,
	}
	if v := strings.TrimSpace(os.Getenv("UNIGO_SITE_NAME")); v != "" {
		site.Name = v
	}
	if v := strings.TrimSpace(os.Getenv("UNIGO_SITE_DESCRIPTION")); v != "" {
		site.Description = v
	}
	if v := strings.TrimSpace(os.Getenv("UNIGO_SITE_BASE_URL")); v != "" {
		site.BaseURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("UNIGO_SITE_AUTHOR")); v != "" {
		site.Author = v
	}
	return site
}
