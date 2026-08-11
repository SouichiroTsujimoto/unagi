package terminal

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unigo-template/internal/config"
)

var (
	goModModuleRE   = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	goModRequireRE  = regexp.MustCompile(`(?m)^\s*([^\s]+)\s+v(\S+)`)
	tailwindVerRE   = regexp.MustCompile(`(?i)tailwindcss\s+v?(\d+\.\d+\.\d+\S*)`)
	airCLIVersionRE = regexp.MustCompile(`(?i)\bair\s+v(\d+\.\d+\.\d+\S*)`)
	daisyVersionRE  = regexp.MustCompile(`(?m)^var version = "([^"]+)"`)
)

var knownBannerFields = map[string]struct{}{
	"version":     {},
	"go":          {},
	"templ":       {},
	"tailwindcss": {},
	"daisyui":     {},
	"air":         {},
	"mode":        {},
	"db":          {},
	"css":         {},
	"git":         {},
	"url":         {},
	"pid":         {},
}

type bannerResolver struct {
	info  BannerInfo
	cache map[string]string
}

func loadBannerFields() []string {
	fields := filterBannerFields(config.Load().Banner.Fields)
	if len(fields) == 0 {
		return filterBannerFields(config.Default().Banner.Fields)
	}
	return fields
}

func filterBannerFields(fields []string) []string {
	var out []string
	for _, field := range fields {
		if field == "" {
			out = append(out, "")
			continue
		}
		key := strings.ToLower(strings.TrimSpace(field))
		if _, ok := knownBannerFields[key]; !ok {
			continue
		}
		out = append(out, key)
	}
	return trimBannerFields(out)
}

func trimBannerFields(fields []string) []string {
	start, end := 0, len(fields)
	for start < end && fields[start] == "" {
		start++
	}
	for end > start && fields[end-1] == "" {
		end--
	}
	return fields[start:end]
}

func newBannerResolver(info BannerInfo) *bannerResolver {
	if strings.TrimSpace(info.Version) == "" {
		info.Version = "dev"
	}
	if strings.TrimSpace(info.Mode) == "" {
		info.Mode = "http"
	}
	return &bannerResolver{info: info, cache: map[string]string{}}
}

func (r *bannerResolver) title() string {
	if title := moduleBaseName("go.mod"); title != "" {
		return title
	}
	return "unigo-template"
}

func (r *bannerResolver) value(field string) string {
	if field == "url" {
		return ""
	}
	if v, ok := r.cache[field]; ok {
		return v
	}
	v := r.resolve(field)
	r.cache[field] = v
	return v
}

func (r *bannerResolver) resolve(field string) string {
	switch field {
	case "version":
		return r.info.Version
	case "go":
		return strings.TrimPrefix(runtime.Version(), "go")
	case "templ":
		return goModRequireVersion("go.mod", "github.com/a-h/templ")
	case "tailwindcss":
		return detectTailwindCSSVersion()
	case "daisyui":
		return detectDaisyUIVersion()
	case "air":
		return detectAirVersion()
	case "mode":
		return r.info.Mode
	case "db":
		return displayOrDash(r.info.DBPath)
	case "css":
		return detectCSSStack()
	case "git":
		return detectGitSummary()
	case "pid":
		return strings.TrimSpace(r.info.PID)
	default:
		return ""
	}
}

func moduleBaseName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	m := goModModuleRE.FindSubmatch(data)
	if len(m) != 2 {
		return ""
	}
	module := string(m[1])
	if i := strings.LastIndex(module, "/"); i >= 0 && i+1 < len(module) {
		return module[i+1:]
	}
	return module
}

func goModRequireVersion(path, modulePath string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, m := range goModRequireRE.FindAllSubmatch(data, -1) {
		if len(m) == 3 && string(m[1]) == modulePath {
			return string(m[2])
		}
	}
	return ""
}

func detectAirVersion() string {
	if v := goModRequireVersion("go.mod", "github.com/air-verse/air"); v != "" {
		return v
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "tool", "air", "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	if m := airCLIVersionRE.FindSubmatch(out); len(m) == 2 {
		return string(m[1])
	}
	return ""
}

func detectTailwindCSSVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./tools/tailwindcss", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if m := tailwindVerRE.FindSubmatch(line); len(m) == 2 {
			return string(m[1])
		}
		break
	}
	return ""
}

func detectDaisyUIVersion() string {
	data, err := os.ReadFile(filepath.Join("tools", "daisyui.mjs"))
	if err != nil {
		return ""
	}
	if m := daisyVersionRE.FindSubmatch(data); len(m) == 2 {
		return string(m[1])
	}
	return ""
}

func detectCSSStack() string {
	info, err := os.Stat(filepath.Join("tools", "tailwindcss"))
	if err == nil && !info.IsDir() {
		return "tailwind"
	}
	return ""
}

func detectGitSummary() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	branchOut, err := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	shaOut, err := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(branchOut))
	sha := strings.TrimSpace(string(shaOut))
	if branch == "" || sha == "" {
		return ""
	}
	return branch + "@" + sha
}

func displayOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

