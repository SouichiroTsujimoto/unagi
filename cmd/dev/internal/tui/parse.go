package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const maxRecentRequests = 5

type rebuildState string

const (
	rebuildUnknown  rebuildState = "—"
	rebuildBuilding rebuildState = "building"
	rebuildOK       rebuildState = "ok"
	rebuildFail     rebuildState = "fail"
)

type rebuildInfo struct {
	State  rebuildState
	At     time.Time
	Detail string
}

type requestInfo struct {
	Method   string
	Path     string
	Status   int
	Duration string
	At       time.Time
}

var (
	ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// tint-style: INF request method=GET path=/ status=200 duration=1ms
	requestLogRE = regexp.MustCompile(`(?i)\brequest\b.*?\bmethod=(\S+)\s+path=(\S+)\s+status=(\d+)\s+duration=(\S+)`)
	// tint-style: INF listening (HTTP) url=... pid=12345
	listeningPIDRE = regexp.MustCompile(`(?i)\blistening\b.*?\bpid=(\d+)`)
)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func applyRebuildLine(info rebuildInfo, line string) rebuildInfo {
	plain := strings.ToLower(stripANSI(line))
	switch {
	case strings.Contains(plain, "building..."):
		return rebuildInfo{State: rebuildBuilding, At: time.Now()}
	case strings.Contains(plain, "running..."):
		return rebuildInfo{State: rebuildOK, At: time.Now()}
	case strings.Contains(plain, "failed to build"):
		detail := strings.TrimSpace(stripANSI(line))
		if i := strings.Index(strings.ToLower(detail), "failed to build"); i >= 0 {
			detail = strings.TrimSpace(detail[i:])
		}
		return rebuildInfo{State: rebuildFail, At: time.Now(), Detail: detail}
	case strings.Contains(plain, "failed to run"), strings.Contains(plain, "failed to start"):
		return rebuildInfo{State: rebuildFail, At: time.Now(), Detail: strings.TrimSpace(stripANSI(line))}
	default:
		return info
	}
}

// isAirCycleStart reports whether this line begins a new air rebuild cycle.
// With log.main_only=true only "has changed" appears; otherwise "building..." does too.
func isAirCycleStart(line string) bool {
	plain := strings.ToLower(stripANSI(line))
	return strings.Contains(plain, "building...") || strings.Contains(plain, " has changed")
}

// templIconRE matches templ sloghandler / CLI status icons.
// ✓/✗ are the real glyphs; v/x show up in some fonts/renderers as lookalikes.
var templIconRE = regexp.MustCompile(`^(\((?:✓|✔|✗|✘|!)\))(\s+)(.*)$`)

// colorizeToolLine restores templ-style icon colors when the child process
// stripped them (non-TTY pipe and/or NO_COLOR).
//
// Only the icon is recolored — matching templ's own handler. Air often prefixes
// the first child-output line with a bare reset (ESC[0m) after its own colored
// status line; that is not real coloring, so we strip ANSI before matching and
// leave non-icon lines (e.g. air's "building...") untouched.
func colorizeToolLine(line string) string {
	plain := strings.TrimRight(stripANSI(line), "\r")
	m := templIconRE.FindStringSubmatch(plain)
	if len(m) != 4 {
		return line
	}
	icon, sep, rest := m[1], m[2], m[3]
	// Force ANSI even under NO_COLOR / non-TTY detection: this text is painted
	// into the Bubble Tea alt screen, not written straight to a dumb pipe.
	restore := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(restore)

	style := lipgloss.NewStyle()
	switch icon {
	case "(✓)", "(✔)":
		style = style.Foreground(lipgloss.Color("10"))
	case "(!)":
		style = style.Foreground(lipgloss.Color("11"))
	case "(✗)", "(✘)":
		style = style.Foreground(lipgloss.Color("9"))
	}
	return style.Render(icon) + sep + rest
}

func parseListeningPID(line string) (string, bool) {
	m := listeningPIDRE.FindStringSubmatch(stripANSI(line))
	if len(m) != 2 {
		return "", false
	}
	return m[1], true
}

func parseRequestLine(line string) (requestInfo, bool) {
	m := requestLogRE.FindStringSubmatch(stripANSI(line))
	if len(m) != 5 {
		return requestInfo{}, false
	}
	status, err := strconv.Atoi(m[3])
	if err != nil {
		return requestInfo{}, false
	}
	return requestInfo{
		Method:   m[1],
		Path:     m[2],
		Status:   status,
		Duration: strings.TrimSuffix(m[4], ","),
		At:       time.Now(),
	}, true
}

func pushRequest(list []requestInfo, req requestInfo) []requestInfo {
	list = append([]requestInfo{req}, list...)
	if len(list) > maxRecentRequests {
		list = list[:maxRecentRequests]
	}
	return list
}

func formatAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 1500*time.Millisecond:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func wrapLogLines(lines []string, width int) string {
	if width < 1 {
		width = 1
	}
	style := lipgloss.NewStyle().Width(width)
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if line == "" {
			b.WriteString(style.Render(" "))
			continue
		}
		b.WriteString(style.Render(line))
	}
	return b.String()
}

func formatRequestRow(req requestInfo, width int) string {
	row := fmt.Sprintf("%s %s %d %s", req.Method, req.Path, req.Status, req.Duration)
	if width > 0 && lipgloss.Width(row) > width {
		// Prefer keeping method/status/duration; trim path.
		suffix := fmt.Sprintf(" %d %s", req.Status, req.Duration)
		budget := width - lipgloss.Width(req.Method) - 1 - lipgloss.Width(suffix)
		path := req.Path
		if budget < 4 {
			return lipgloss.NewStyle().Width(width).Render(row)
		}
		if lipgloss.Width(path) > budget {
			runes := []rune(path)
			keep := budget - 1
			if keep < 1 {
				keep = 1
			}
			if keep > len(runes) {
				keep = len(runes)
			}
			path = string(runes[:keep]) + "…"
		}
		row = fmt.Sprintf("%s %s%s", req.Method, path, suffix)
	}
	return row
}
