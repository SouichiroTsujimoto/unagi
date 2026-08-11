package terminal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const listenBannerGate = "tmp/.listen_banner"

// BannerInfo is the listen-time context shown in the startup banner.
type BannerInfo struct {
	Version string
	DBPath  string
	Mode    string
	PID     string
	URLs    []string
}

// ShouldPrintListenBanner shows the banner once per Air session.
// When tmp/ exists (just run), a gate file suppresses repeats on hot reload.
// Production binaries without tmp/ always print the banner.
// Dev TUI owns the banner, so the server process skips printing.
func ShouldPrintListenBanner() bool {
	if os.Getenv("UNIGO_DEV_TUI") == "1" {
		return false
	}
	if _, err := os.Stat(listenBannerGate); err == nil {
		return false
	}
	info, err := os.Stat(filepath.Dir(listenBannerGate))
	if err != nil || !info.IsDir() {
		return true
	}
	_ = os.WriteFile(listenBannerGate, []byte{}, 0o644)
	return true
}

// PrintListenBanner writes the configured listen banner to w.
func PrintListenBanner(w io.Writer, style string, info BannerInfo) {
	if text := RenderListenBanner(style, info); text != "" {
		fmt.Fprintln(w, lipgloss.NewStyle().MarginTop(1).MarginBottom(1).Render(text))
	}
}

// RenderListenBanner returns the listen banner without a trailing newline.
func RenderListenBanner(style string, info BannerInfo) string {
	return renderListenBanner(style, info, true)
}

// RenderListenBannerPlain returns the banner body without an outer box border.
// Used when another frame (e.g. pane-switch TUI) already provides the chrome.
func RenderListenBannerPlain(style string, info BannerInfo) string {
	return renderListenBanner(style, info, false)
}

func renderListenBanner(style string, info BannerInfo, bordered bool) string {
	if len(info.URLs) == 0 {
		return ""
	}
	style = NormalizeBanner(style)
	fields := loadBannerFields()
	resolver := newBannerResolver(info)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"})
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"})
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#292524", Dark: "#E7E5E4"})
	urlStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#7DD3FC"})

	labelWidth := 0
	for _, field := range fields {
		if field == "" {
			continue
		}
		// Include the trailing ':' so lipgloss.Width does not wrap the label.
		if w := len(field) + 1; w > labelWidth {
			labelWidth = w
		}
	}
	if labelWidth == 0 {
		labelWidth = 8
	}

	kv := func(label, value string, valueRender lipgloss.Style) string {
		return labelStyle.Width(labelWidth).Render(label+":") + " " + valueRender.Render(value)
	}

	var lines []string
	for _, field := range fields {
		if field == "" {
			lines = append(lines, "")
			continue
		}
		if field == "url" {
			for i, url := range info.URLs {
				if i == 0 {
					lines = append(lines, kv("url", url, urlStyle))
				} else {
					lines = append(lines, strings.Repeat(" ", labelWidth+1)+urlStyle.Render(url))
				}
			}
			continue
		}
		lines = append(lines, kv(field, displayOrDash(resolver.value(field)), valueStyle))
	}

	var body strings.Builder
	body.WriteString(titleStyle.Render(resolver.title()))
	if len(lines) > 0 {
		body.WriteString("\n\n")
		body.WriteString(strings.Join(lines, "\n"))
	}

	content := body.String()
	if style == "full" {
		if logo := renderASCIILogo(); logo != "" {
			logoBlock := lipgloss.NewStyle().MarginRight(2).Render(logo)
			content = lipgloss.JoinHorizontal(lipgloss.Center, logoBlock, content)
		}
	}

	wrap := lipgloss.NewStyle().Padding(0, 1)
	if bordered {
		wrap = wrap.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#2DD4BF"})
	} else {
		// Replace the missing border with a little breathing room inside the outer frame.
		wrap = wrap.Padding(1, 2)
	}
	return wrap.Render(content)
}

// NormalizeBanner returns "full" or "compact".
func NormalizeBanner(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "compact":
		return "compact"
	default:
		return "full"
	}
}

// BannerStyleFromLogo maps the just-run logo switch onto the banner style.
// logo on → full (ASCII logo + box); logo off → compact (box only).
func BannerStyleFromLogo(logo bool) string {
	if logo {
		return "full"
	}
	return "compact"
}
