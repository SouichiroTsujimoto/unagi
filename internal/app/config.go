package app

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/config"
	"github.com/SouichiroTsujimoto/unagi/internal/db"
	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
)

const (
	defaultSupabaseURL = "http://127.0.0.1:54321"
	siteDescription    = "wuhu1slandの技術ノート"
)

type Config struct {
	Address           string
	DB                db.Config
	Version           string
	Banner            string
	Site              layout.Site
	Auth              featureauth.Config
	MediaPublicBase   string
	SupabaseSecretKey string
	ContentSyncSecret string
	DevAdminBypass    bool
}

func LoadConfig(version string) Config {
	file := config.Load()
	supabaseURL := strings.TrimRight(envOr("UNIGO_SUPABASE_URL", defaultSupabaseURL), "/")
	siteBaseURL := strings.TrimRight(envOr("UNIGO_SITE_BASE_URL", file.Site.BaseURL), "/")
	mediaPublicBase := strings.TrimRight(
		envOr("UNIGO_MEDIA_PUBLIC_BASE", supabaseURL+"/storage/v1/object/public/"+media.Bucket),
		"/",
	)
	cfg := Config{
		Address: ":8080",
		DB: db.Config{
			DSN: envOr("UNIGO_DB_DSN", file.DB.DSN),
		},
		Version: version,
		Banner: terminal.NormalizeBanner(
			envOr("UNIGO_BANNER", terminal.BannerStyleFromLogo(file.Logo())),
		),
		Site: layout.Site{
			Name:        file.Site.Name,
			Description: siteDescription,
			BaseURL:     siteBaseURL,
			MediaOrigin: layout.OriginOf(mediaPublicBase),
		},
		Auth: featureauth.Config{
			SupabaseURL:    supabaseURL,
			PublishableKey: strings.TrimSpace(os.Getenv("UNIGO_SUPABASE_PUBLISHABLE_KEY")),
			AdminUserIDs:   splitCSV(os.Getenv("UNIGO_ADMIN_USER_IDS")),
			AllowedOrigins: []string{siteBaseURL},
			SiteBaseURL:    siteBaseURL,
			SessionTTL:     7 * 24 * time.Hour,
			SecureCookies:  strings.HasPrefix(strings.ToLower(siteBaseURL), "https://"),
		},
		MediaPublicBase:   mediaPublicBase,
		SupabaseSecretKey: strings.TrimSpace(os.Getenv("UNIGO_SUPABASE_SECRET_KEY")),
		ContentSyncSecret: strings.TrimSpace(os.Getenv("UNIGO_CONTENT_SYNC_SECRET")),
		DevAdminBypass:    developmentAdminBypass(siteBaseURL),
	}
	return cfg.withDefaults()
}

func AddressFromPort(fallback string) string {
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}
	return fallback
}

func (cfg Config) withDefaults() Config {
	cfg.Address = strings.TrimSpace(cfg.Address)
	if cfg.Address == "" {
		cfg.Address = ":8080"
	}
	cfg.DB = cfg.DB.WithDefaults()
	cfg.Site.Name = strings.TrimSpace(cfg.Site.Name)
	if cfg.Site.Name == "" {
		cfg.Site.Name = "unagi"
	}
	cfg.Site.Description = strings.TrimSpace(cfg.Site.Description)
	if cfg.Site.Description == "" {
		cfg.Site.Description = siteDescription
	}
	cfg.Site.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Site.BaseURL), "/")
	cfg.MediaPublicBase = strings.TrimRight(strings.TrimSpace(cfg.MediaPublicBase), "/")
	if cfg.MediaPublicBase == "" && cfg.Auth.SupabaseURL != "" {
		cfg.MediaPublicBase = strings.TrimRight(cfg.Auth.SupabaseURL, "/") + "/storage/v1/object/public/" + media.Bucket
	}
	if cfg.Site.MediaOrigin == "" {
		cfg.Site.MediaOrigin = layout.OriginOf(cfg.MediaPublicBase)
	}
	if cfg.Auth.SiteBaseURL == "" {
		cfg.Auth.SiteBaseURL = cfg.Site.BaseURL
	}
	if len(cfg.Auth.AllowedOrigins) == 0 && cfg.Site.BaseURL != "" {
		cfg.Auth.AllowedOrigins = []string{cfg.Site.BaseURL}
	}
	if cfg.Auth.SessionTTL <= 0 {
		cfg.Auth.SessionTTL = 7 * 24 * time.Hour
	}
	return cfg
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(raw string) []string {
	var values []string
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func developmentAdminBypass(siteBaseURL string) bool {
	devMode, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("UNIGO_DEV_MODE")))
	if err != nil || !devMode {
		return false
	}
	parsed, err := url.Parse(siteBaseURL)
	if err != nil || parsed.Scheme != "http" {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
