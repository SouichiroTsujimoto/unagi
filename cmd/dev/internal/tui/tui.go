package tui

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SouichiroTsujimoto/unagi/cmd/dev/internal/reload"
	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const (
	maxDevLogLines    = 3000
	maxAirLogLines    = 16 // air panel: latest cycle only
	minSidePanelWidth = 28
	minLogContentRows = 2 // below this, split layout is too cramped for logs
)

// DevConfig configures the development launcher (TUI or plain Air).
type DevConfig struct {
	BannerStyle string // "full" (logo) or "compact" (no logo)
	TUI         bool
	Version     string
	Address     string
	DB          db.Config
}

type appLogLineMsg string
type airLogLineMsg string
type procDoneMsg struct{ err error }

type attachStreamsMsg struct {
	stopFn   func()
	appLogCh <-chan string
	airLogCh <-chan string
	doneCh   <-chan error
}

// devPane is the active content in cramped (pane-switch) layout.
// Bubble Tea owns the selection; lipgloss only styles the tab buttons.
type devPane int

const (
	paneBanner devPane = iota
	paneAir
	paneLogs
)

func (p devPane) label() string {
	switch p {
	case paneBanner:
		return "banner"
	case paneAir:
		return "air"
	case paneLogs:
		return "logs"
	default:
		return "logs"
	}
}

type devModel struct {
	cfg        DevConfig
	bannerInfo terminal.BannerInfo
	banner     string
	ready      bool
	activePane devPane

	appViewport viewport.Model
	airViewport viewport.Model
	appLines    []string
	airLines    []string

	width  int
	height int
	err    error

	stopOnce *sync.Once
	stopFn   func()
	appLogCh <-chan string
	airLogCh <-chan string
	doneCh   <-chan error
}

// RunDev starts the development launcher.
// With TUI, app logs arrive via a Unix socket and Air/css-watch stay on pipes.
// Without TUI (or when stdout is not a TTY), Air runs with normal streaming logs
// and the server process prints the listen banner itself.
func RunDev(cfg DevConfig) error {
	if strings.TrimSpace(cfg.Version) == "" {
		cfg.Version = "dev"
	}
	if strings.TrimSpace(cfg.Address) == "" {
		cfg.Address = ":8080"
	}
	cfg.DB = cfg.DB.WithDefaults()
	cfg.BannerStyle = terminal.NormalizeBanner(cfg.BannerStyle)

	if err := ensureDevPortsFree(cfg.Address); err != nil {
		return err
	}

	stopReload, err := startReloadHub()
	if err != nil {
		return err
	}
	if stopReload != nil {
		defer stopReload()
	}

	if !cfg.TUI || !term.IsTerminal(int(os.Stdout.Fd())) {
		return runDevPlain(cfg)
	}

	m := newDevModel(cfg)
	// No mouse reporting: terminals disable native drag-select while mouse
	// tracking is on, which blocks copy/paste from the TUI.
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if dm, ok := final.(devModel); ok {
		dm.stop()
		if dm.err != nil && err == nil {
			return dm.err
		}
	}
	return err
}

func newDevModel(cfg DevConfig) devModel {
	info := terminal.BannerInfo{
		Version: cfg.Version,
		DBPath:  cfg.DB.Label(),
		Mode:    "http",
		URLs:    []string{httpDisplayURL(cfg.Address)},
	}
	return devModel{
		cfg:        cfg,
		bannerInfo: info,
		banner:     terminal.RenderListenBanner(cfg.BannerStyle, info),
		activePane: paneLogs,
		stopOnce:   &sync.Once{},
	}
}

func (m devModel) Init() tea.Cmd {
	appLogCh, airLogCh, doneCh, stopFn, err := startDevProcesses(m.cfg)
	if err != nil {
		return func() tea.Msg { return procDoneMsg{err: err} }
	}
	return tea.Batch(
		func() tea.Msg {
			return attachStreamsMsg{
				stopFn:   stopFn,
				appLogCh: appLogCh,
				airLogCh: airLogCh,
				doneCh:   doneCh,
			}
		},
		waitAppLog(appLogCh),
		waitAirLog(airLogCh),
		waitDone(doneCh),
	)
}

func waitAppLog(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return appLogLineMsg(line)
	}
}

func waitAirLog(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return airLogLineMsg(line)
	}
}

func waitDone(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-ch
		if !ok {
			return procDoneMsg{}
		}
		return procDoneMsg{err: err}
	}
}

func (m devModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case attachStreamsMsg:
		m.stopFn = msg.stopFn
		m.appLogCh = msg.appLogCh
		m.airLogCh = msg.airLogCh
		m.doneCh = msg.doneCh
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncViewportSize()

	case appLogLineMsg:
		follow := !m.ready || m.appViewport.AtBottom()
		line := string(msg)
		m.appLines = append(m.appLines, line)
		if len(m.appLines) > maxDevLogLines {
			m.appLines = m.appLines[len(m.appLines)-maxDevLogLines:]
		}
		if pid, ok := parseListeningPID(line); ok && pid != m.bannerInfo.PID {
			m.bannerInfo.PID = pid
			m.banner = terminal.RenderListenBanner(m.cfg.BannerStyle, m.bannerInfo)
			if m.ready {
				m.syncViewportSize()
			}
		}
		if m.ready {
			m.refreshAppLogContent()
			if follow {
				m.appViewport.GotoBottom()
			}
		}
		if m.appLogCh != nil {
			cmds = append(cmds, waitAppLog(m.appLogCh))
		}

	case airLogLineMsg:
		follow := !m.ready || m.airViewport.AtBottom()
		line := colorizeToolLine(string(msg))
		// A new build cycle replaces previous air noise; keep only the latest.
		if isAirCycleStart(line) {
			m.airLines = nil
		}
		m.airLines = append(m.airLines, line)
		if len(m.airLines) > maxAirLogLines {
			m.airLines = m.airLines[len(m.airLines)-maxAirLogLines:]
		}
		if m.ready {
			m.refreshAirLogContent()
			if follow {
				m.airViewport.GotoBottom()
			}
		}
		if m.airLogCh != nil {
			cmds = append(cmds, waitAirLog(m.airLogCh))
		}

	case procDoneMsg:
		m.err = msg.err
		m.stop()
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.stop()
			return m, tea.Quit
		case "1":
			if m.usePaneSwitch() {
				m.activePane = paneBanner
				return m, nil
			}
		case "2":
			if m.usePaneSwitch() {
				m.activePane = paneAir
				return m, nil
			}
		case "3":
			if m.usePaneSwitch() {
				m.activePane = paneLogs
				return m, nil
			}
		case "tab":
			if m.usePaneSwitch() {
				m.activePane = (m.activePane + 1) % 3
				return m, nil
			}
		case "shift+tab":
			if m.usePaneSwitch() {
				m.activePane = (m.activePane + 2) % 3
				return m, nil
			}
		case "shift+up", "shift+k", "K":
			// Shift+↑/k scrolls air; plain ↑/jk stay on logs (or the active pane).
			if m.ready {
				m.airViewport.ScrollUp(1)
			}
			return m, tea.Batch(cmds...)
		case "shift+down", "shift+j", "J":
			if m.ready {
				m.airViewport.ScrollDown(1)
			}
			return m, tea.Batch(cmds...)
		case "g":
			if !m.usePaneSwitch() || m.activePane == paneLogs {
				m.appViewport.GotoTop()
			} else if m.activePane == paneAir {
				m.airViewport.GotoTop()
			}
		case "G":
			if !m.usePaneSwitch() || m.activePane == paneLogs {
				m.appViewport.GotoBottom()
			} else if m.activePane == paneAir {
				m.airViewport.GotoBottom()
			}
		}
	}

	if m.ready && (!m.usePaneSwitch() || m.activePane == paneLogs) {
		var cmd tea.Cmd
		m.appViewport, cmd = m.appViewport.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.ready && m.usePaneSwitch() && m.activePane == paneAir {
		// Arrow keys scroll air when it is the active pane.
		if _, ok := msg.(tea.KeyMsg); ok {
			var cmd tea.Cmd
			m.airViewport, cmd = m.airViewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// shouldUsePaneSwitch reports when the split layout cannot show air beside the
// banner, or when logs would get fewer than minLogContentRows content lines.
// State/selection is Bubble Tea; tab chrome is lipgloss.
func shouldUsePaneSwitch(termW, termH, bannerW, bannerH, footerH, minAirW int) bool {
	if termW < 1 || termH < 1 {
		return true
	}
	if termW-bannerW-1 < minAirW {
		return true
	}
	logInnerH := termH - bannerH - footerH - 3 // outer logs box: border + title
	return logInnerH < minLogContentRows
}

func (m devModel) usePaneSwitch() bool {
	if m.width < 1 || m.height < 1 {
		return false
	}
	footerH := 1 // help line; avoid depending on rendered mode text
	return shouldUsePaneSwitch(
		m.width,
		m.height,
		lipgloss.Width(m.banner),
		lipgloss.Height(m.banner),
		footerH,
		minSidePanelWidth,
	)
}

func (m *devModel) syncViewportSize() {
	appFollow := !m.ready || m.appViewport.AtBottom()
	airFollow := !m.ready || m.airViewport.AtBottom()

	var appW, appH, airW, airH int
	if m.usePaneSwitch() {
		appW, appH = m.paneContentInnerSize()
		airW, airH = appW, appH
	} else {
		appW, appH = m.logInnerSize()
		airW, airH = m.airInnerSize()
	}

	if !m.ready {
		m.appViewport = viewport.New(appW, appH)
		m.airViewport = viewport.New(airW, airH)
		m.ready = true
		m.refreshAppLogContent()
		m.refreshAirLogContent()
		m.appViewport.GotoBottom()
		m.airViewport.GotoBottom()
		return
	}

	m.appViewport.Width = appW
	m.appViewport.Height = appH
	m.airViewport.Width = airW
	m.airViewport.Height = airH
	m.refreshAppLogContent()
	m.refreshAirLogContent()
	if appFollow {
		m.appViewport.GotoBottom()
	}
	if airFollow {
		m.airViewport.GotoBottom()
	}
}

func (m *devModel) refreshAppLogContent() {
	if !m.ready {
		return
	}
	m.appViewport.SetContent(wrapLogLines(m.appLines, m.appViewport.Width))
}

func (m *devModel) refreshAirLogContent() {
	if !m.ready {
		return
	}
	m.airViewport.SetContent(wrapLogLines(m.airLines, m.airViewport.Width))
}

func (m devModel) sidePanelWidth() int {
	if m.usePaneSwitch() {
		return 0
	}
	rem := m.width - lipgloss.Width(m.banner) - 1
	if rem < minSidePanelWidth {
		return 0
	}
	return rem
}

func (m devModel) footerHeight() int {
	return lipgloss.Height(m.footerView())
}

func (m devModel) headerHeight() int {
	return lipgloss.Height(m.headerView())
}

func (m devModel) logOuterSize() (w, h int) {
	w = m.width
	h = m.height - m.headerHeight() - m.footerHeight()
	return w, h
}

func (m devModel) logInnerSize() (w, h int) {
	outerW, outerH := m.logOuterSize()
	w = outerW - 2
	h = outerH - 3
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func (m devModel) airInnerSize() (w, h int) {
	panelW := m.sidePanelWidth()
	if panelW <= 0 {
		return 1, 1
	}
	panelH := lipgloss.Height(m.banner)
	// border(2) + title(1) + padding vertical handled by content height
	w = panelW - 4 // border + padding
	h = panelH - 3 // border + title
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// paneContentInnerSize is the scrollable body inside the single pane-switch frame.
func (m devModel) paneContentInnerSize() (w, h int) {
	// border(2) + tab row(1) + footer(1)
	w = m.width - 2
	h = m.height - m.footerHeight() - 3
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func (m devModel) View() string {
	if !m.ready {
		return "starting dev tui…"
	}
	if m.usePaneSwitch() {
		return m.paneSwitchView() + "\n" + m.footerView()
	}
	return m.headerView() + "\n" + m.logBox() + "\n" + m.footerView()
}

func (m devModel) headerView() string {
	w := m.sidePanelWidth()
	if w <= 0 {
		return m.banner
	}
	bannerH := lipgloss.Height(m.banner)
	row := lipgloss.JoinHorizontal(lipgloss.Top, m.banner, " ", m.airBox(w, bannerH))
	return row
}

func (m devModel) logBox() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"})
	title := titleStyle.Render("logs")

	inner := lipgloss.JoinVertical(lipgloss.Left, title, m.appViewport.View())
	outerW, _ := m.logOuterSize()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#2DD4BF"}).
		Width(max(outerW-2, 1)).
		Render(inner)
}

func (m devModel) airBox(width, height int) string {
	if width < minSidePanelWidth {
		width = minSidePanelWidth
	}
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"})
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"})
	title := titleStyle.Render("air") + metaStyle.Render(fmt.Sprintf("  ·  %d", len(m.airLines)))

	body := m.airViewport.View()
	if !m.ready || len(m.airLines) == 0 {
		body = metaStyle.Render("waiting…")
	}
	inner := lipgloss.JoinVertical(lipgloss.Left, title, body)
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#2DD4BF"}).
		Width(max(width-2, 1)).
		Padding(0, 1)
	if height > 2 {
		style = style.Height(height - 2)
	}
	return style.Render(inner)
}

func (m devModel) paneSwitchView() string {
	tabs := m.tabsView()
	body := m.paneBodyView()
	inner := lipgloss.JoinVertical(lipgloss.Left, tabs, body)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#2DD4BF"}).
		Width(max(m.width-2, 1)).
		Render(inner)
}

func (m devModel) paneTabLabel(p devPane) string {
	if p == paneAir {
		return fmt.Sprintf("air · %d", len(m.airLines))
	}
	return p.label()
}

func (m devModel) paneTabCell(p devPane) string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"}).
		Background(lipgloss.AdaptiveColor{Light: "#CCFBF1", Dark: "#134E4A"}).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"}).
		Padding(0, 1)
	label := m.paneTabLabel(p)
	if p == m.activePane {
		return activeStyle.Render(label)
	}
	return inactiveStyle.Render(label)
}

func (m devModel) tabsView() string {
	panes := []devPane{paneBanner, paneAir, paneLogs}
	parts := make([]string, 0, len(panes))
	for _, p := range panes {
		parts = append(parts, m.paneTabCell(p))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#A8A29E", Dark: "#78716C"}).
		Render("  1/2/3 · tab/⇧tab")
	return lipgloss.JoinHorizontal(lipgloss.Center, row, hint)
}

func (m devModel) paneBodyView() string {
	w, h := m.paneContentInnerSize()
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"})

	switch m.activePane {
	case paneBanner:
		// Outer pane frame already has a border; omit the banner's own box.
		plain := terminal.RenderListenBannerPlain(m.cfg.BannerStyle, m.bannerInfo)
		return lipgloss.NewStyle().Width(w).MaxHeight(h).Render(plain)
	case paneAir:
		if !m.ready || len(m.airLines) == 0 {
			return metaStyle.Width(w).Height(h).Render("waiting…")
		}
		return m.airViewport.View()
	default:
		return m.appViewport.View()
	}
}

func (m devModel) footerView() string {
	help := "↑↓/jk logs · shift+↑↓/jk air · G/g top/bottom · q quit"
	if m.usePaneSwitch() {
		help = "1/2/3 · tab · ↑↓/jk scroll · shift+↑↓/jk air · G/g · q quit"
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"}).
		Padding(0, 1)
	return style.Width(max(m.width, 1)).Render(help)
}

func (m *devModel) stop() {
	if m.stopOnce == nil {
		return
	}
	m.stopOnce.Do(func() {
		if m.stopFn != nil {
			m.stopFn()
		}
	})
}

func cssWatchAvailable() (string, bool) {
	fi, err := os.Stat("tools/tailwindcss")
	if err != nil || fi.IsDir() {
		return "", false
	}
	return "./tools/tailwindcss", true
}

// devChildEnviron drops parent NO_COLOR/FORCE_COLOR=0 so child tools (air, templ)
// are not forced into monochrome. Air still runs with piped stdio, so templ's
// fatih/color may still strip ANSI; colorizeToolLine restores those lines.
func devChildEnviron() []string {
	out := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "NO_COLOR="),
			e == "FORCE_COLOR=0",
			e == "TERM=dumb":
			continue
		default:
			out = append(out, e)
		}
	}
	out = append(out, "TERM=xterm-256color")
	return out
}

func startDevProcesses(cfg DevConfig) (appLogCh <-chan string, airLogCh <-chan string, doneCh <-chan error, stopFn func(), err error) {
	appCh := make(chan string, 256)
	airCh := make(chan string, 256)
	done := make(chan error, 1)

	ln, err := terminal.ListenDevLogSock(terminal.DevLogSockPath)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var mu sync.Mutex
	var cmds []*exec.Cmd
	var procWG sync.WaitGroup
	var connWG sync.WaitGroup
	var acceptDone sync.WaitGroup

	acceptDone.Add(1)
	go func() {
		defer acceptDone.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			connWG.Add(1)
			go func(c net.Conn) {
				defer connWG.Done()
				streamReader(c, appCh)
			}(conn)
		}
	}()

	start := func(name string, args []string, out chan<- string, extraEnv ...string) error {
		cmd := exec.Command(name, args...)
		env := append(devChildEnviron(),
			"UNIGO_BANNER="+cfg.BannerStyle,
			"UNIGO_DB_DSN="+cfg.DB.DSN,
			"UNIGO_DEV_TUI=1",
			terminal.DevLogSockEnv+"="+terminal.DevLogSockPath,
		)
		env = append(env, extraEnv...)
		cmd.Env = env
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}

		mu.Lock()
		cmds = append(cmds, cmd)
		mu.Unlock()

		procWG.Add(2)
		go func() {
			defer procWG.Done()
			streamReader(stdout, out)
		}()
		go func() {
			defer procWG.Done()
			streamReader(stderr, out)
		}()
		return nil
	}

	if path, ok := cssWatchAvailable(); ok {
		// --watch=always: keep watching when stdin is closed (non-TTY pipes).
		_ = start(path, []string{
			"--silent",
			"-i", "./internal/web/css/styles.css",
			"-o", "./static/app.css",
			"--watch=always",
		}, airCh)
	}

	// Force ANSI colors: pipes are non-TTY, so air's default "auto" disables fatih/color.
	if err := start("go", []string{
		"tool", "air",
		"-log.main_only=false",
		"-color=always",
		"-color.mode=always",
	}, airCh); err != nil {
		_ = ln.Close()
		_ = os.Remove(terminal.DevLogSockPath)
		stopAll(&mu, cmds)
		return nil, nil, nil, nil, fmt.Errorf("start air: %w", err)
	}

	go func() {
		procWG.Wait()
		_ = ln.Close()
		acceptDone.Wait()
		connWG.Wait()
		mu.Lock()
		var firstErr error
		for _, cmd := range cmds {
			if err := cmd.Wait(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		mu.Unlock()
		close(airCh)
		close(appCh)
		done <- firstErr
		close(done)
		_ = os.Remove(terminal.DevLogSockPath)
	}()

	stopFn = func() {
		stopAll(&mu, cmds)
		_ = ln.Close()
		_ = os.Remove(terminal.DevLogSockPath)
	}
	return appCh, airCh, done, stopFn, nil
}

func streamReader(r io.Reader, out chan<- string) {
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		out <- sc.Text()
	}
	if c, ok := r.(net.Conn); ok {
		_ = c.Close()
	}
}

func stopAll(mu *sync.Mutex, cmds []*exec.Cmd) {
	mu.Lock()
	defer mu.Unlock()
	for _, cmd := range cmds {
		if cmd.Process == nil {
			continue
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	deadline := time.Now().Add(2 * time.Second)
	for _, cmd := range cmds {
		if cmd.Process == nil {
			continue
		}
		for time.Now().Before(deadline) {
			if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

func runDevPlain(cfg DevConfig) error {
	if path, ok := cssWatchAvailable(); ok {
		css := exec.Command(path,
			"--silent",
			"-i", "./internal/web/css/styles.css",
			"-o", "./static/app.css",
			"--watch=always",
		)
		css.Stdout = os.Stdout
		css.Stderr = os.Stderr
		if err := css.Start(); err == nil {
			defer func() {
				_ = css.Process.Kill()
				_ = css.Wait()
			}()
		}
	}
	cmd := exec.Command("go", "tool", "air")
	// Plain mode: server owns the listen banner (do not set UNIGO_DEV_TUI).
	cmd.Env = append(plainDevEnviron(),
		"UNIGO_BANNER="+cfg.BannerStyle,
		"UNIGO_DB_DSN="+cfg.DB.DSN,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func plainDevEnviron() []string {
	out := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "UNIGO_DEV_TUI=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ensureDevPortsFree fails fast when another just run (or stray tmp/main) still
// owns the app port. Silent sharing previously caused "address already in use"
// on every Air restart and disabled browser reload when :8199 was taken.
func ensureDevPortsFree(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("dev: listen address %s is already in use (another `just run` or tmp/main still running?): %w", addr, err)
	}
	_ = ln.Close()
	return nil
}

// startReloadHub starts the process-stable SSE hub used for browser auto-reload.
// Failure is fatal: without the hub, Air rebuilds never refresh open tabs.
func startReloadHub() (func(), error) {
	hub, err := reload.Start(reload.DefaultAddr)
	if err != nil {
		return nil, fmt.Errorf("dev: reload hub on %s (another `just run` still running?): %w", reload.DefaultAddr, err)
	}
	_ = os.Setenv(terminal.DevReloadNotifyEnv, hub.NotifyURL())
	_ = os.Setenv(terminal.DevReloadScriptEnv, hub.ScriptURL())
	return func() {
		_ = hub.Close()
		_ = os.Unsetenv(terminal.DevReloadNotifyEnv)
		_ = os.Unsetenv(terminal.DevReloadScriptEnv)
	}, nil
}

func httpDisplayURL(addr string) string {
	host := addr
	port := ""
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
		port = addr[i+1:]
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	if port == "" {
		return "http://" + host
	}
	return "http://" + host + ":" + port
}
