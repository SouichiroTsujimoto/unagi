package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/SouichiroTsujimoto/unigo-template/internal/db"
)

func TestShouldUsePaneSwitch(t *testing.T) {
	t.Parallel()

	const (
		bannerW = 40
		bannerH = 10
		footerH = 1
		minAirW = minSidePanelWidth
	)

	tests := []struct {
		name       string
		termW      int
		termH      int
		wantSwitch bool
	}{
		{
			name:       "spacious split fits air and logs",
			termW:      bannerW + 1 + minAirW + 10,
			termH:      bannerH + footerH + 3 + minLogContentRows + 5,
			wantSwitch: false,
		},
		{
			name:       "narrow width hides air",
			termW:      bannerW + minAirW, // rem = minAirW-1
			termH:      80,
			wantSwitch: true,
		},
		{
			name:       "short height leaves only one log row",
			termW:      bannerW + 1 + minAirW + 10,
			termH:      bannerH + footerH + 3 + 1, // logInnerH == 1
			wantSwitch: true,
		},
		{
			name:       "exactly two log rows keeps split",
			termW:      bannerW + 1 + minAirW + 10,
			termH:      bannerH + footerH + 3 + minLogContentRows,
			wantSwitch: false,
		},
		{
			name:       "zero size is treated as cramped",
			termW:      0,
			termH:      0,
			wantSwitch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldUsePaneSwitch(tt.termW, tt.termH, bannerW, bannerH, footerH, minAirW)
			if got != tt.wantSwitch {
				t.Fatalf("shouldUsePaneSwitch(...) = %v, want %v", got, tt.wantSwitch)
			}
		})
	}
}

func TestPaneSwitchViewRendersTabs(t *testing.T) {
	t.Parallel()

	m := newDevModel(DevConfig{
		BannerStyle: "compact",
		Version:     "dev",
		Address:     ":8080",
		DB:          db.Config{Driver: db.DriverSQLite, DSN: "app.db"},
	})
	// Narrower than banner + air side panel.
	m.width = lipgloss.Width(m.banner) + minSidePanelWidth
	m.height = 40
	m.ready = true
	m.appViewport = viewport.New(20, 10)
	m.airViewport = viewport.New(20, 10)
	m.appViewport.SetContent("hello-log")
	m.airLines = []string{"building…"}
	m.refreshAirLogContent()

	if !m.usePaneSwitch() {
		t.Fatal("expected pane-switch layout for narrow width")
	}
	view := m.View()
	for _, want := range []string{"banner", "air", "logs", "hello-log"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}
