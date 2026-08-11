package tui

import (
	"strings"
	"testing"
)

func TestApplyRebuildLine(t *testing.T) {
	t.Parallel()

	var info rebuildInfo
	info = applyRebuildLine(info, "building...")
	if info.State != rebuildBuilding {
		t.Fatalf("state=%q want building", info.State)
	}
	info = applyRebuildLine(info, "running...")
	if info.State != rebuildOK {
		t.Fatalf("state=%q want ok", info.State)
	}
	info = applyRebuildLine(info, "failed to build, error: exit status 1")
	if info.State != rebuildFail {
		t.Fatalf("state=%q want fail", info.State)
	}
}

func TestColorizeToolLine(t *testing.T) {
	t.Parallel()

	got := colorizeToolLine("(✓) Complete [ updates=0 duration=1ms ]")
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ansi color, got %q", got)
	}
	if stripANSI(got) != "(✓) Complete [ updates=0 duration=1ms ]" {
		t.Fatalf("unexpected content: %q", got)
	}

	errLine := `(✗) Error [ error=failed to generate code for "/tmp/a.templ": parsing error: <>: close tag not found: line 15, col 3 ]`
	gotErr := colorizeToolLine(errLine)
	if !strings.Contains(gotErr, "\x1b[") {
		t.Fatalf("expected ansi on error icon, got %q", gotErr)
	}
	if stripANSI(gotErr) != errLine {
		t.Fatalf("error line should keep full text, got %q", gotErr)
	}

	cmdFail := "(✗) Command failed: generation completed with 1 errors"
	if stripANSI(colorizeToolLine(cmdFail)) != cmdFail {
		t.Fatal("command failed line mismatch")
	}

	// Air prefixes the first child line after a colored status with a bare reset.
	airPrefixed := "\x1b[0m(✗) Error [ error=boom ]"
	gotPrefixed := colorizeToolLine(airPrefixed)
	if !strings.Contains(gotPrefixed, "\x1b[") {
		t.Fatalf("expected recolor after air reset prefix, got %q", gotPrefixed)
	}
	if stripANSI(gotPrefixed) != "(✗) Error [ error=boom ]" {
		t.Fatalf("unexpected recolor content: %q", gotPrefixed)
	}

	// Non-icon air lines must keep their original ANSI.
	building := "\x1b[0m\x1b[33mbuilding...\x1b[0m"
	if colorizeToolLine(building) != building {
		t.Fatal("air status line should pass through unchanged")
	}
}

func TestIsAirCycleStart(t *testing.T) {
	t.Parallel()
	if !isAirCycleStart("building...") {
		t.Fatal("expected building start")
	}
	if !isAirCycleStart("\x1b[33mbuilding...\x1b[0m") {
		t.Fatal("expected ansi building start")
	}
	if !isAirCycleStart("internal/web/home/accounts.templ has changed") {
		t.Fatal("expected has-changed start")
	}
	if isAirCycleStart("running...") {
		t.Fatal("running should not reset")
	}
}

func TestParseListeningPID(t *testing.T) {
	t.Parallel()

	pid, ok := parseListeningPID("2:06AM INF listening (HTTP) url=http://localhost:8080 pid=4321")
	if !ok || pid != "4321" {
		t.Fatalf("parseListeningPID = %q ok=%v", pid, ok)
	}
	colored := "\x1b[92mINF\x1b[0m listening (HTTP) url=http://localhost:8080 pid=99"
	pid, ok = parseListeningPID(colored)
	if !ok || pid != "99" {
		t.Fatalf("ansi pid parse failed: %q ok=%v", pid, ok)
	}
}

func TestParseRequestLine(t *testing.T) {
	t.Parallel()

	line := "2:06AM INF request method=GET path=/accounts status=200 duration=3ms"
	req, ok := parseRequestLine(line)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if req.Method != "GET" || req.Path != "/accounts" || req.Status != 200 || req.Duration != "3ms" {
		t.Fatalf("unexpected request: %+v", req)
	}

	colored := "\x1b[92mINF\x1b[0m request method=POST path=/accounts status=204 duration=1ms"
	req, ok = parseRequestLine(colored)
	if !ok || req.Method != "POST" || req.Status != 204 {
		t.Fatalf("ansi parse failed: ok=%v req=%+v", ok, req)
	}
}

func TestWrapLogLines(t *testing.T) {
	t.Parallel()

	got := wrapLogLines([]string{"abcdefghijklmnopqrstuvwxyz"}, 10)
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected wrapped lines, got %q", got)
	}
}
