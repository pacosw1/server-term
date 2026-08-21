package ui

import (
	stdbuf "bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/franciscosainzwilliams/server-term/internal/agentclient"
	"github.com/franciscosainzwilliams/server-term/internal/collector"
	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/desktopclient"
	"github.com/franciscosainzwilliams/server-term/internal/devtools"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
)

var (
	cyan       = lipgloss.Color("#67E8F9")
	green      = lipgloss.Color("#4ADE80")
	yellow     = lipgloss.Color("#FACC15")
	red        = lipgloss.Color("#FB7185")
	muted      = lipgloss.Color("#64748B")
	panel      = lipgloss.Color("#1E293B")
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	dimStyle   = lipgloss.NewStyle().Foreground(muted)
	okStyle    = lipgloss.NewStyle().Foreground(green)
	warnStyle  = lipgloss.NewStyle().Foreground(yellow)
	errStyle   = lipgloss.NewStyle().Foreground(red)
)

type resultMsg struct {
	Index  int
	Sample metrics.Sample
}
type tickMsg time.Time
type streamMsg struct {
	Index   int
	Sample  metrics.Sample
	History []metrics.Sample
	Stream  *agentclient.Stream
	Err     error
}
type reconnectMsg int
type frameMsg time.Time
type historyMsg struct {
	Index   int
	Samples []metrics.Sample
	Err     error
}
type desktopShotMsg struct {
	Index int
	Frame string
	Err   error
}
type desktopStreamMsg struct {
	Index  int
	Stream *desktopclient.Stream
	Frame  []byte
	Err    error
}
type desktopStreamFrameMsg struct {
	Index int
	Frame []byte
	Err   error
}
type sshStartMsg struct {
	Pane *sshPane
	Err  error
}
type devtoolStatusMsg struct {
	Status   map[string]bool
	Versions map[string]string
	Err      error
}
type devtoolActionMsg struct {
	Tool, Action, Output string
	Err                  error
}
type Model struct {
	cfg             config.Config
	collector       collector.Collector
	samples         []metrics.Sample
	history         [][]metrics.Sample
	cursor          int
	detail          bool
	detailTab       int
	detailScroll    int
	rangeIndex      int
	displayCPU      []float64
	displayCores    [][]float64
	streamBuffers   [][]metrics.Sample
	desktopFrames   map[int]string
	desktopErrors   map[int]string
	desktopStreams  map[int]*desktopclient.Stream
	ssh             *sshPane
	sshText         string
	devtoolCursor   int
	devtoolStatus   map[string]bool
	devtoolVersions map[string]string
	devtoolConfirm  string
	devtoolMessage  string
	devtoolBusy     bool
	width, height   int
	collecting      bool
	pending         int
	lastRefresh     time.Time
}

func New(cfg config.Config) Model {
	n := 0
	for _, server := range cfg.Servers {
		if server.AgentURL == "" {
			n++
		}
	}
	return Model{cfg: cfg, collector: collector.Collector{SSH: cfg.SSH}, samples: make([]metrics.Sample, len(cfg.Servers)), history: make([][]metrics.Sample, len(cfg.Servers)), displayCPU: make([]float64, len(cfg.Servers)), displayCores: make([][]float64, len(cfg.Servers)), streamBuffers: make([][]metrics.Sample, len(cfg.Servers)), desktopFrames: map[int]string{}, desktopErrors: map[int]string{}, desktopStreams: map[int]*desktopclient.Stream{}, devtoolStatus: map[string]bool{}, devtoolVersions: map[string]string{}, collecting: n > 0, pending: n}
}
func (m Model) Init() tea.Cmd { return tea.Batch(append(m.collectAll(), m.nextFrame())...) }
func (m Model) nextFrame() tea.Cmd {
	return tea.Tick(time.Second/10, func(t time.Time) tea.Msg { return frameMsg(t) })
}
func (m Model) collect(i int) tea.Cmd {
	return func() tea.Msg {
		return resultMsg{Index: i, Sample: m.collector.Collect(context.Background(), m.cfg.Servers[i])}
	}
}
func (m Model) collectAll() []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.cfg.Servers))
	for i, server := range m.cfg.Servers {
		if server.AgentURL != "" {
			cmds = append(cmds, m.connectStream(i))
		} else {
			cmds = append(cmds, m.collect(i))
		}
	}
	return cmds
}
func (m Model) connectStream(i int) tea.Cmd {
	return func() tea.Msg {
		server := m.cfg.Servers[i]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		token := os.Getenv(server.TokenEnv)
		if server.TokenFile != "" {
			b, err := os.ReadFile(config.ExpandHome(server.TokenFile))
			if err != nil {
				return streamMsg{Index: i, Err: fmt.Errorf("read token: %w", err)}
			}
			token = strings.TrimSpace(string(b))
		}
		history, _ := agentclient.History(ctx, server.AgentURL, token, time.Hour, m.cfg.HistorySize)
		stream, err := agentclient.Connect(ctx, server.AgentURL, token)
		if err != nil {
			return streamMsg{Index: i, Err: err}
		}
		w, err := stream.Read(context.Background())
		return streamMsg{Index: i, Sample: w.Sample, History: history, Stream: stream, Err: err}
	}
}
func (m Model) readStream(i int, stream *agentclient.Stream) tea.Cmd {
	return func() tea.Msg {
		w, err := stream.Read(context.Background())
		return streamMsg{Index: i, Sample: w.Sample, Stream: stream, Err: err}
	}
}
func (m Model) reconnect(i int) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return reconnectMsg(i) })
}
func (m Model) nextTick() tea.Cmd {
	return tea.Tick(m.cfg.RefreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.ssh != nil {
			m.ssh.resize(msg.Width, max(1, msg.Height-4))
		}
	case tea.KeyMsg:
		if m.ssh != nil {
			switch msg.String() {
			case "x":
				m.ssh.close()
				m.ssh = nil
				m.sshText = ""
				return m, nil
			case "tab":
				maxTabs := 9
				if m.desktopForServer(m.cursor) != nil {
					maxTabs = 10
				}
				m.detailTab = (m.detailTab + 1) % maxTabs
				m.detailScroll = 0
				if m.detailTab == 9 {
					return m, m.loadDevtoolsStatus(m.cursor)
				}
				return m, nil
			case "1", "2", "3", "4", "5", "6", "7", "8", "9":
				n := int(msg.Runes[0] - '1')
				if n < 9 || (n == 9) {
					if n == 7 && m.desktopForServer(m.cursor) == nil {
						return m, nil
					}
					m.detailTab = n
					m.detailScroll = 0
					return m, nil
				}
			case "esc":
				m.ssh.close()
				m.ssh = nil
				m.sshText = ""
				return m, nil
			}
			_ = m.ssh.write(keyBytes(msg))
			return m, nil
		}
		if m.detail && m.detailTab == 9 {
			switch msg.String() {
			case "up", "k":
				if m.devtoolCursor > 0 {
					m.devtoolCursor--
				}
				return m, nil
			case "down", "j":
				if m.devtoolCursor < len(devtools.Catalog)-1 {
					m.devtoolCursor++
				}
				return m, nil
			case "esc":
				m.devtoolConfirm = ""
				return m, nil
			case "enter", "i":
				if m.devtoolConfirm == "install" {
					m.devtoolConfirm = ""
					m.devtoolBusy = true
					return m, m.runDevtoolAction(m.cursor, m.devtoolCursor, false)
				}
				m.devtoolConfirm = "install"
				return m, nil
			case "u":
				if m.devtoolConfirm == "uninstall" {
					m.devtoolConfirm = ""
					m.devtoolBusy = true
					return m, m.runDevtoolAction(m.cursor, m.devtoolCursor, true)
				}
				m.devtoolConfirm = "uninstall"
				return m, nil
			}
		}
		if m.detail && m.detailTab == 7 && isRemoteDesktopKey(msg.String()) {
			return m, m.sendDesktopKey(m.cursor, msg.String())
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.detail {
				m.detailScroll = max(0, m.detailScroll-1)
			} else if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.detail {
				m.detailScroll++
			} else if m.cursor < len(m.cfg.Servers)-1 {
				m.cursor++
			}
		case "enter", "right", "l":
			if m.detail && m.detailTab == 7 {
				return m, m.startDesktop(m.cursor)
			}
			if m.detail && m.detailTab == 8 {
				return m, m.startSSH(m.cursor)
			}
			m.detail = true
		case "tab":
			if m.detail {
				maxTabs := 9
				if m.desktopForServer(m.cursor) != nil {
					maxTabs = 10
				}
				m.detailTab = (m.detailTab + 1) % maxTabs
				m.detailScroll = 0
			}
		case "1":
			if m.detail {
				m.detailTab = 0
				m.detailScroll = 0
			}
		case "2":
			if m.detail {
				m.detailTab = 1
				m.detailScroll = 0
			}
		case "3", "4", "5", "6", "7":
			if m.detail {
				m.detailTab = int(msg.Runes[0] - '1')
				m.detailScroll = 0
			}
		case "8":
			if m.detail && m.desktopForServer(m.cursor) != nil {
				m.detailTab = 7
				m.detailScroll = 0
			}
		case "9":
			if m.detail {
				m.detailTab = 8
				m.detailScroll = 0
			}
		case "d":
			if m.detail {
				m.detailTab = 9
				m.detailScroll = 0
				return m, m.loadDevtoolsStatus(m.cursor)
			}
		case "c":
			if m.detail && m.detailTab == 7 {
				return m, openDesktop(m.desktopForServer(m.cursor))
			}
		case "s":
			if !m.detail {
				m.detail = true
				m.detailTab = 8
				m.sshText = ""
			} else if m.detailTab != 7 {
				m.detailTab = 8
				m.sshText = ""
			}
		case "x":
			if m.detailTab == 7 {
				if s := m.desktopStreams[m.cursor]; s != nil {
					s.Close()
				}
				m.desktopStreams[m.cursor] = nil
				m.desktopFrames[m.cursor] = ""
				m.desktopErrors[m.cursor] = ""
			}
			if m.detailTab == 8 && m.ssh != nil {
				m.ssh.close()
				m.ssh = nil
				m.sshText = ""
			}
		case "[":
			if m.detail && m.rangeIndex > 0 {
				m.rangeIndex--
				return m, m.loadHistory(m.cursor)
			}
		case "]":
			if m.detail && m.rangeIndex < len(historyRanges)-1 {
				m.rangeIndex++
				return m, m.loadHistory(m.cursor)
			}
		case "esc", "left", "h":
			for i, s := range m.desktopStreams {
				if s != nil {
					s.Close()
					m.desktopStreams[i] = nil
				}
			}
			m.detail = false
		case "r":
			if !m.collecting {
				m.pending = m.sshCount()
				m.collecting = m.pending > 0
				if m.pending > 0 {
					return m, tea.Batch(m.collectSSH()...)
				}
			}
		}
	case sshStartMsg:
		if msg.Err != nil {
			m.sshText = "SSH: " + msg.Err.Error()
			return m, nil
		}
		m.ssh = msg.Pane
		m.sshText = ""
		return m, m.ssh.read()
	case devtoolStatusMsg:
		m.devtoolBusy = false
		if msg.Err != nil {
			m.devtoolMessage = "status error: " + msg.Err.Error()
		} else {
			m.devtoolStatus = msg.Status
			m.devtoolVersions = msg.Versions
			m.devtoolMessage = ""
		}
		return m, nil
	case devtoolActionMsg:
		m.devtoolBusy = false
		if msg.Err != nil {
			m.devtoolMessage = msg.Action + " failed: " + msg.Err.Error()
		} else {
			m.devtoolMessage = msg.Action + " complete: " + strings.TrimSpace(msg.Output)
		}
		return m, m.loadDevtoolsStatus(m.cursor)
	case resultMsg:
		prev := m.samples[msg.Index]
		metrics.Derive(&prev, &msg.Sample)
		m.samples[msg.Index] = msg.Sample
		h := append(m.history[msg.Index], msg.Sample)
		if len(h) > m.cfg.HistorySize {
			h = h[len(h)-m.cfg.HistorySize:]
		}
		m.history[msg.Index] = h
		m.pending--
		if m.pending <= 0 {
			m.collecting = false
			m.lastRefresh = time.Now()
			return m, m.nextTick()
		}
	case streamMsg:
		if msg.Err != nil {
			if msg.Stream != nil {
				msg.Stream.Close()
			}
			m.samples[msg.Index] = metrics.Sample{At: time.Now(), Online: false, Error: "agent: " + msg.Err.Error()}
			return m, m.reconnect(msg.Index)
		}
		m.streamBuffers[msg.Index] = append(m.streamBuffers[msg.Index], msg.Sample)
		h := m.history[msg.Index]
		if len(msg.History) > 0 {
			h = msg.History
		}
		if len(h) == 0 || msg.Sample.At.Sub(h[len(h)-1].At) >= time.Second {
			h = append(h, msg.Sample)
		}
		if len(h) > m.cfg.HistorySize {
			h = h[len(h)-m.cfg.HistorySize:]
		}
		m.history[msg.Index] = h
		return m, m.readStream(msg.Index, msg.Stream)
	case reconnectMsg:
		return m, m.connectStream(int(msg))
	case historyMsg:
		if msg.Err == nil && len(msg.Samples) > 0 {
			m.history[msg.Index] = msg.Samples
		}
	case desktopShotMsg:
		if msg.Err != nil {
			m.desktopErrors[msg.Index] = msg.Err.Error()
			m.desktopFrames[msg.Index] = ""
		} else {
			m.desktopErrors[msg.Index] = ""
			m.desktopFrames[msg.Index] = msg.Frame
			if m.detail && m.detailTab == 7 && msg.Index == m.cursor {
				return m, m.nextDesktop(msg.Index)
			}
		}
	case desktopStreamMsg:
		if msg.Err != nil {
			m.desktopErrors[msg.Index] = msg.Err.Error()
			return m, nil
		}
		m.desktopStreams[msg.Index] = msg.Stream
		m.desktopErrors[msg.Index] = ""
		m.desktopFrames[msg.Index] = m.renderDesktopFrame(msg.Index, msg.Frame)
		return m, m.readDesktopStream(msg.Index)
	case desktopStreamFrameMsg:
		if msg.Err != nil {
			m.desktopErrors[msg.Index] = msg.Err.Error()
			if s := m.desktopStreams[msg.Index]; s != nil {
				s.Close()
			}
			m.desktopStreams[msg.Index] = nil
			return m, nil
		}
		m.desktopFrames[msg.Index] = m.renderDesktopFrame(msg.Index, msg.Frame)
		return m, m.readDesktopStream(msg.Index)
	case sshOutputMsg:
		if msg.Data != "" && m.ssh != nil {
			m.sshText = m.ssh.feed(msg.Data)
		}
		if msg.Err != nil {
			if m.ssh != nil {
				m.ssh.close()
				m.ssh = nil
			}
			return m, nil
		}
		if m.ssh != nil {
			return m, m.ssh.read()
		}
	case tea.MouseMsg:
		if m.detail && m.detailTab == 7 && msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonRight) {
			return m, m.sendDesktopClick(m.cursor, msg.X, msg.Y, msg.Button == tea.MouseButtonRight)
		}
	case desktopRefreshMsg:
		if m.detail && m.detailTab == 7 && msg.Index == m.cursor {
			return m, m.loadDesktop(msg.Index)
		}
	case frameMsg:
		target := time.Now().Add(-time.Second)
		for i, buf := range m.streamBuffers {
			for len(buf) >= 2 && !buf[1].At.After(target) {
				buf = buf[1:]
			}
			if len(buf) >= 2 && !buf[0].At.After(target) {
				m.samples[i] = interpolate(buf[0], buf[1], target)
			}
			m.streamBuffers[i] = buf
		}
		for i, s := range m.samples {
			m.displayCPU[i] += (s.CPUPercent - m.displayCPU[i]) * .22
			if len(m.displayCores[i]) != len(s.CorePercent) {
				m.displayCores[i] = append([]float64(nil), s.CorePercent...)
			} else {
				for j := range s.CorePercent {
					m.displayCores[i][j] += (s.CorePercent[j] - m.displayCores[i][j]) * .22
				}
			}
		}
		return m, m.nextFrame()
	case tickMsg:
		if m.collecting {
			return m, nil
		}
		m.collecting = true
		m.pending = m.sshCount()
		if m.pending == 0 {
			m.collecting = false
			return m, nil
		}
		return m, tea.Batch(m.collectSSH()...)
	}
	return m, nil
}
func interpolate(a, b metrics.Sample, at time.Time) metrics.Sample {
	out := a
	span := b.At.Sub(a.At).Seconds()
	alpha := 0.0
	if span > 0 {
		alpha = at.Sub(a.At).Seconds() / span
	}
	alpha = math.Max(0, math.Min(1, alpha))
	lerp := func(x, y float64) float64 { return x + (y-x)*alpha }
	out.At = at
	out.CPUPercent = lerp(a.CPUPercent, b.CPUPercent)
	out.NetRxRate = lerp(a.NetRxRate, b.NetRxRate)
	out.NetTxRate = lerp(a.NetTxRate, b.NetTxRate)
	out.MemAvailable = uint64(lerp(float64(a.MemAvailable), float64(b.MemAvailable)))
	if len(a.CorePercent) == len(b.CorePercent) {
		out.CorePercent = make([]float64, len(a.CorePercent))
		for i := range a.CorePercent {
			out.CorePercent[i] = lerp(a.CorePercent[i], b.CorePercent[i])
		}
	}
	return out
}
func (m Model) visual(i int) metrics.Sample {
	s := m.samples[i]
	s.CPUPercent = m.displayCPU[i]
	if len(m.displayCores[i]) == len(s.CorePercent) {
		s.CorePercent = m.displayCores[i]
	}
	return s
}

var historyRanges = []struct {
	label string
	span  time.Duration
}{{"1H", time.Hour}, {"24H", 24 * time.Hour}, {"7D", 7 * 24 * time.Hour}, {"30D", 30 * 24 * time.Hour}}

func (m Model) loadHistory(i int) tea.Cmd {
	return func() tea.Msg {
		s := m.cfg.Servers[i]
		if s.AgentURL == "" {
			return historyMsg{Index: i, Err: fmt.Errorf("history requires agent")}
		}
		token := os.Getenv(s.TokenEnv)
		if s.TokenFile != "" {
			b, err := os.ReadFile(config.ExpandHome(s.TokenFile))
			if err != nil {
				return historyMsg{Index: i, Err: err}
			}
			token = strings.TrimSpace(string(b))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		samples, err := agentclient.History(ctx, s.AgentURL, token, historyRanges[m.rangeIndex].span, m.cfg.HistorySize)
		return historyMsg{Index: i, Samples: samples, Err: err}
	}
}
func (m Model) sshCount() int {
	n := 0
	for _, s := range m.cfg.Servers {
		if s.AgentURL == "" {
			n++
		}
	}
	return n
}
func (m Model) collectSSH() []tea.Cmd {
	cmds := []tea.Cmd{}
	for i, s := range m.cfg.Servers {
		if s.AgentURL == "" {
			cmds = append(cmds, m.collect(i))
		}
	}
	return cmds
}
func (m Model) View() string {
	if m.width == 0 {
		return "Starting servterm..."
	}
	var body string
	if m.detail {
		body = m.detailView()
	} else {
		body = m.overview()
	}
	help := "  ↑/↓ navigate  enter details  s SSH  esc overview  r refresh  q quit"
	if m.detail {
		help = "  tab / 1..9 widgets  s SSH  c connect desktop  [ / ] history  j/k scroll  esc overview  q quit   LIVE -1.0s • 10fps"
		if m.ssh != nil {
			help = "  SSH session active  esc close session  ctrl-c remote"
		}
	}
	header := m.header()
	footer := dimStyle.Render(help)
	if m.ssh != nil || m.detail {
		available := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer))
		body = clipLines(body, m.detailScroll, available)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
func clipLines(s string, start, count int) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	start = min(start, max(0, len(lines)-count))
	return strings.Join(lines[start:min(len(lines), start+count)], "\n")
}
func (m Model) header() string {
	online := 0
	for _, s := range m.samples {
		if s.Online {
			online++
		}
	}
	status := fmt.Sprintf("%d/%d online", online, len(m.samples))
	if m.collecting {
		status += "  • refreshing"
	} else if !m.lastRefresh.IsZero() {
		status += "  • updated " + humanAgo(m.lastRefresh)
	}
	return lipgloss.NewStyle().Padding(1, 2).Width(max(0, m.width-4)).Render(titleStyle.Render("SERVTERM") + "  " + dimStyle.Render("fleet observability") + lipgloss.NewStyle().Foreground(green).Render("  "+status))
}
func (m Model) overview() string {
	if m.width < 85 {
		return m.compactOverview()
	}
	head := dimStyle.Render(fmt.Sprintf("  %-2s %-18s %-10s %-8s %-9s %-9s %-8s %-12s %s", "", "SERVER", "LOCATION", "LATENCY", "CPU", "MEMORY", "DISK /", "NET", "CPU HISTORY"))
	rows := []string{head}
	for i := range m.samples {
		s := m.visual(i)
		srv := m.cfg.Servers[i]
		mark := " "
		if i == m.cursor {
			mark = "›"
		}
		name := truncate(srv.Name, 18)
		loc := truncate(or(srv.Location, "—"), 10)
		lat, cpu, mem, disk, net, hist := "…", "…", "…", "…", "…", ""
		if !s.At.IsZero() && !s.Online {
			lat = errStyle.Render("offline")
			cpu, mem, disk, net = "—", "—", "—", "—"
		} else if s.Online {
			lat = fmt.Sprintf("%4d ms", s.Latency.Milliseconds())
			cpu = percentText(s.CPUPercent)
			mem = percentText(metrics.Percent(s.MemTotal-s.MemAvailable, s.MemTotal))
			disk = percentText(rootDisk(s))
			net = fmt.Sprintf("↓%s ↑%s", rate(s.NetRxRate), rate(s.NetTxRate))
			hist = spark(m.history[i], func(x metrics.Sample) float64 { return x.CPUPercent })
		}
		line := fmt.Sprintf("  %-2s %-18s %-10s %-8s %-9s %-9s %-8s %-12s %s", mark, name, loc, lat, cpu, mem, disk, net, hist)
		if i == m.cursor {
			line = lipgloss.NewStyle().Background(panel).Bold(true).Render(pad(line, m.width))
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n") + "\n"
}
func (m Model) compactOverview() string {
	rows := []string{}
	for i := range m.samples {
		s := m.visual(i)
		mark := " "
		if i == m.cursor {
			mark = "›"
		}
		line := fmt.Sprintf("  %s %-18s ", mark, truncate(m.cfg.Servers[i].Name, 18))
		if s.At.IsZero() {
			line += "waiting…"
		} else if !s.Online {
			line += errStyle.Render("offline")
		} else {
			line += fmt.Sprintf("%dms  CPU %s  MEM %s", s.Latency.Milliseconds(), percentText(s.CPUPercent), percentText(metrics.Percent(s.MemTotal-s.MemAvailable, s.MemTotal)))
		}
		if i == m.cursor {
			line = lipgloss.NewStyle().Background(panel).Render(pad(line, m.width))
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n") + "\n"
}
func (m Model) detailView() string {
	srv := m.cfg.Servers[m.cursor]
	s := m.visual(m.cursor)
	back := dimStyle.Render("OVERVIEW / ") + titleStyle.Render(srv.Name)
	if s.At.IsZero() {
		return "  " + back + "\n\n  Waiting for first sample…\n"
	}
	if !s.Online {
		return "  " + back + "\n\n  " + errStyle.Bold(true).Render("OFFLINE") + "  " + truncate(s.Error, max(20, m.width-16)) + "\n"
	}
	info := fmt.Sprintf("  %s\n  %s  •  %s  •  kernel %s\n  %d cores  •  uptime %s  •  %d ms\n\n", back, or(s.Hostname, srv.Address), s.OS, s.Kernel, s.Cores, duration(s.UptimeSeconds), s.Latency.Milliseconds())
	labels := []string{"1 CPU", "2 MEMORY", "3 STORAGE", "4 NETWORK", "5 RUNNERS", "6 PROCESSES", "7 ACCEL"}
	if desktop := m.desktopForServer(m.cursor); desktop != nil {
		labels = append(labels, "8 DESKTOP")
	}
	labels = append(labels, "9 SSH")
	labels = append(labels, "10 DEVTOOLS")
	tabs := "  "
	for i, label := range labels {
		tabs += tabLabel(label, m.detailTab == i) + "  "
	}
	tabs += "   " + dimStyle.Render("history ") + titleStyle.Render(historyRanges[m.rangeIndex].label) + "  " + dimStyle.Render("[ / ]") + "\n\n"
	common := statStrip(s)
	var body string
	switch m.detailTab {
	case 0:
		body = cpuView(s, m.width)
	case 1:
		body = memoryView(s, m.history[m.cursor])
	case 2:
		body = storageView(s)
	case 3:
		body = networkView(s, m.history[m.cursor])
	case 4:
		body = runnerView(s, m.history[m.cursor], m.width)
	case 5:
		body = processView(s)
	case 6:
		body = acceleratorView(s)
	case 7:
		body = desktopView(m.desktopForServer(m.cursor), m.desktopFrames[m.cursor], m.desktopErrors[m.cursor], m.width)
	case 8:
		body = "  SSH SESSION\n\n" + m.sshText
		if m.ssh == nil && m.sshText == "" {
			body += "  Press Enter to connect.\n  x disconnects and keeps the tab open.\n"
		}
	case 9:
		body = m.devtoolsView()
	}
	return info + common + tabs + body
}

func (m Model) desktopForServer(index int) *config.Desktop {
	if index < 0 || index >= len(m.cfg.Servers) {
		return nil
	}
	host := m.cfg.Servers[index].Address
	for i := range m.cfg.Desktops {
		if m.cfg.Desktops[i].Host == host {
			return &m.cfg.Desktops[i]
		}
	}
	return nil
}
func desktopView(desktop *config.Desktop, frame, errText string, width int) string {
	if desktop == nil {
		return "  No desktop is configured for this server.\n\n"
	}
	if errText != "" {
		return "  DESKTOP\n\n  " + errStyle.Render("screenshot unavailable: "+errText) + "\n\n  Press 8 to retry.\n\n"
	}
	if frame == "" {
		return fmt.Sprintf("  DESKTOP\n\n  %-14s %s\n  %-14s %s\n\n  Press Enter to connect.\n  x disconnects and keeps the tab open.\n\n", "NAME", desktop.Name, "PLATFORM", desktop.Platform)
	}
	if frame != "" {
		return "  DESKTOP  " + desktop.Name + "\n\n" + frame + "\n"
	}
	port := desktop.VNCPort
	if port == 0 {
		port = 5900
	}
	return fmt.Sprintf("  DESKTOP\n\n  %-14s %s\n  %-14s %s\n  %-14s %d\n  %-14s %s\n\n  %s\n\n", "NAME", desktop.Name, "PLATFORM", desktop.Platform, "VNC PORT", port, "AGENT", desktop.AgentURL, dimStyle.Render("c connect  •  view-only by default  •  agent input requires confirmation"))
}
func (m Model) devtoolsView() string {
	var b strings.Builder
	b.WriteString("  DEV TOOLS\n\n  TOOL           STATUS       DESCRIPTION\n")
	for i, t := range devtools.Catalog {
		state := "missing"
		stateStyle := errStyle
		if m.devtoolStatus[t.Command] {
			state = "installed"
			stateStyle = okStyle
		}
		version := m.devtoolVersions[t.Command]
		if version == "" {
			version = "—"
		}
		lineStyle := lipgloss.NewStyle()
		if i == m.devtoolCursor {
			lineStyle = lipgloss.NewStyle().Background(panel).Bold(true).Foreground(cyan)
		}
		b.WriteString(lineStyle.Render(fmt.Sprintf("  %-14s ", t.ID)))
		b.WriteString(stateStyle.Render(fmt.Sprintf("%-11s", state)))
		b.WriteString(dimStyle.Render(fmt.Sprintf(" %-18s", version)))
		b.WriteString(lineStyle.Render("  " + t.Description + "\n"))
	}
	if m.devtoolBusy {
		b.WriteString(warnStyle.Render("\n  ◌ working…\n"))
	}
	if m.devtoolConfirm != "" {
		fmt.Fprintf(&b, "\n  Confirm %s: press Enter again; Esc cancels.\n", m.devtoolConfirm)
	} else {
		b.WriteString("\n  ↑/↓ select  Enter/i install  u uninstall  Esc cancel\n")
	}
	if m.devtoolMessage != "" {
		b.WriteString("  " + m.devtoolMessage + "\n")
	}
	return b.String()
}
func (m Model) startSSH(index int) tea.Cmd {
	return func() tea.Msg {
		pane, err := startSSHPane(m.cfg.Servers[index])
		return sshStartMsg{Pane: pane, Err: err}
	}
}
func (m Model) loadDesktop(index int) tea.Cmd {
	return func() tea.Msg {
		d := m.desktopForServer(index)
		if d == nil {
			return desktopShotMsg{Index: index, Err: fmt.Errorf("no desktop configured")}
		}
		token := ""
		if d.TokenEnv != "" {
			token = os.Getenv(d.TokenEnv)
		} else if d.TokenFile != "" {
			b, err := os.ReadFile(config.ExpandHome(d.TokenFile))
			if err != nil {
				return desktopShotMsg{Index: index, Err: err}
			}
			token = strings.TrimSpace(string(b))
		}
		b, err := desktopclient.FetchScreenshot(context.Background(), *d, token)
		if err != nil {
			return desktopShotMsg{Index: index, Err: err}
		}
		img, err := png.Decode(stdbuf.NewReader(b))
		if err != nil {
			return desktopShotMsg{Index: index, Err: err}
		}
		cols := max(40, min(110, m.width-4))
		switch d.Quality {
		case "speed":
			cols = max(40, min(80, m.width-4))
		case "quality":
			cols = max(40, min(180, m.width-4))
		}
		if inline := emitDesktopImage(b, cols, 24); inline != "" {
			return desktopShotMsg{Index: index, Frame: inline}
		}
		return desktopShotMsg{Index: index, Frame: ansiFrame(img, cols)}
	}
}
func (m Model) loadDevtoolsStatus(index int) tea.Cmd {
	return func() tea.Msg {
		if index < 0 || index >= len(m.cfg.Servers) {
			return devtoolStatusMsg{Err: fmt.Errorf("no server")}
		}
		detailed, err := devtools.StatusDetailed(context.Background(), m.cfg.Servers[index])
		status := map[string]bool{}
		versions := map[string]string{}
		for k, v := range detailed {
			status[k] = v.Installed
			versions[k] = v.Version
		}
		return devtoolStatusMsg{Status: status, Versions: versions, Err: err}
	}
}
func (m Model) runDevtoolAction(index, cursor int, remove bool) tea.Cmd {
	return func() tea.Msg {
		if index < 0 || index >= len(m.cfg.Servers) || cursor < 0 || cursor >= len(devtools.Catalog) {
			return devtoolActionMsg{Err: fmt.Errorf("invalid tool")}
		}
		t := devtools.Catalog[cursor]
		out, err := devtools.Install(context.Background(), m.cfg.Servers[index], t.ID, remove)
		action := "install"
		if remove {
			action = "uninstall"
		}
		return devtoolActionMsg{Tool: t.ID, Action: action, Output: out, Err: err}
	}
}
func (m Model) startDesktop(index int) tea.Cmd {
	return func() tea.Msg {
		d := m.desktopForServer(index)
		if d == nil {
			return desktopStreamMsg{Index: index, Err: fmt.Errorf("no desktop configured")}
		}
		token := ""
		if d.TokenEnv != "" {
			token = os.Getenv(d.TokenEnv)
		} else if d.TokenFile != "" {
			b, err := os.ReadFile(config.ExpandHome(d.TokenFile))
			if err != nil {
				return desktopStreamMsg{Index: index, Err: err}
			}
			token = strings.TrimSpace(string(b))
		}
		s, err := desktopclient.OpenStream(context.Background(), *d, token)
		if err != nil {
			return desktopStreamMsg{Index: index, Err: err}
		}
		frame, err := s.Read(context.Background())
		if err != nil {
			s.Close()
			return desktopStreamMsg{Index: index, Err: err}
		}
		return desktopStreamMsg{Index: index, Stream: s, Frame: frame}
	}
}
func (m Model) readDesktopStream(index int) tea.Cmd {
	s := m.desktopStreams[index]
	if s == nil {
		return nil
	}
	return func() tea.Msg {
		b, err := s.Read(context.Background())
		return desktopStreamFrameMsg{Index: index, Frame: b, Err: err}
	}
}
func (m Model) renderDesktopFrame(index int, b []byte) string {
	d := m.desktopForServer(index)
	cols := max(40, min(110, m.width-4))
	if d != nil {
		switch d.Quality {
		case "speed":
			cols = max(40, min(80, m.width-4))
		case "quality":
			cols = max(40, min(180, m.width-4))
		}
	}
	if inline := emitDesktopImage(b, cols, 24); inline != "" {
		return inline
	}
	img, err := png.Decode(stdbuf.NewReader(b))
	if err != nil {
		return ""
	}
	return ansiFrame(img, cols)
}
func isRemoteDesktopKey(key string) bool {
	if key == "q" || key == "ctrl+c" || key == "esc" || key == "tab" || key == "8" || key == "c" {
		return false
	}
	return key != "" && key != "up" && key != "down" && key != "left" && key != "right" || key == "up" || key == "down" || key == "left" || key == "right"
}
func (m Model) sendDesktopKey(index int, combo string) tea.Cmd {
	return func() tea.Msg {
		d := m.desktopForServer(index)
		if d == nil {
			return nil
		}
		token := ""
		if d.TokenEnv != "" {
			token = os.Getenv(d.TokenEnv)
		} else if d.TokenFile != "" {
			b, err := os.ReadFile(config.ExpandHome(d.TokenFile))
			if err != nil {
				return nil
			}
			token = strings.TrimSpace(string(b))
		}
		_ = desktopclient.SendKey(context.Background(), *d, token, combo)
		return nil
	}
}
func (m Model) sendDesktopClick(index, x, y int, right bool) tea.Cmd {
	return func() tea.Msg {
		d := m.desktopForServer(index)
		if d == nil {
			return nil
		}
		token := ""
		if d.TokenEnv != "" {
			token = os.Getenv(d.TokenEnv)
		} else if d.TokenFile != "" {
			b, err := os.ReadFile(config.ExpandHome(d.TokenFile))
			if err != nil {
				return nil
			}
			token = strings.TrimSpace(string(b))
		}
		remoteX := max(0, min(1279, x*1280/max(1, m.width)))
		remoteY := max(0, min(799, (y-10)*800/24))
		_ = right
		_ = desktopclient.Click(context.Background(), *d, token, remoteX, remoteY)
		return nil
	}
}
func (m Model) nextDesktop(index int) tea.Cmd {
	d := m.desktopForServer(index)
	fps := 60
	if d != nil && d.RefreshFPS > 0 {
		fps = d.RefreshFPS
	}
	if fps > 60 {
		fps = 60
	}
	interval := time.Second / time.Duration(fps)
	return tea.Tick(interval, func(time.Time) tea.Msg { return desktopRefreshMsg{Index: index} })
}

type desktopRefreshMsg struct{ Index int }

func ansiFrame(src image.Image, maxCols int) string {
	b := src.Bounds()
	scale := 1.0
	if b.Dx() > maxCols {
		scale = float64(maxCols) / float64(b.Dx())
	}
	w := max(1, int(float64(b.Dx())*scale))
	h := max(1, int(float64(b.Dy())*scale))
	px := func(x, y int) (uint8, uint8, uint8) {
		sx := b.Min.X + int(float64(x)/scale)
		sy := b.Min.Y + int(float64(y)/scale)
		r, g, bl, _ := src.At(sx, sy).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8)
	}
	var out strings.Builder
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			r1, g1, b1 := px(x, y)
			if y+1 < h {
				r2, g2, b2 := px(x, y+1)
				fmt.Fprintf(&out, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", r1, g1, b1, r2, g2, b2)
			} else {
				fmt.Fprintf(&out, "\x1b[38;2;%d;%d;%dm▀", r1, g1, b1)
			}
		}
		out.WriteString("\x1b[0m\n")
	}
	return out.String()
}
func openDesktop(desktop *config.Desktop) tea.Cmd {
	return func() tea.Msg {
		if desktop == nil {
			return nil
		}
		port := desktop.VNCPort
		if port == 0 {
			port = 5900
		}
		uri := fmt.Sprintf("vnc://%s:%d", desktop.Host, port)
		if runtime.GOOS == "darwin" {
			_ = exec.Command("open", uri).Run()
		} else {
			_ = exec.Command("xdg-open", uri).Run()
		}
		return nil
	}
}
func openSSH(server config.Server) tea.Cmd {
	args := []string{"-o", "BatchMode=yes"}
	if server.Port != 0 {
		args = append(args, "-p", fmt.Sprint(server.Port))
	}
	if server.IdentityFile != "" {
		args = append(args, "-i", config.ExpandHome(server.IdentityFile))
	}
	target := server.Address
	if server.User != "" {
		target = server.User + "@" + target
	}
	args = append(args, target)
	cmd := exec.Command("ssh", args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return nil })
}
func tabLabel(label string, active bool) string {
	style := dimStyle
	if active {
		style = lipgloss.NewStyle().Bold(true).Foreground(cyan).Underline(true)
	}
	return style.Render(label)
}
func cpuView(s metrics.Sample, width int) string {
	barWidth := 28
	if width < 90 {
		barWidth = 18
	}
	usedCores := s.CPUPercent / 100 * float64(s.Cores)
	pressure := "n/a"
	if s.PressureCPU > 0 || s.PressureMemory > 0 || s.PressureIO > 0 {
		pressure = fmt.Sprintf("%.2f%%", max(s.PressureCPU, max(s.PressureMemory, s.PressureIO)))
	}
	power := "n/a"
	if s.PowerKnown {
		power = fmt.Sprintf("%.1f W", s.PowerWatts)
	}
	if s.BatteryKnown {
		state := "on battery"
		if s.BatteryCharging {
			state = "charging"
		}
		power = fmt.Sprintf("%s  %.0f%%", power, s.BatteryPercent)
		if s.PowerKnown {
			power += "  " + state
		}
	}
	out := fmt.Sprintf("  TOTAL CPU  %s %5.1f%%   (~%.1f/%d cores)\n  LOAD       %.2f / %.2f / %.2f   PRESSURE %s   POWER %s\n", bar(s.CPUPercent, barWidth), s.CPUPercent, usedCores, s.Cores, s.Load1, s.Load5, s.Load15, pressure, power)
	if len(s.CorePercent) == 0 {
		return out + "\n  Per-core breakdown is unavailable from this macOS sampler.\n\n"
	}
	out += "\n  LOGICAL CPU USAGE\n"
	cols := 4
	if width < 90 {
		cols = 2
	}
	cellWidth := 22
	for i, v := range s.CorePercent {
		out += fmt.Sprintf("  %s", fmt.Sprintf("CPU%02d %s %5.1f%%", i, bar(v, 7), v))
		if (i+1)%cols == 0 {
			out += "\n"
		} else {
			out += strings.Repeat(" ", max(1, cellWidth-18))
		}
	}
	if len(s.CorePercent)%cols != 0 {
		out += "\n"
	}
	return out + "\n"
}
func processView(s metrics.Sample) string {
	out := "  TOP RESOURCE USERS (executable names only)\n  " + dimStyle.Render(fmt.Sprintf("%-7s %-11s %-22s %7s %7s %8s", "PID", "USER", "COMMAND", "CPU", "MEM", "RSS")) + "\n"
	limit := min(8, len(s.Processes))
	for _, p := range s.Processes[:limit] {
		out += fmt.Sprintf("  %-7d %-11s %-22s %s %s %8s\n", p.PID, truncate(p.User, 11), truncate(p.Command, 22), usageText(p.CPU), usageText(p.Memory), bytes(p.RSS))
	}
	return out + "\n"
}
func acceleratorView(s metrics.Sample) string {
	out := "  GPU / NPU ACCELERATORS\n\n"
	if len(s.Accelerators) == 0 {
		return out + dimStyle.Render("  No supported accelerator was detected.") + "\n"
	}
	for _, accelerator := range s.Accelerators {
		kind := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(fmt.Sprintf("%-4s", accelerator.Kind))
		usage := dimStyle.Render("activity unavailable")
		if accelerator.UtilizationKnown {
			usage = usageText(accelerator.Utilization) + "  " + bar(accelerator.Utilization, 18)
		}
		out += fmt.Sprintf("  %s  %-38s %s\n", kind, truncate(accelerator.Name, 38), usage)
	}
	return out + "\n"
}
func memoryView(s metrics.Sample, h []metrics.Sample) string {
	used := s.MemTotal - s.MemAvailable
	swap := s.SwapTotal - s.SwapFree
	return fmt.Sprintf("  MEMORY  %s %s  %s / %s\n  HISTORY %s\n  SWAP    %s %s  %s / %s\n  PRESSURE %.2f%%\n\n", bar(metrics.Percent(used, s.MemTotal), 32), percentText(metrics.Percent(used, s.MemTotal)), bytes(used), bytes(s.MemTotal), spark(h, func(x metrics.Sample) float64 { return metrics.Percent(x.MemTotal-x.MemAvailable, x.MemTotal) }), bar(metrics.Percent(swap, s.SwapTotal), 32), percentText(metrics.Percent(swap, s.SwapTotal)), bytes(swap), bytes(s.SwapTotal), s.PressureMemory)
}
func networkView(s metrics.Sample, h []metrics.Sample) string {
	iface := s.NetworkInterface
	if iface == "" {
		iface = "unknown interface"
	}
	kind := s.NetworkType
	if kind == "" {
		kind = "unknown"
	}
	link := "speed unavailable"
	if s.NetworkLinkMbps > 0 {
		link = fmt.Sprintf("%d Mbps", s.NetworkLinkMbps)
	}
	linkStyle := okStyle
	if s.NetworkLinkMbps > 0 && s.NetworkLinkMbps < 100 {
		linkStyle = warnStyle
	}
	errText := fmt.Sprintf("errors RX %d / TX %d   drops RX %d / TX %d", s.NetRxErrors, s.NetTxErrors, s.NetRxDrops, s.NetTxDrops)
	errStyle := dimStyle
	if s.NetRxErrors+s.NetTxErrors+s.NetRxDrops+s.NetTxDrops > 0 {
		errStyle = errStyle.Bold(true)
	}
	return fmt.Sprintf("  CONNECTION  %s  %s  %s\n  LINK        %s\n  LIVE THROUGHPUT\n  DOWNLOAD  %10s/s  %s\n  UPLOAD    %10s/s  %s\n  RX TOTAL  %s\n  TX TOTAL  %s\n  %s\n\n", titleStyle.Render(iface), dimStyle.Render(kind), linkStyle.Render(link), linkStyle.Render(link), bytes(uint64(s.NetRxRate)), spark(h, func(x metrics.Sample) float64 { return math.Min(100, x.NetRxRate/1024/1024) }), bytes(uint64(s.NetTxRate)), spark(h, func(x metrics.Sample) float64 { return math.Min(100, x.NetTxRate/1024/1024) }), bytes(s.NetRx), bytes(s.NetTx), errStyle.Render(errText))
}
func runnerView(s metrics.Sample, h []metrics.Sample, width int) string {
	jobs := "jobs"
	if s.Runners.ActiveJobs == 1 {
		jobs = "job"
	}
	cores := s.Runners.CPU / 100
	hostShare := 0.0
	if s.Cores > 0 {
		hostShare = s.Runners.CPU / float64(s.Cores)
	}
	summary := fmt.Sprintf("%s  %d listeners    %s  %d %s\n%s  %.2f cores / %.1f%% host    %s  %s RSS", titleStyle.Render("RUNNERS"), s.Runners.Listeners, okStyle.Bold(true).Render("ACTIVE"), s.Runners.ActiveJobs, jobs, warnStyle.Render("CPU"), cores, hostShare, lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA")).Render("MEM"), bytes(s.Runners.RSS))
	out := lipgloss.NewStyle().MarginLeft(2).Border(lipgloss.RoundedBorder()).BorderForeground(panel).Padding(0, 1).Render(summary) + "\n\n"
	if len(s.RunnerJobs) == 0 {
		return out + dimStyle.Render("  Waiting for sanitized runner probe metadata…") + "\n"
	}
	out += "  " + titleStyle.Render("ACTIVE JOBS") + "  " + dimStyle.Render("live process-tree usage") + "\n\n"
	cardWidth := max(48, min(76, width-8))
	for _, job := range s.RunnerJobs {
		jobCores := job.CPU / 100
		share := 0.0
		if s.Cores > 0 {
			share = job.CPU / float64(s.Cores)
		}
		title := job.Workflow + " / " + job.Job
		if title == " / " {
			title = "job metadata pending"
		}
		accent := green
		if share >= 20 {
			accent = red
		} else if share >= 8 {
			accent = yellow
		}
		busy := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("● BUSY")
		runner := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(strings.ToUpper(or(job.Runner, "UNKNOWN RUNNER")))
		repo := dimStyle.Render(or(job.Repository, "repository pending"))
		heading := runner + "  " + busy + "\n" + repo
		workflow := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Render(title)
		run := dimStyle.Render("run #" + or(job.RunID, "pending") + "  •  " + elapsedDuration(time.Since(job.StartedAt)))
		cpuStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)
		memStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
		metricsLine := cpuStyle.Render(fmt.Sprintf("CPU %.2f cores", jobCores)) + dimStyle.Render("  •  ") + cpuStyle.Render(fmt.Sprintf("%.1f%% host", share)) + dimStyle.Render("  •  ") + memStyle.Render("RAM "+bytes(job.RSS)) + dimStyle.Render(fmt.Sprintf("  •  %d processes", job.Processes))
		chart := dimStyle.Render("CPU HISTORY  ") + lipgloss.NewStyle().Foreground(accent).Render(jobSpark(h, job.Runner))
		content := heading + "\n" + workflow + "\n" + run + "\n" + metricsLine + "\n" + chart
		card := lipgloss.NewStyle().Width(cardWidth).Border(lipgloss.RoundedBorder()).BorderLeft(true).BorderForeground(accent).Padding(0, 1).Render(content)
		out += lipgloss.NewStyle().MarginLeft(2).Render(card) + "\n"
	}
	return out + "\n"
}
func jobSpark(h []metrics.Sample, runner string) string {
	return spark(h, func(s metrics.Sample) float64 {
		for _, j := range s.RunnerJobs {
			if j.Runner == runner {
				return math.Min(100, j.CPU)
			}
		}
		return 0
	})
}
func storageView(s metrics.Sample) string {
	disks := "  FILESYSTEM USAGE\n"
	sort.Slice(s.Disks, func(i, j int) bool { return s.Disks[i].Mount < s.Disks[j].Mount })
	shown := 0
	for _, d := range s.Disks {
		if strings.HasPrefix(d.Mount, "/snap/") {
			continue
		}
		disks += fmt.Sprintf("  %-18s %s  %5.1f%%  %7s used / %-7s  %s\n", truncate(d.Mount, 18), diskBar(metrics.Percent(d.Used, d.Total), 24, shown), metrics.Percent(d.Used, d.Total), bytes(d.Used), bytes(d.Total), d.FSType)
		shown++
	}
	if len(s.Devices) > 0 {
		disks += "\n  STORAGE DEVICES\n"
		for _, d := range s.Devices {
			disks += fmt.Sprintf("  %-10s %-3s  %s\n", d.Name, strings.ToUpper(d.Kind), bytes(d.Size))
		}
	}
	return disks + "\n"
}
func diskBar(v float64, n, index int) string {
	palette := []lipgloss.Color{cyan, green, lipgloss.Color("#A78BFA"), yellow, lipgloss.Color("#F472B6"), lipgloss.Color("#38BDF8")}
	v = math.Max(0, math.Min(100, v))
	filled := int(math.Round(v / 100 * float64(n)))
	color := palette[index%len(palette)]
	if v >= 90 {
		color = red
	} else if v >= 75 {
		color = yellow
	}
	used := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	free := dimStyle.Render(strings.Repeat("░", n-filled))
	return used + free
}
func percentText(v float64) string {
	t := fmt.Sprintf("%5.1f%%", v)
	if v >= 90 {
		return errStyle.Render(t)
	}
	if v >= 75 {
		return warnStyle.Render(t)
	}
	return okStyle.Render(t)
}
func bar(v float64, n int) string {
	v = math.Max(0, math.Min(100, v))
	filled := int(math.Round(v / 100 * float64(n)))
	color := green
	if v >= 90 {
		color = red
	} else if v >= 75 {
		color = yellow
	}
	return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", n-filled))
}
func usageText(v float64) string {
	style := okStyle
	if v >= 90 {
		style = errStyle
	} else if v >= 60 {
		style = warnStyle
	}
	return style.Render(fmt.Sprintf("%6.1f%%", v))
}
func statStrip(s metrics.Sample) string {
	mem := metrics.Percent(s.MemTotal-s.MemAvailable, s.MemTotal)
	disk := rootDisk(s)
	cores := s.CPUPercent / 100 * float64(s.Cores)
	cpuPart := usageText(s.CPUPercent) + fmt.Sprintf(" (~%.1f/%d cores)", cores, s.Cores)
	memPart := usageText(mem) + " " + bytes(s.MemTotal-s.MemAvailable) + "/" + bytes(s.MemTotal)
	diskPart := usageText(disk)
	pressure := max(s.PressureCPU, max(s.PressureMemory, s.PressureIO))
	pressurePart := "n/a"
	if pressure > 0 {
		pressurePart = usageText(pressure)
	}
	line1 := titleStyle.Render("SERVER HEALTH") + "   CPU " + cpuPart + "   MEM " + memPart
	line2 := "DISK / " + diskPart + "   NET ↓" + bytes(uint64(s.NetRxRate)) + "/s ↑" + bytes(uint64(s.NetTxRate)) + "/s   PRESSURE " + pressurePart
	return lipgloss.NewStyle().MarginLeft(2).MarginBottom(1).Border(lipgloss.RoundedBorder()).BorderForeground(panel).Padding(0, 1).Render(line1+"\n"+line2) + "\n"
}
func spark(h []metrics.Sample, value func(metrics.Sample) float64) string {
	chars := []rune("▁▂▃▄▅▆▇█")
	if len(h) == 0 {
		return ""
	}
	start := max(0, len(h)-18)
	var b strings.Builder
	for _, s := range h[start:] {
		if !s.Online {
			b.WriteRune('·')
			continue
		}
		i := int(math.Round(math.Max(0, math.Min(100, value(s))) / 100 * 7))
		b.WriteRune(chars[i])
	}
	return b.String()
}
func rootDisk(s metrics.Sample) float64 {
	for _, d := range s.Disks {
		if d.Mount == "/" {
			return metrics.Percent(d.Used, d.Total)
		}
	}
	return 0
}
func bytes(v uint64) string {
	units := []string{"B", "K", "M", "G", "T"}
	f := float64(v)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d%s", v, units[i])
	}
	return fmt.Sprintf("%.1f%s", f, units[i])
}
func rate(v float64) string { return bytes(uint64(v)) + "/s" }
func duration(sec float64) string {
	d := time.Duration(sec) * time.Second
	days := d / (24 * time.Hour)
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, (d%(24*time.Hour))/time.Hour)
	}
	return d.Truncate(time.Minute).String()
}
func elapsedDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Truncate(time.Second).String()
}
func humanAgo(t time.Time) string { return time.Since(t).Truncate(time.Second).String() + " ago" }
func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
func truncate(s string, n int) string {
	if n <= 1 {
		return s[:min(len(s), n)]
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
func pad(s string, n int) string {
	w := lipgloss.Width(s)
	if w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}
