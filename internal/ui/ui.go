package ui

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/franciscosainzwilliams/server-term/internal/agentclient"
	"github.com/franciscosainzwilliams/server-term/internal/collector"
	"github.com/franciscosainzwilliams/server-term/internal/config"
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
type Model struct {
	cfg           config.Config
	collector     collector.Collector
	samples       []metrics.Sample
	history       [][]metrics.Sample
	cursor        int
	detail        bool
	detailTab     int
	detailScroll  int
	rangeIndex    int
	displayCPU    []float64
	displayCores  [][]float64
	streamBuffers [][]metrics.Sample
	width, height int
	collecting    bool
	pending       int
	lastRefresh   time.Time
}

func New(cfg config.Config) Model {
	n := 0
	for _, server := range cfg.Servers {
		if server.AgentURL == "" {
			n++
		}
	}
	return Model{cfg: cfg, collector: collector.Collector{SSH: cfg.SSH}, samples: make([]metrics.Sample, len(cfg.Servers)), history: make([][]metrics.Sample, len(cfg.Servers)), displayCPU: make([]float64, len(cfg.Servers)), displayCores: make([][]float64, len(cfg.Servers)), streamBuffers: make([][]metrics.Sample, len(cfg.Servers)), collecting: n > 0, pending: n}
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
	case tea.KeyMsg:
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
			m.detail = true
		case "tab":
			if m.detail {
				m.detailTab = (m.detailTab + 1) % 7
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
	help := "  ↑/↓ navigate  enter details  esc overview  r refresh  q quit"
	if m.detail {
		help = "  tab / 1..7 widgets  [ / ] history  j/k scroll  esc overview  q quit   LIVE -1.0s • 10fps"
	}
	header := m.header()
	footer := dimStyle.Render(help)
	if m.detail {
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
	}
	return info + common + tabs + body
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
	out := fmt.Sprintf("  TOTAL CPU  %s %5.1f%%   (~%.1f/%d cores)\n  LOAD       %.2f / %.2f / %.2f   PRESSURE %.2f%%\n", bar(s.CPUPercent, barWidth), s.CPUPercent, usedCores, s.Cores, s.Load1, s.Load5, s.Load15, s.PressureCPU)
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
	return fmt.Sprintf("  LIVE THROUGHPUT\n  DOWNLOAD  %10s/s  %s\n  UPLOAD    %10s/s  %s\n  RX TOTAL  %s\n  TX TOTAL  %s\n\n", bytes(uint64(s.NetRxRate)), spark(h, func(x metrics.Sample) float64 { return math.Min(100, x.NetRxRate/1024/1024) }), bytes(uint64(s.NetTxRate)), spark(h, func(x metrics.Sample) float64 { return math.Min(100, x.NetTxRate/1024/1024) }), bytes(s.NetRx), bytes(s.NetTx))
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
	pressurePart := usageText(pressure)
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
