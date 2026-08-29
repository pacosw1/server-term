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
	"github.com/franciscosainzwilliams/server-term/internal/widget"
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
type orchestratorMsg widget.OrchestratorSnapshot
type orchestratorTickMsg time.Time
type orchestratorModeMsg widget.OrchestratorModeResult
type cipMsg widget.CIPSnapshot
type cipTickMsg time.Time
type cipRunMsg widget.CIPRunDetail
type cipRunTickMsg time.Time
type cipPromotionsMsg widget.CIPPromotionList
type cipPromotionMsg widget.CIPPromotionDetail

// cipActionMsg is the answer to a write. Exactly one of Message and Error
// is set, so an outcome can never read as both a success and a failure.
type cipActionMsg struct {
	Message string
	Error   string
}

// agentUsagePoint is one CPU/memory reading for one agent, kept in a small
// ring buffer so the AGENTS tab can draw a live trend, the same way the
// server list keeps a history of metrics.Sample.
type agentUsagePoint struct {
	CPUPercent float64
	RSSBytes   int64
}
type Model struct {
	cfg                            config.Config
	collector                      collector.Collector
	samples                        []metrics.Sample
	history                        [][]metrics.Sample
	cursor                         int
	detail                         bool
	detailTab                      int
	detailScroll                   int
	rangeIndex                     int
	displayCPU                     []float64
	displayCores                   [][]float64
	streamBuffers                  [][]metrics.Sample
	desktopFrames                  map[int]string
	desktopFrameRaw                map[int][]byte
	desktopFrameSize               map[int]image.Point
	desktopClear                   string
	desktopErrors                  map[int]string
	desktopStreams                 map[int]*desktopclient.Stream
	ssh                            *sshPane
	sshText                        string
	devtoolCursor                  int
	devtoolStatus                  map[string]bool
	devtoolVersions                map[string]string
	devtoolConfirm                 string
	devtoolMessage                 string
	devtoolBusy                    bool
	orchestrator                   widget.OrchestratorSnapshot
	orchestratorSel                int
	agentUsage                     map[int][]agentUsagePoint
	agentTail                      map[int][]string
	agentTick                      int
	orchestratorModeMenu           bool
	orchestratorModeCursor         int
	orchestratorModeConfirm        bool
	orchestratorModeBusy           bool
	orchestratorModeMessage        string
	orchestratorModeMessageIsError bool
	cip                            widget.CIPSnapshot
	cipSel                         int
	cipOpenID                      int
	cipDetail                      widget.CIPRunDetail
	cipFrame                       int
	cipPromotions                  widget.CIPPromotionList
	cipOpenPromotionID             int
	cipStageSel                    int
	// cipSpecs caches the spec of each promotion. A spec is a snapshot
	// taken when the promotion started, so it never changes and one read
	// per promotion is enough.
	cipSpecs map[int]widget.CIPSpec
	// The CIP tab can write: it re-runs failed jobs, and it approves a
	// gated stage. Both change a real pipeline, so both arm first and act
	// only on a second, deliberate keypress.
	cipJobSel        int
	cipAction        string
	cipActionArmed   bool
	cipActionBusy    bool
	cipActionMessage string
	cipActionIsError bool
	cipReasonInput   bool
	cipReason        string
	width, height    int
	collecting       bool
	pending          int
	lastRefresh      time.Time
}

func New(cfg config.Config) Model {
	n := 0
	for _, server := range cfg.Servers {
		if server.AgentURL == "" {
			n++
		}
	}
	return Model{cfg: cfg, collector: collector.Collector{SSH: cfg.SSH}, samples: make([]metrics.Sample, len(cfg.Servers)), history: make([][]metrics.Sample, len(cfg.Servers)), displayCPU: make([]float64, len(cfg.Servers)), displayCores: make([][]float64, len(cfg.Servers)), streamBuffers: make([][]metrics.Sample, len(cfg.Servers)), desktopFrames: map[int]string{}, desktopFrameRaw: map[int][]byte{}, desktopFrameSize: map[int]image.Point{}, desktopErrors: map[int]string{}, desktopStreams: map[int]*desktopclient.Stream{}, devtoolStatus: map[string]bool{}, devtoolVersions: map[string]string{}, agentUsage: map[int][]agentUsagePoint{}, agentTail: map[int][]string{}, cipSpecs: map[int]widget.CIPSpec{}, collecting: n > 0, pending: n}
}
func (m Model) Init() tea.Cmd {
	cmds := append(m.collectAll(), m.nextFrame())
	if m.orchestratorWidget() != nil {
		cmds = append(cmds, m.fetchOrchestrator(), m.nextOrchestratorTick())
	}
	if m.cipWidget() != nil {
		cmds = append(cmds, m.fetchCIP(), m.fetchCIPPromotions(), m.nextCIPTick())
	}
	return tea.Batch(cmds...)
}

// nextOrchestratorTick schedules the next orchestrator refresh. It runs on
// the same cadence as the server metrics, independent of the SSH poll cycle
// so an all-agent inventory (no SSH servers) still refreshes the widget.
func (m Model) nextOrchestratorTick() tea.Cmd {
	return tea.Tick(orchestratorRefresh(m.agentsTabFocused()), func(t time.Time) tea.Msg {
		return orchestratorTickMsg(t)
	})
}

// agentsTabFocused reports whether the reader is looking at the AGENTS tab.
func (m Model) agentsTabFocused() bool {
	return m.detail && m.detailTab == 10
}

// orchestratorWidget returns the first configured "orchestrator" widget, or
// nil when the inventory does not have one. Use it only to fetch the
// snapshot. The AGENTS tab uses orchestratorWidgetFor, because the daemon
// runs on one server, not on every server.
func (m Model) orchestratorWidget() *config.Widget {
	for i := range m.cfg.Widgets {
		if m.cfg.Widgets[i].Type == "orchestrator" {
			return &m.cfg.Widgets[i]
		}
	}
	return nil
}

// orchestratorWidgetFor returns the orchestrator widget that runs on the
// server at index, or nil when that server does not run one. A widget with
// no host and no endpoint host belongs to no server, so the tab stays
// hidden instead of appearing on every server.
func (m Model) orchestratorWidgetFor(index int) *config.Widget {
	if index < 0 || index >= len(m.cfg.Servers) {
		return nil
	}
	address := m.cfg.Servers[index].Address
	for i := range m.cfg.Widgets {
		w := &m.cfg.Widgets[i]
		if w.Type != "orchestrator" {
			continue
		}
		if host := w.HostAddress(); host != "" && host == address {
			return w
		}
	}
	return nil
}

// tabCIP is the CIP tab. A detailTab value is the fixed identity of a tab.
// The body switch and the shortcut keys read it. It is NOT the position of
// the tab on screen, because the optional tabs show only on some servers.
const tabCIP = 11

// detailTabEntry is one tab of the detail view. Index is the tab identity.
// Label is the text on screen, which starts with the key that selects the
// tab. Keep the two apart: a list built from the configured tabs would give
// a tab the wrong identity if the position set it.
type detailTabEntry struct {
	Index int
	Label string
}

// detailTabs lists the tabs the server at index shows, in screen order. A
// tab the server does not have is absent, so a reader can never select a
// tab that shows nothing.
func (m Model) detailTabs(index int) []detailTabEntry {
	tabs := []detailTabEntry{
		{0, "1 CPU"}, {1, "2 MEMORY"}, {2, "3 STORAGE"}, {3, "4 NETWORK"},
		{4, "5 RUNNERS"}, {5, "6 PROCESSES"}, {6, "7 ACCEL"},
	}
	if m.desktopForServer(index) != nil {
		tabs = append(tabs, detailTabEntry{7, "8 DESKTOP"})
	}
	tabs = append(tabs, detailTabEntry{8, "9 SSH"}, detailTabEntry{9, "10 DEVTOOLS"})
	if m.orchestratorWidgetFor(index) != nil {
		tabs = append(tabs, detailTabEntry{10, "o AGENTS"})
	}
	if m.cipWidgetFor(index) != nil {
		tabs = append(tabs, detailTabEntry{tabCIP, "c CIP"})
	}
	return tabs
}

// nextDetailTab is the tab that follows current on the server at index. It
// wraps at the end of the list. It skips a tab this server does not have,
// so the cycle shows only the tabs the reader can see. A current tab that
// the server does not have starts the cycle again at the first tab.
func (m Model) nextDetailTab(index, current int) int {
	tabs := m.detailTabs(index)
	for i, entry := range tabs {
		if entry.Index == current {
			return tabs[(i+1)%len(tabs)].Index
		}
	}
	return tabs[0].Index
}

// clampDetailTab leaves a tab that the server under the cursor does not
// have. The cursor moves only in the overview, so the detail view can open
// on a server without the AGENTS tab while that tab is still selected.
func (m *Model) clampDetailTab() {
	if m.detailTab == 10 && m.orchestratorWidgetFor(m.cursor) == nil {
		m.detailTab = 0
		m.detailScroll = 0
	}
	if m.detailTab == tabCIP && m.cipWidgetFor(m.cursor) == nil {
		m.detailTab = 0
		m.detailScroll = 0
		// Drop the open run and promotion too. Work of another server's
		// daemon must not stay open behind the tab.
		m.cipOpenID, m.cipSel = 0, 0
		m.cipOpenPromotionID, m.cipStageSel = 0, 0
	}
}

// fetchOrchestrator does one authenticated read of the orchestrator status
// endpoint. It never issues an action against the orchestrator.
func (m Model) fetchOrchestrator() tea.Cmd {
	provider := m.orchestratorWidget()
	if provider == nil {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return orchestratorMsg(widget.OrchestratorSnapshot{Name: p.Name, At: time.Now(), Error: err.Error()})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return orchestratorMsg(widget.FetchOrchestrator(ctx, p, token))
	}
}

// cipWidget returns the first configured "cip" widget, or nil when the
// inventory does not have one. Use it only to fetch the snapshot. The CIP
// tab uses cipWidgetFor, because the daemon runs on one server, not on
// every server.
func (m Model) cipWidget() *config.Widget {
	for i := range m.cfg.Widgets {
		if m.cfg.Widgets[i].Type == "cip" {
			return &m.cfg.Widgets[i]
		}
	}
	return nil
}

// cipWidgetFor returns the cip widget that runs on the server at index, or
// nil when that server does not run one. A widget with no host and no
// endpoint host belongs to no server, so the tab stays hidden instead of
// appearing on every server.
func (m Model) cipWidgetFor(index int) *config.Widget {
	if index < 0 || index >= len(m.cfg.Servers) {
		return nil
	}
	address := m.cfg.Servers[index].Address
	for i := range m.cfg.Widgets {
		w := &m.cfg.Widgets[i]
		if w.Type != "cip" {
			continue
		}
		if host := w.HostAddress(); host != "" && host == address {
			return w
		}
	}
	return nil
}

// cipTabFocused reports whether the reader is looking at the CIP tab.
func (m Model) cipTabFocused() bool {
	return m.detail && m.detailTab == tabCIP
}

// fetchCIP does one authenticated read of the cip runs and storage
// endpoints. It never starts, stops, or cancels a pipeline run.
func (m Model) fetchCIP() tea.Cmd {
	provider := m.cipWidget()
	if provider == nil {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return cipMsg(widget.CIPSnapshot{Name: p.Name, At: time.Now(), Error: err.Error()})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return cipMsg(widget.FetchCIP(ctx, p, token))
	}
}

// cipRowKind tells a row of the CIP list apart. The list shows the
// promotions first, then the plain runs.
type cipRowKind int

const (
	cipRowPromotion cipRowKind = iota
	cipRowRun
)

// cipRow is one selectable row of the CIP list. Index points into the
// promotions or into the runs, depending on Kind.
type cipRow struct {
	Kind  cipRowKind
	Index int
}

// cipRows is the whole selectable list: every promotion, then every run. A
// daemon with no promotion gives exactly the run list it always gave.
func (m Model) cipRows() []cipRow {
	rows := make([]cipRow, 0, len(m.cipPromotions.Promotions)+len(m.cip.Runs))
	for i := range m.cipPromotions.Promotions {
		rows = append(rows, cipRow{Kind: cipRowPromotion, Index: i})
	}
	for i := range m.cip.Runs {
		rows = append(rows, cipRow{Kind: cipRowRun, Index: i})
	}
	return rows
}

// cipSelectedRow is the row under the selection bar.
func (m Model) cipSelectedRow() (cipRow, bool) {
	rows := m.cipRows()
	if m.cipSel < 0 || m.cipSel >= len(rows) {
		return cipRow{}, false
	}
	return rows[m.cipSel], true
}

// cipFocusPromotion is the promotion the flow draws: the open one, or the
// selected one while the list is shown.
func (m Model) cipFocusPromotion() (widget.CIPPromotionEntry, bool) {
	if m.cipOpenPromotionID != 0 {
		for _, entry := range m.cipPromotions.Promotions {
			if entry.Promotion.ID == m.cipOpenPromotionID {
				return entry, true
			}
		}
		return widget.CIPPromotionEntry{}, false
	}
	row, ok := m.cipSelectedRow()
	if !ok || row.Kind != cipRowPromotion || row.Index >= len(m.cipPromotions.Promotions) {
		return widget.CIPPromotionEntry{}, false
	}
	return m.cipPromotions.Promotions[row.Index], true
}

// cipSelectedStage is the stage under the selection bar inside an open
// promotion.
func (m Model) cipSelectedStage() (widget.CIPStage, bool) {
	entry, ok := m.cipFocusPromotion()
	if !ok || m.cipStageSel < 0 || m.cipStageSel >= len(entry.Stages) {
		return widget.CIPStage{}, false
	}
	return entry.Stages[m.cipStageSel], true
}

// fetchCIPPromotions reads the promotion list on the normal cadence. The
// list alone carries every stage state, so the flow needs no other read.
func (m Model) fetchCIPPromotions() tea.Cmd {
	provider := m.cipWidget()
	if provider == nil {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return cipPromotionsMsg(widget.CIPPromotionList{Name: p.Name, At: time.Now(), Error: err.Error()})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return cipPromotionsMsg(widget.FetchCIPPromotions(ctx, p, token))
	}
}

// fetchCIPPromotion reads one promotion, only to get its spec.
func (m Model) fetchCIPPromotion(id int) tea.Cmd {
	provider := m.cipWidget()
	if provider == nil || id == 0 {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return cipPromotionMsg(widget.CIPPromotionDetail{Name: p.Name, At: time.Now(),
				Promotion: widget.CIPPromotion{ID: id}, Error: err.Error()})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return cipPromotionMsg(widget.FetchCIPPromotion(ctx, p, token, id))
	}
}

// fetchCIPFocusSpec reads the spec of the promotion the reader points at,
// and only when the cache does not hold it. A spec never changes, so one
// read per promotion is enough for the whole session.
func (m Model) fetchCIPFocusSpec() tea.Cmd {
	entry, ok := m.cipFocusPromotion()
	if !ok || entry.Promotion.ID == 0 {
		return nil
	}
	if _, cached := m.cipSpecs[entry.Promotion.ID]; cached {
		return nil
	}
	return m.fetchCIPPromotion(entry.Promotion.ID)
}

// cipFocusSpec is the cached spec of the promotion in view. An empty spec
// is not a fault: the flow still lists every stage, without the edges.
func (m Model) cipFocusSpec() widget.CIPSpec {
	entry, ok := m.cipFocusPromotion()
	if !ok {
		return widget.CIPSpec{}
	}
	return m.cipSpecs[entry.Promotion.ID]
}

// The two writes the CIP tab can perform.
const (
	cipActionRerun   = "rerun"
	cipActionApprove = "approve"
)

// cipApprover is the name recorded against an approval. The daemon keeps an
// audit trail, so the name must never be empty.
func cipApprover() string {
	for _, name := range []string{os.Getenv("USER"), os.Getenv("LOGNAME")} {
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}
	return "servterm"
}

// cipRerunTarget is the run to re-run, and the one job to re-run inside it.
// An empty job means every failed job of the run.
//
// An open run re-runs the job under the cursor, because the reader points
// at one job. A run row or a stage row re-runs every failed job, because
// the reader points at the whole run.
func (m Model) cipRerunTarget() (int, string, bool) {
	if m.cipOpenID != 0 {
		job := ""
		if m.cipJobSel >= 0 && m.cipJobSel < len(m.cipDetail.Jobs) && m.cipDetail.Run.ID == m.cipOpenID {
			job = m.cipDetail.Jobs[m.cipJobSel].Name
		}
		return m.cipOpenID, job, true
	}
	if m.cipOpenPromotionID != 0 {
		stage, ok := m.cipSelectedStage()
		if !ok || !stage.HasRun() {
			return 0, "", false
		}
		return stage.RunID, "", true
	}
	run, ok := m.cipSelectedRun()
	if !ok {
		return 0, "", false
	}
	return run.ID, "", true
}

// cipApproveTarget is the stage an approval would act on: the stage under
// the cursor inside an open promotion, or the stage that waits at a gate in
// the promotion the reader points at.
func (m Model) cipApproveTarget() (int, widget.CIPStage, bool) {
	entry, ok := m.cipFocusPromotion()
	if !ok {
		return 0, widget.CIPStage{}, false
	}
	if m.cipOpenPromotionID != 0 {
		stage, ok := m.cipSelectedStage()
		if !ok {
			return 0, widget.CIPStage{}, false
		}
		return entry.Promotion.ID, stage, true
	}
	for _, stage := range entry.Stages {
		if stage.State == "gated" {
			return entry.Promotion.ID, stage, true
		}
	}
	return 0, widget.CIPStage{}, false
}

// rerunCIP asks the daemon to run the failed jobs again. This is a WRITE.
// Only an armed and confirmed keypress may call it.
func (m Model) rerunCIP(id int, job string) tea.Cmd {
	provider := m.cipWidget()
	if provider == nil || id == 0 {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return cipActionMsg{Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result := widget.RerunCIPRun(ctx, p, token, id, job)
		if result.Error != "" {
			return cipActionMsg{Error: result.Error}
		}
		return cipActionMsg{Message: result.Summary()}
	}
}

// approveCIPStage approves one gated stage. This is a WRITE, and it lets a
// release go ahead. Only an armed and confirmed keypress may call it.
func (m Model) approveCIPStage(promotionID int, stage, reason string) tea.Cmd {
	provider := m.cipWidget()
	if provider == nil || promotionID == 0 || stage == "" {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return cipActionMsg{Error: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result := widget.ApproveCIPStage(ctx, p, token, promotionID, stage, cipApprover(), reason)
		if result.Error != "" {
			return cipActionMsg{Error: result.Error}
		}
		return cipActionMsg{Message: "approved " + stage}
	}
}

// cipClearAction drops a part-finished action. The reader must never
// confirm one thing and act on another.
func (m *Model) cipClearAction() {
	m.cipAction, m.cipActionArmed = "", false
	m.cipReasonInput, m.cipReason = false, ""
}

// cipSelectedRun is the run under the selection bar in the run list.
func (m Model) cipSelectedRun() (widget.CIPRun, bool) {
	row, ok := m.cipSelectedRow()
	if !ok || row.Kind != cipRowRun || row.Index >= len(m.cip.Runs) {
		return widget.CIPRun{}, false
	}
	return m.cip.Runs[row.Index], true
}

// cipFocusRunID is the run the graph draws: the open run, or the selected
// run while the list is shown. It is 0 when the list has no run.
func (m Model) cipFocusRunID() int {
	if m.cipOpenID != 0 {
		return m.cipOpenID
	}
	if run, ok := m.cipSelectedRun(); ok {
		return run.ID
	}
	return 0
}

// fetchCIPRun does one authenticated read of the jobs of one run.
func (m Model) fetchCIPRun(id int) tea.Cmd {
	provider := m.cipWidget()
	if provider == nil || id == 0 {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return cipRunMsg(widget.CIPRunDetail{Name: p.Name, At: time.Now(),
				Run: widget.CIPRun{ID: id}, Error: err.Error()})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return cipRunMsg(widget.FetchCIPRun(ctx, p, token, id))
	}
}

// fetchCIPFocusRun reads the jobs of the run the reader points at, but only
// when the widget does not already hold them. This is what keeps the reads
// down while the reader moves the selection bar over old runs.
func (m Model) fetchCIPFocusRun() tea.Cmd {
	id := m.cipFocusRunID()
	if id == 0 || m.cipDetail.Run.ID == id {
		return nil
	}
	return m.fetchCIPRun(id)
}

// nextCIPRunTick schedules the next read of the open graph. It returns nil
// for a run that already ended, because a finished run never changes again
// and polling it would only waste reads.
func (m Model) nextCIPRunTick() tea.Cmd {
	id := m.cipFocusRunID()
	if id == 0 || m.cipDetail.Run.ID != id || m.cipDetail.Run.Status != "running" {
		return nil
	}
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return cipRunTickMsg(t) })
}

// cipRunIDAtRow reads the run id from one row of the screen. It renders the
// current view and reads the row the mouse hit, so the hit test always
// agrees with what the reader sees.
func (m Model) cipRunIDAtRow(y int) (int, bool) {
	if y < 0 {
		return 0, false
	}
	lines := strings.Split(m.View(), "\n")
	if y >= len(lines) {
		return 0, false
	}
	return cipRunIDAtLine(lines[y])
}

// nextCIPTick schedules the next cip refresh.
func (m Model) nextCIPTick() tea.Cmd {
	return tea.Tick(cipRefresh(m.cipTabFocused()), func(t time.Time) tea.Msg {
		return cipTickMsg(t)
	})
}

// cipRefresh reads often while the reader watches the tab, and rarely when
// the reader looks somewhere else, so an idle session stays cheap.
func cipRefresh(focused bool) time.Duration {
	if focused {
		return 2 * time.Second
	}
	return 20 * time.Second
}

// orchestratorToken reads the widget's token the same way for every
// orchestrator request, whether it reads status or sets the mode.
func orchestratorToken(p config.Widget) (string, error) {
	if p.TokenFile == "" {
		return os.Getenv(p.TokenEnv), nil
	}
	b, err := os.ReadFile(config.ExpandHome(p.TokenFile))
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// orchestratorModes lists the daemon's three run modes, in the wording the
// operator's own CLI uses, for the AGENTS tab's mode menu.
var orchestratorModes = []struct{ Value, Description string }{
	{"fast", "full fanout"},
	{"economy", "one third"},
	{"paused", "takes no new work — running tasks finish"},
}

// orchestratorModeIndex finds mode in orchestratorModes, defaulting to the
// first entry when the daemon reports a mode the menu does not list yet.
func orchestratorModeIndex(mode string) int {
	for i, opt := range orchestratorModes {
		if opt.Value == mode {
			return i
		}
	}
	return 0
}

// setOrchestratorMode sends the widget's one write: a request to change the
// daemon's run mode. Every mode only reduces work, so this can never raise
// fanout, spend, or point the daemon at a different repository.
func (m Model) setOrchestratorMode(mode string) tea.Cmd {
	provider := m.orchestratorWidget()
	if provider == nil {
		return nil
	}
	p := *provider
	return func() tea.Msg {
		token, err := orchestratorToken(p)
		if err != nil {
			return orchestratorModeMsg(widget.OrchestratorModeResult{Error: err.Error()})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return orchestratorModeMsg(widget.SetOrchestratorMode(ctx, p, token, mode))
	}
}
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
		if m.detail && m.detailTab == 7 {
			m.desktopClear = ""
			if raw := m.desktopFrameRaw[m.cursor]; len(raw) > 0 {
				m.desktopFrames[m.cursor] = m.renderDesktopFrame(m.cursor, raw)
			}
		}
	case tea.KeyMsg:
		if m.ssh != nil {
			switch msg.String() {
			case "ctrl+x", "cmd+x", "super+x":
				m.ssh.close()
				m.ssh = nil
				m.sshText = ""
				return m, nil
			case "tab":
				m.detailTab = m.nextDetailTab(m.cursor, m.detailTab)
				if m.detailTab == 7 {
					m.desktopClear = ""
				}
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
			if msg.String() == "p" {
				return m, m.sendDesktopClipboard(m.cursor)
			}
			return m, m.sendDesktopKey(m.cursor, msg.String())
		}
		// The CIP tab drives its own run list: up and down move the
		// selection bar, enter opens the run below the graph, and esc
		// closes it again. Every other key falls through to the general
		// handler, so esc still leaves the detail view when no run is open.
		// The reason prompt takes every key while it is open. Without
		// that, typing a reason would drive the list under the reader.
		if m.detail && m.detailTab == tabCIP && m.cipReasonInput {
			switch msg.Type {
			case tea.KeyEsc:
				m.cipClearAction()
				return m, nil
			case tea.KeyBackspace:
				if r := []rune(m.cipReason); len(r) > 0 {
					m.cipReason = string(r[:len(r)-1])
				}
				return m, nil
			case tea.KeyEnter:
				if m.cipActionBusy {
					return m, nil
				}
				// The first enter arms the approval. The second sends it.
				if !m.cipActionArmed {
					m.cipActionArmed = true
					return m, nil
				}
				id, stage, ok := m.cipApproveTarget()
				if !ok {
					m.cipClearAction()
					return m, nil
				}
				m.cipActionArmed, m.cipActionBusy = false, true
				m.cipActionMessage = ""
				return m, m.approveCIPStage(id, stage.Stage, m.cipReason)
			case tea.KeyRunes, tea.KeySpace:
				// A new character changes the request, so it must cancel a
				// confirmation the reader already gave.
				m.cipActionArmed = false
				if msg.Type == tea.KeySpace {
					m.cipReason += " "
				} else {
					m.cipReason += string(msg.Runes)
				}
				return m, nil
			}
			return m, nil
		}
		if m.detail && m.detailTab == tabCIP && m.cipWidgetFor(m.cursor) != nil {
			switch msg.String() {
			case "r":
				// Re-run the failed work. This is a WRITE, so the first
				// press only arms it.
				if m.cipActionBusy {
					return m, nil
				}
				id, job, ok := m.cipRerunTarget()
				if !ok {
					m.cipClearAction()
					m.cipActionIsError = true
					m.cipActionMessage = "select a run or a stage that has a run"
					return m, nil
				}
				if m.cipActionArmed && m.cipAction == cipActionRerun {
					m.cipActionArmed, m.cipActionBusy = false, true
					m.cipActionMessage = ""
					return m, m.rerunCIP(id, job)
				}
				m.cipAction, m.cipActionArmed = cipActionRerun, true
				m.cipActionIsError, m.cipActionMessage = false, ""
				return m, nil
			case "a":
				// Approve a gated stage. This lets a release go ahead, so
				// it asks for a reason and then for a confirmation.
				if m.cipActionBusy {
					return m, nil
				}
				_, stage, ok := m.cipApproveTarget()
				if !ok || stage.State != "gated" {
					m.cipClearAction()
					m.cipActionIsError = true
					m.cipActionMessage = "only a gated stage can be approved"
					return m, nil
				}
				m.cipAction, m.cipReasonInput = cipActionApprove, true
				m.cipActionArmed, m.cipReason = false, ""
				m.cipActionIsError, m.cipActionMessage = false, ""
				return m, nil
			case "up", "k":
				// A move points the reader at something else, so it drops
				// any armed action. A confirmation must never apply to a
				// target the reader left.
				m.cipClearAction()
				if m.cipOpenID != 0 {
					if m.cipJobSel > 0 {
						m.cipJobSel--
					}
					return m, nil
				}
				if m.cipOpenPromotionID != 0 {
					if m.cipStageSel > 0 {
						m.cipStageSel--
					}
					return m, nil
				}
				if m.cipSel > 0 {
					m.cipSel--
					m.detailScroll = 0
					return m, tea.Batch(m.fetchCIPFocusRun(), m.fetchCIPFocusSpec())
				}
				return m, nil
			case "down", "j":
				m.cipClearAction()
				if m.cipOpenID != 0 {
					if m.cipJobSel < len(m.cipDetail.Jobs)-1 {
						m.cipJobSel++
					}
					return m, nil
				}
				if m.cipOpenPromotionID != 0 {
					if entry, ok := m.cipFocusPromotion(); ok && m.cipStageSel < len(entry.Stages)-1 {
						m.cipStageSel++
					}
					return m, nil
				}
				if m.cipSel < len(m.cipRows())-1 {
					m.cipSel++
					m.detailScroll = 0
					return m, tea.Batch(m.fetchCIPFocusRun(), m.fetchCIPFocusSpec())
				}
				return m, nil
			case "enter":
				if m.cipOpenID != 0 {
					return m, nil
				}
				// Inside a promotion, enter opens the run of the stage. A
				// stage that never ran has no run to open.
				if m.cipOpenPromotionID != 0 {
					stage, ok := m.cipSelectedStage()
					if !ok || !stage.HasRun() {
						return m, nil
					}
					m.cipOpenID = stage.RunID
					m.detailScroll = 0
					return m, tea.Batch(m.fetchCIPFocusRun(), m.nextCIPRunTick())
				}
				row, ok := m.cipSelectedRow()
				if !ok {
					return m, nil
				}
				if row.Kind == cipRowPromotion {
					m.cipOpenPromotionID = m.cipPromotions.Promotions[row.Index].Promotion.ID
					m.cipStageSel, m.detailScroll = 0, 0
					return m, m.fetchCIPFocusSpec()
				}
				run, ok := m.cipSelectedRun()
				if !ok {
					return m, nil
				}
				m.cipOpenID = run.ID
				m.detailScroll = 0
				return m, tea.Batch(m.fetchCIPFocusRun(), m.nextCIPRunTick())
			case "esc":
				// Escape drops a part-finished action first, so a reader
				// can back out of a write without leaving the view.
				if m.cipAction != "" || m.cipActionArmed {
					m.cipClearAction()
					return m, nil
				}
				// Then it walks back one level at a time: the run, then the
				// promotion, then the detail view itself.
				if m.cipOpenID != 0 {
					m.cipOpenID = 0
					m.detailScroll = 0
					return m, m.fetchCIPFocusRun()
				}
				if m.cipOpenPromotionID != 0 {
					m.cipOpenPromotionID, m.cipStageSel = 0, 0
					m.detailScroll = 0
					return m, m.fetchCIPFocusRun()
				}
			}
		}
		// The mode menu is a small modal on top of the AGENTS tab: while it
		// is open, up/down/enter/esc drive the menu, not the agent list.
		if m.detail && m.detailTab == 10 && m.orchestratorWidgetFor(m.cursor) != nil && m.orchestratorModeMenu {
			switch msg.String() {
			case "up", "k":
				if m.orchestratorModeCursor > 0 {
					m.orchestratorModeCursor--
				}
				m.orchestratorModeConfirm = false
				return m, nil
			case "down", "j":
				if m.orchestratorModeCursor < len(orchestratorModes)-1 {
					m.orchestratorModeCursor++
				}
				m.orchestratorModeConfirm = false
				return m, nil
			case "esc":
				m.orchestratorModeMenu = false
				m.orchestratorModeConfirm = false
				return m, nil
			case "enter":
				if m.orchestratorModeBusy {
					return m, nil
				}
				if m.orchestratorModeConfirm {
					m.orchestratorModeConfirm = false
					m.orchestratorModeBusy = true
					m.orchestratorModeMessage = ""
					return m, m.setOrchestratorMode(orchestratorModes[m.orchestratorModeCursor].Value)
				}
				m.orchestratorModeConfirm = true
				return m, nil
			}
			return m, nil
		}
		if m.detail && m.detailTab == 10 && m.orchestratorWidgetFor(m.cursor) != nil {
			switch msg.String() {
			case "up", "k":
				if m.orchestratorSel > 0 {
					m.orchestratorSel--
				}
				return m, nil
			case "down", "j":
				if m.orchestratorSel < len(m.orchestrator.Agents)-1 {
					m.orchestratorSel++
				}
				return m, nil
			case "p":
				if a := m.orchestratorSelected(); a != nil {
					if url := orchestratorPRURL(m.orchestrator.Repo, a.PRNumber); url != "" {
						return m, openURL(url)
					}
				}
				return m, nil
			case "i":
				if a := m.orchestratorSelected(); a != nil {
					return m, openURL(orchestratorIssueURL(m.orchestrator.Repo, a.Issue))
				}
				return m, nil
			case "m":
				m.orchestratorModeMenu = true
				m.orchestratorModeCursor = orchestratorModeIndex(m.orchestrator.Mode)
				m.orchestratorModeConfirm = false
				m.orchestratorModeMessage = ""
				return m, nil
			}
		}
		if m.detail && m.detailTab == 7 && leavesDesktopTab(msg.String()) {
			m.desktopClear = clearDesktopImage()
			m.desktopFrameRaw[m.cursor] = nil
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
			m.clampDetailTab()
		case "tab":
			if m.detail {
				if m.detailTab == 7 {
					m.desktopClear = clearDesktopImage()
				}
				m.detailTab = m.nextDetailTab(m.cursor, m.detailTab)
				if m.detailTab == 7 {
					m.desktopClear = ""
				}
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
				m.desktopClear = ""
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
		case "o":
			if m.detail && m.orchestratorWidgetFor(m.cursor) != nil {
				m.detailTab = 10
				m.detailScroll = 0
			}
		case "c":
			// The desktop tab owned this key first. It keeps it there, so
			// the new tab takes no shortcut away from the reader.
			if m.detail && m.detailTab == 7 {
				return m, openDesktop(m.desktopForServer(m.cursor))
			}
			if m.detail && m.cipWidgetFor(m.cursor) != nil {
				m.detailTab = tabCIP
				m.detailScroll = 0
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
				m.desktopClear = clearDesktopImage()
				m.desktopFrames[m.cursor] = m.desktopClear
				m.desktopFrameRaw[m.cursor] = nil
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
			if m.detail && m.detailTab == 7 {
				m.desktopClear = clearDesktopImage()
			}
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
		m.desktopFrameRaw[msg.Index] = append(m.desktopFrameRaw[msg.Index][:0], msg.Frame...)
		m.rememberDesktopFrameSize(msg.Index, msg.Frame)
		m.desktopFrames[msg.Index] = m.renderDesktopFrame(msg.Index, msg.Frame)
		if stream := m.desktopStreams[msg.Index]; stream != nil {
			if text := stream.Clipboard(); text != "" {
				setLocalClipboard(text)
			}
		}
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
		m.desktopFrameRaw[msg.Index] = append(m.desktopFrameRaw[msg.Index][:0], msg.Frame...)
		m.rememberDesktopFrameSize(msg.Index, msg.Frame)
		m.desktopFrames[msg.Index] = m.renderDesktopFrame(msg.Index, msg.Frame)
		if stream := m.desktopStreams[msg.Index]; stream != nil {
			if text := stream.Clipboard(); text != "" {
				setLocalClipboard(text)
			}
		}
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
		// A click on a run row selects that run and opens it in one step.
		// Two clicks cannot work here: opening a run redraws the graph
		// above the list, so the rows move under the pointer between the
		// two clicks. The esc key goes back.
		if m.detail && m.detailTab == tabCIP && m.cipWidgetFor(m.cursor) != nil &&
			msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			lines := strings.Split(m.View(), "\n")
			if msg.Y < 0 || msg.Y >= len(lines) {
				return m, nil
			}
			line := lines[msg.Y]
			// A promotion row opens the promotion and its stage flow.
			if id, ok := cipPromotionIDAtLine(line); ok {
				for i, entry := range m.cipPromotions.Promotions {
					if entry.Promotion.ID != id {
						continue
					}
					m.cipSel, m.cipOpenPromotionID = i, id
					m.cipStageSel, m.detailScroll = 0, 0
					return m, m.fetchCIPFocusSpec()
				}
				return m, nil
			}
			id, ok := cipRunIDAtLine(line)
			if !ok {
				return m, nil
			}
			rows := m.cipRows()
			for i, row := range rows {
				if row.Kind != cipRowRun || m.cip.Runs[row.Index].ID != id {
					continue
				}
				m.cipSel, m.cipOpenID = i, id
				m.detailScroll = 0
				return m, tea.Batch(m.fetchCIPFocusRun(), m.nextCIPRunTick())
			}
			return m, nil
		}
		if m.detail && m.detailTab == 7 && msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonRight) {
			return m, m.sendDesktopClick(m.cursor, msg.X, msg.Y, msg.Button == tea.MouseButtonRight)
		}
	case desktopRefreshMsg:
		if m.detail && m.detailTab == 7 && msg.Index == m.cursor {
			return m, m.loadDesktop(msg.Index)
		}
	case frameMsg:
		// One step per frame drives the pipeline spinner. The counter lives
		// in the model, so a test can pin the animation to an exact frame.
		m.cipFrame++
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
	case orchestratorMsg:
		m.orchestrator = widget.OrchestratorSnapshot(msg)
		if m.orchestratorSel >= len(m.orchestrator.Agents) {
			m.orchestratorSel = max(0, len(m.orchestrator.Agents)-1)
		}
		// Append one CPU/RSS point per live agent, capped at HistorySize
		// like the rest of the UI, then drop the series for any issue that
		// is no longer live so the map cannot grow across a long session.
		live := map[int]bool{}
		for _, a := range m.orchestrator.Agents {
			live[a.Issue] = true
			pts := append(m.agentUsage[a.Issue], agentUsagePoint{CPUPercent: a.CPUPercent, RSSBytes: a.RSSBytes})
			if len(pts) > m.cfg.HistorySize {
				pts = pts[len(pts)-m.cfg.HistorySize:]
			}
			m.agentUsage[a.Issue] = pts
			if a.LastActivity != nil {
				m.agentTail[a.Issue] = appendTail(m.agentTail[a.Issue], *a.LastActivity, agentTailSize)
			}
		}
		// One step per snapshot, so the spinner turns at the refresh rate.
		m.agentTick++
		for issue := range m.agentUsage {
			if !live[issue] {
				delete(m.agentUsage, issue)
			}
		}
		return m, nil
	case orchestratorModeMsg:
		m.orchestratorModeBusy = false
		result := widget.OrchestratorModeResult(msg)
		if !result.OK {
			m.orchestratorModeMessageIsError = true
			if result.Error != "" {
				m.orchestratorModeMessage = result.Error
			} else {
				m.orchestratorModeMessage = "mode change failed"
			}
			return m, nil
		}
		// Close the menu and refresh from the daemon rather than trusting
		// the local write, so the header shows the mode the daemon is
		// actually running, not just the one this request asked for.
		m.orchestratorModeMenu = false
		m.orchestratorModeMessageIsError = false
		m.orchestratorModeMessage = "mode set to " + result.Mode
		return m, m.fetchOrchestrator()
	case orchestratorTickMsg:
		return m, tea.Batch(m.fetchOrchestrator(), m.nextOrchestratorTick())
	case cipMsg:
		m.cip = widget.CIPSnapshot(msg)
		// The list can shrink between two reads, so keep the selection bar
		// on a real run instead of past the end of the list.
		if m.cipSel >= len(m.cip.Runs) {
			m.cipSel = max(0, len(m.cip.Runs)-1)
		}
		return m, m.fetchCIPFocusRun()
	case cipTickMsg:
		return m, tea.Batch(m.fetchCIP(), m.fetchCIPPromotions(), m.nextCIPTick())
	case cipPromotionsMsg:
		m.cipPromotions = widget.CIPPromotionList(msg)
		if rows := len(m.cipRows()); m.cipSel >= rows {
			m.cipSel = max(0, rows-1)
		}
		// Read the spec of the promotion in view only, and only once. The
		// list itself already carries every stage state.
		return m, m.fetchCIPFocusSpec()
	case cipPromotionMsg:
		detail := widget.CIPPromotionDetail(msg)
		// Cache only a good answer. Caching a failed read would keep the
		// flow without edges for the rest of the session.
		if detail.Error == "" && detail.Promotion.ID != 0 {
			if m.cipSpecs == nil {
				m.cipSpecs = map[int]widget.CIPSpec{}
			}
			m.cipSpecs[detail.Promotion.ID] = detail.Spec
		}
		return m, nil
	case cipActionMsg:
		m.cipActionBusy, m.cipActionArmed = false, false
		m.cipReasonInput, m.cipReason = false, ""
		m.cipAction = ""
		if msg.Error != "" {
			m.cipActionIsError, m.cipActionMessage = true, msg.Error
			return m, nil
		}
		m.cipActionIsError, m.cipActionMessage = false, msg.Message
		// Read the new state at once, so the reader sees what changed
		// instead of waiting for the next tick.
		return m, tea.Batch(m.fetchCIP(), m.fetchCIPPromotions(), m.fetchCIPRun(m.cipFocusRunID()))
	case cipRunMsg:
		m.cipDetail = widget.CIPRunDetail(msg)
		if m.cipJobSel >= len(m.cipDetail.Jobs) {
			m.cipJobSel = max(0, len(m.cipDetail.Jobs)-1)
		}
		return m, m.nextCIPRunTick()
	case cipRunTickMsg:
		return m, m.fetchCIPRun(m.cipFocusRunID())
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
		// "p" belongs to the remote desktop tab only. Advertising it
		// everywhere made it look like it clashed with the agents tab,
		// where "p" opens the selected pull request.
		clip := ""
		if m.detailTab == 7 {
			clip = "  p clipboard"
		}
		help = "  tab / 1..9 widgets  s SSH  c connect desktop" + clip + "  [ / ] history  j/k scroll  esc overview  q quit   LIVE -1.0s • 10fps"
		if m.ssh != nil {
			help = "  SSH session active  cmd+x close session  ctrl-c remote"
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
	tabs := "  "
	// Compare the tab identity, not the position. The optional tabs shift
	// every later position, so a position test would underline the wrong
	// tab whenever a server has no desktop.
	for _, entry := range m.detailTabs(m.cursor) {
		tabs += tabLabel(entry.Label, m.detailTab == entry.Index) + "  "
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
	case 10:
		body = m.orchestratorView()
	case tabCIP:
		body = m.cipView()
	}
	if m.detail && m.detailTab != 7 && m.desktopClear != "" {
		body = m.desktopClear + body
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
	contentWidth := max(60, m.width-4)
	descWidth := max(12, contentWidth-2-14-11-24-3)
	b.WriteString("  DEV TOOLS\n\n")
	b.WriteString(dimStyle.Render(truncate(fmt.Sprintf("  %-14s %-11s %-24s %s", "TOOL", "STATUS", "VERSION", "DESCRIPTION"), contentWidth)) + "\n")
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
		line := fmt.Sprintf("  %-14s %-11s %-24s %s", t.ID, state, truncate(version, 24), truncate(t.Description, descWidth))
		if i == m.devtoolCursor {
			b.WriteString(lipgloss.NewStyle().Background(panel).Bold(true).Foreground(cyan).Render(pad(line, contentWidth)) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %-14s ", t.ID))
			b.WriteString(stateStyle.Render(fmt.Sprintf("%-11s", state)))
			b.WriteString(dimStyle.Render(fmt.Sprintf(" %-24s", truncate(version, 24))))
			b.WriteString(" " + truncate(t.Description, descWidth) + "\n")
		}
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

// orchestratorView renders the agent orchestrator status: a header line,
// the live agent list with the selected agent highlighted, and the full
// detail of the selected agent below the list.
// orchestratorSelected returns the currently highlighted agent, or nil when
// the list is empty or the cursor is out of range.
func (m Model) orchestratorSelected() *widget.OrchestratorAgent {
	if m.orchestratorSel < 0 || m.orchestratorSel >= len(m.orchestrator.Agents) {
		return nil
	}
	return &m.orchestrator.Agents[m.orchestratorSel]
}

// orchestratorIssueURL builds the GitHub issue link. Every agent has an
// issue number, so this link is always available once repo is known.
func orchestratorIssueURL(repo string, issue int) string {
	if repo == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/issues/%d", repo, issue)
}

// orchestratorPRURL builds the GitHub pull request link. It returns "" when
// the agent has not opened a pull request yet, so openURL does nothing
// rather than open a half-built link.
func orchestratorPRURL(repo string, pr *int) string {
	if repo == "" || pr == nil {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/pull/%d", repo, *pr)
}

// orchestratorHistoryView lists the tasks that already left the live list.
//
// A finished or blocked agent used to vanish from the widget entirely, which
// made a failure invisible: the only trace was a GitHub comment. The daemon
// keeps these records in its state file, so they survive a restart and stay
// readable long after the worker died.
func orchestratorHistoryView(snap widget.OrchestratorSnapshot, width int) string {
	if len(snap.Recent) == 0 {
		return ""
	}
	out := "  " + titleStyle.Render("HISTORY") + "  " + dimStyle.Render("tasks an agent already handled") + "\n\n"
	out += dimStyle.Render(truncate(fmt.Sprintf("  %-6s %-10s %-7s %s", "ISSUE", "STATE", "PR", "TITLE / REASON"), width)) + "\n"
	for _, r := range snap.Recent {
		style := dimStyle
		switch r.State {
		case "done", "merged":
			style = okStyle
		case "blocked", "failed":
			style = errStyle
		}
		pr := "—"
		if r.PRNumber != nil {
			pr = fmt.Sprintf("#%d", *r.PRNumber)
		}
		out += fmt.Sprintf("  #%-5d %s %-7s %s\n",
			r.Issue, style.Render(pad(r.State, 10)), pr, truncate(or(r.Title, "title pending"), 44))
		// The reason a task stopped is the point of the history, so it gets
		// its own line rather than being cut off at the end of the row.
		if r.LastError != "" {
			out += "         " + dimStyle.Render(truncate(r.LastError, max(20, width-10))) + "\n"
		}
	}
	return out
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerFrame turns a tick counter into one frame of the spinner. A running
// agent that prints a still row for minutes reads as a hang, so the motion is
// the point.
func spinnerFrame(tick int) string {
	n := len(spinnerFrames)
	return spinnerFrames[((tick%n)+n)%n]
}

const agentTailSize = 6

// appendTail records one activity line for an agent, newest last.
//
// The daemon reports the CURRENT line on every poll, so the same line arrives
// many times while one command runs. Repeating it would fill the tail with a
// single command, so only a change is kept.
func appendTail(tail []string, line string, cap int) []string {
	if line == "" {
		return tail
	}
	if len(tail) > 0 && tail[len(tail)-1] == line {
		return tail
	}
	out := append(tail, line)
	if len(out) > cap {
		out = out[len(out)-cap:]
	}
	return out
}

// orchestratorRefresh is how often the widget re-reads the daemon.
//
// A person watching the tab wants the tail to move; a person on another tab
// wants their bandwidth and the daemon's CPU back. So the rate follows the
// focus rather than being one compromise between the two.
// cipView draws the CIP tab: the daemon state, the newest pipeline runs,
// and the disk each pipeline occupies. A failed read shows the reason and
// nothing else, because an empty panel reads as a healthy daemon.
func (m Model) cipView() string {
	snap := m.cip
	if snap.Error != "" {
		return "  " + titleStyle.Render("CIP PIPELINES") + "\n\n  " + errStyle.Render("unavailable: "+snap.Error) + "\n"
	}
	health := okStyle.Bold(true).Render("HEALTHY")
	if !snap.Healthy {
		health = errStyle.Bold(true).Render("DEGRADED")
	}
	summary := fmt.Sprintf("%s  %s  •  %s running  •  %s failed  •  %d success",
		titleStyle.Render("CIP PIPELINES"), health,
		warnStyle.Render(fmt.Sprintf("%d", snap.Running)),
		errStyle.Render(fmt.Sprintf("%d", snap.Failed)), snap.Succeeded)
	out := lipgloss.NewStyle().MarginLeft(2).Border(lipgloss.RoundedBorder()).BorderForeground(panel).Padding(0, 1).Render(summary) + "\n\n"
	contentWidth := max(70, m.width-4)
	// Measure every duration against the time of the snapshot, so each row
	// on one screen shares one clock.
	now := snap.At
	if now.IsZero() {
		now = time.Now()
	}
	// The graph is pinned at the top. It stays there whether the reader
	// browses the list or looks at one run, so the pipeline never leaves
	// the screen.
	out += m.cipGraphPane(contentWidth) + "\n"
	out += m.cipActionPane(contentWidth)
	if m.cipOpenID != 0 {
		return out + m.cipRunPane(contentWidth)
	}
	if m.cipOpenPromotionID != 0 {
		return out + m.cipStagePane(contentWidth)
	}
	if len(m.cipPromotions.Promotions) > 0 {
		out += "  " + titleStyle.Render("PROMOTIONS") + "  " + dimStyle.Render("↑↓ select  enter open  a approve a gated stage  click opens one") + "\n\n"
		out += dimStyle.Render(truncate(fmt.Sprintf("   %-5s %-24s %-8s %-18s %s", "ID", "REPO", "STATE", "BRANCH", "STAGES"), contentWidth)) + "\n"
		for i, entry := range m.cipPromotions.Promotions {
			row := truncate(" "+m.cipSelectMark(i)+cipPromotionLine(entry), contentWidth)
			out += cipListRowStyle(i == m.cipSel, entry.Promotion.State).Render(row) + "\n"
		}
		out += "\n"
	}
	if len(snap.Runs) == 0 {
		out += dimStyle.Render("  No pipeline run exists yet.") + "\n\n"
	} else {
		out += "  " + titleStyle.Render("RECENT RUNS") + "  " + dimStyle.Render("↑↓ select  enter open  r re-run failed jobs  click opens a run") + "\n\n"
		out += dimStyle.Render(truncate(fmt.Sprintf("   %-5s %-24s %-8s %-14s %s", "RUN", "REPO", "STATUS", "TOOK", "BRANCH"), contentWidth)) + "\n"
		// The runs follow the promotions in one selectable list, so a run
		// row sits at its own index plus the number of promotions.
		offset := len(m.cipPromotions.Promotions)
		for i, run := range snap.Runs {
			// Style the whole row only after it is cut to width, because
			// the style codes would otherwise count against the width.
			row := truncate(" "+m.cipSelectMark(offset+i)+run.Line(now), contentWidth)
			out += cipListRowStyle(offset+i == m.cipSel, run.Status).Render(row) + "\n"
		}
		out += "\n"
	}
	out += "  " + titleStyle.Render("STORAGE") + "\n\n"
	for _, line := range snap.StorageLines() {
		out += truncate("  "+line, contentWidth) + "\n"
	}
	return out
}

// cipActionPane shows the state of a write: what it will do, how to
// confirm it, and what the daemon answered. An outcome is never silent,
// because a quiet no-op is the failure this widget must avoid.
func (m Model) cipActionPane(width int) string {
	out := ""
	switch {
	case m.cipActionBusy:
		out += "  " + warnStyle.Render("◌ working…") + "\n"
	case m.cipReasonInput:
		id, stage, ok := m.cipApproveTarget()
		name := stage.Stage
		if !ok {
			name = "this stage"
		}
		out += "  " + titleStyle.Render(fmt.Sprintf("APPROVE %s of P%d", name, id)) + "\n"
		out += "  " + dimStyle.Render("reason: ") + m.cipReason + lipgloss.NewStyle().Foreground(cyan).Render("▏") + "\n"
		if m.cipActionArmed {
			out += "  " + warnStyle.Render("Press Enter again to approve. Esc cancels.") + "\n"
		} else {
			out += "  " + dimStyle.Render("Enter arms the approval. Esc cancels.") + "\n"
		}
	case m.cipActionArmed && m.cipAction == cipActionRerun:
		id, job, ok := m.cipRerunTarget()
		what := fmt.Sprintf("every failed job of run #%d", id)
		if job != "" {
			what = fmt.Sprintf("the job %s of run #%d", job, id)
		}
		if ok {
			out += "  " + warnStyle.Render("Re-run "+what+"? Press r again. Esc cancels.") + "\n"
		}
	}
	if m.cipActionMessage != "" {
		style := okStyle
		if m.cipActionIsError {
			style = errStyle
		}
		out += "  " + style.Render(clampLine(m.cipActionMessage, width-2)) + "\n"
	}
	if out != "" {
		out += "\n"
	}
	return out
}

// cipSelectMark is the pointer in front of the selected row. The bar must
// be visible without color, so the row carries a mark as well as a style.
func (m Model) cipSelectMark(index int) string {
	return m.cipSelectMarkAt(index, m.cipSel)
}

// cipSelectMarkAt marks the selected row of any of the CIP lists.
func (m Model) cipSelectMarkAt(index, selected int) string {
	if index == selected {
		return "▸ "
	}
	return "  "
}

// cipListRowStyle colors one row of the CIP list. The selected row wins
// over the state color, so the reader never loses the selection bar.
func cipListRowStyle(selected bool, state string) lipgloss.Style {
	if selected {
		return lipgloss.NewStyle().Bold(true).Foreground(cyan)
	}
	switch state {
	case "failed":
		return errStyle
	case "running", "active":
		return warnStyle
	default:
		return lipgloss.NewStyle()
	}
}

// cipPromotionLine is one row of the promotion list. It starts with "P"
// and the id, which is how a mouse click finds the promotion again.
func cipPromotionLine(entry widget.CIPPromotionEntry) string {
	p := entry.Promotion
	where := p.Branch
	if sha := p.ShortSHA(); sha != "" {
		where += "@" + sha
	}
	// Name the stage that needs attention, because that is why a reader
	// looks at a promotion at all.
	note := fmt.Sprintf("%d stages", len(entry.Stages))
	for _, stage := range entry.Stages {
		if stage.State == "gated" {
			note = cipGatedMark + " " + stage.Stage
			break
		}
		if stage.State == "running" {
			note = "▶ " + stage.Stage
			break
		}
		if stage.State == "failed" {
			note = "✗ " + stage.Stage
			break
		}
	}
	return fmt.Sprintf("P%-4d %-24s %-8s %-18s %s", p.ID, p.Repo, p.State, where, note)
}

// cipStagePane replaces the list with the stages of the open promotion.
func (m Model) cipStagePane(width int) string {
	out := "  " + titleStyle.Render(fmt.Sprintf("STAGES OF P%d", m.cipOpenPromotionID)) + "  " +
		dimStyle.Render("↑↓ select  enter opens the run  a approve  r re-run  esc back") + "\n\n"
	entry, ok := m.cipFocusPromotion()
	if !ok {
		return out + dimStyle.Render("  This promotion is gone.") + "\n"
	}
	now := m.cipPromotions.At
	if now.IsZero() {
		now = time.Now()
	}
	spec := m.cipFocusSpec()
	out += dimStyle.Render(truncate(fmt.Sprintf("   %-16s %-11s %-8s %s", "STAGE", "STATE", "RUN", "WAITING FOR"), width)) + "\n"
	for i, stage := range entry.Stages {
		run := "—"
		if stage.HasRun() {
			run = fmt.Sprintf("#%d", stage.RunID)
		}
		row := fmt.Sprintf(" %s%-16s %-11s %-8s %s", m.cipSelectMarkAt(i, m.cipStageSel),
			truncate(stage.Stage, 16), stage.State, run, cipStageNote(stage, spec, now))
		out += cipListRowStyle(i == m.cipStageSel, stage.State).Render(clampLine(strings.TrimRight(row, " "), width)) + "\n"
	}
	return out
}

// cipGraphPane draws the pinned pipeline graph for the run the reader
// points at. It waits rather than draw the jobs of another run, because a
// graph under the wrong heading is worse than a short wait.
func (m Model) cipGraphPane(width int) string {
	// A failed promotion read shows as a banner above the pane. It must
	// never hide the graph, and it must never pass unseen.
	banner := ""
	if m.cipPromotions.Error != "" {
		banner = "  " + errStyle.Render("promotions unavailable: "+m.cipPromotions.Error) + "\n\n"
	}
	// While no run is open, a promotion in view draws the stage flow. That
	// flow is the shape of the pipeline; the job graph is one stage of it.
	if m.cipOpenID == 0 {
		if entry, ok := m.cipFocusPromotion(); ok {
			now := m.cipPromotions.At
			if now.IsZero() {
				now = time.Now()
			}
			return banner + cipStageFlowView(entry, m.cipFocusSpec(), now, m.cipFrame, width)
		}
	}
	return banner + m.cipJobGraphPane(width)
}

// cipJobGraphPane draws the job graph of the run the reader points at.
func (m Model) cipJobGraphPane(width int) string {
	id := m.cipFocusRunID()
	if id == 0 {
		return "  " + titleStyle.Render("PIPELINE") + "\n\n" + dimStyle.Render("  Select a run to see its pipeline.") + "\n"
	}
	if m.cipDetail.Error == "" && m.cipDetail.Run.ID != id {
		return "  " + titleStyle.Render("PIPELINE") + "\n\n" + dimStyle.Render(fmt.Sprintf("  Loading the pipeline of run #%d…", id)) + "\n"
	}
	now := m.cipDetail.At
	if now.IsZero() {
		now = time.Now()
	}
	return cipGraphView(m.cipDetail, now, m.cipFrame, width)
}

// cipRunPane replaces the run list with the jobs of the open run.
func (m Model) cipRunPane(width int) string {
	detail := m.cipDetail
	out := "  " + titleStyle.Render(fmt.Sprintf("RUN #%d", m.cipOpenID)) + "  " + dimStyle.Render("↑↓ select a job  r re-runs it  esc back") + "\n\n"
	if detail.Error != "" {
		return out + "  " + errStyle.Render("unavailable: "+detail.Error) + "\n"
	}
	if detail.Run.ID != m.cipOpenID {
		return out + dimStyle.Render("  Loading…") + "\n"
	}
	now := detail.At
	if now.IsZero() {
		now = time.Now()
	}
	static := detail.Run.Status != "running"
	if len(detail.Jobs) == 0 {
		return out + dimStyle.Render("  No job exists for this run yet.") + "\n"
	}
	out += dimStyle.Render(truncate(fmt.Sprintf("    %-18s %-9s %-7s %-12s %s", "JOB", "STATUS", "STEPS", "TOOK", "NEEDS"), width)) + "\n"
	for i, job := range detail.Jobs {
		row := fmt.Sprintf(" %s%s %-18s %-9s %-7s %-12s %s", m.cipSelectMarkAt(i, m.cipJobSel),
			cipJobMark(job.Status, m.cipFrame, static), truncate(job.Name, 18), job.Status,
			cipJobSteps(job), cipJobTime(job, now), strings.Join(job.Needs, ", "))
		out += cipListRowStyle(i == m.cipJobSel, job.Status).Render(clampLine(strings.TrimRight(row, " "), width)) + "\n"
	}
	return out
}

func orchestratorRefresh(focused bool) time.Duration {
	if focused {
		return time.Second
	}
	return 15 * time.Second
}

func (m Model) orchestratorView() string {
	snap := m.orchestrator
	if snap.Error != "" {
		return "  " + titleStyle.Render("ORCHESTRATOR") + "\n\n  " + errStyle.Render("unavailable: "+snap.Error) + "\n"
	}
	health := okStyle.Bold(true).Render("HEALTHY")
	if !snap.Healthy {
		health = errStyle.Bold(true).Render("DEGRADED")
	}
	agentWord := "agents"
	if snap.Totals.Live == 1 {
		agentWord = "agent"
	}
	summary := fmt.Sprintf("%s  %s  •  mode %s  •  %s  •  %d %s live\n%s %s %s    %s  %s",
		titleStyle.Render("ORCHESTRATOR"), health, snap.Mode, dimStyle.Render(snap.AccountLabel()), snap.Totals.Live, agentWord,
		warnStyle.Render("CPU"), bar(snap.Daemon.CPUPercent, 10), percentText(snap.Daemon.CPUPercent),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA")).Render("MEM"), bytes(uint64(max(0, snap.Daemon.RSSBytes))))
	out := lipgloss.NewStyle().MarginLeft(2).Border(lipgloss.RoundedBorder()).BorderForeground(panel).Padding(0, 1).Render(summary) + "\n\n"
	if m.orchestratorModeMenu {
		out += m.orchestratorModeMenuView(snap)
	}
	out += orchestratorLimitsView(snap.Limits)
	// The plan usage bars above are the real constraint on a subscription;
	// the dollar figure is secondary information, so it stays small and
	// dim rather than competing with the bars for attention.
	out += "  " + dimStyle.Render(snap.CostText()) + "\n\n"
	out += orchestratorDiskView(snap.Disk)
	if len(snap.Agents) == 0 {
		// History still matters when nothing runs: it is the only place a
		// blocked or finished task can be seen at all.
		return out + dimStyle.Render("  No agents are running.") + "\n\n" +
			orchestratorHistoryView(snap, max(70, m.width-4)) + "\n  m set mode\n"
	}
	out += "  " + titleStyle.Render("ACTIVE AGENTS") + "  " + dimStyle.Render("live task list") + "\n\n"
	contentWidth := max(70, m.width-4)
	out += dimStyle.Render(truncate(fmt.Sprintf("  %-6s %s %-11s %-9s %-6s %-4s %s", "ISSUE", " ", "STATE", "ELAPSED", "WK%", "SUB", "TITLE"), contentWidth)) + "\n"
	for i, a := range snap.Agents {
		wk := "—"
		if a.WeeklyPercentUsed != nil {
			wk = fmt.Sprintf("%.1f%%", *a.WeeklyPercentUsed)
		}
		// A marker only appears when the daemon reports children for this
		// agent; an absent or empty children list leaves the column blank
		// so it never falls out of alignment with the other rows.
		sub := ""
		if len(a.Children) > 0 {
			sub = fmt.Sprintf("+%d", len(a.Children))
		}
		// A running agent gets a turning spinner. A still row for minutes
		// reads as a hang, even when the work is going fine.
		mark := " "
		if a.PID != 0 {
			mark = spinnerFrame(m.agentTick)
		}
		line := fmt.Sprintf("  #%-5d %s %-11s %-9s %-6s %-4s %s", a.Issue, mark, a.State,
			elapsedDuration(time.Duration(a.ElapsedSeconds)*time.Second), wk, sub, truncate(a.Title, 40))
		if i == m.orchestratorSel {
			out += lipgloss.NewStyle().Background(panel).Bold(true).Foreground(cyan).Render(pad(line, contentWidth)) + "\n"
		} else {
			out += line + "\n"
		}
	}
	if m.orchestratorSel < 0 || m.orchestratorSel >= len(snap.Agents) {
		return out
	}
	out += "\n" + m.orchestratorAgentDetail(snap.Agents[m.orchestratorSel])
	prHint := dimStyle.Render("p open pull request (none yet)")
	if a := m.orchestratorSelected(); a != nil && orchestratorPRURL(m.orchestrator.Repo, a.PRNumber) != "" {
		prHint = "p open pull request"
	}
	out += "\n" + orchestratorHistoryView(snap, contentWidth)
	out += "\n  ↑/↓ select agent  •  i open issue  •  " + prHint + "  •  m set mode\n"
	return out
}

// orchestratorModeMenuView renders the mode-select menu: the three run
// modes with the wording the operator's own CLI uses, the current mode
// highlighted, and a second Enter required to confirm before the write is
// sent — the same arm-then-confirm shape as the DEVTOOLS tab.
func (m Model) orchestratorModeMenuView(snap widget.OrchestratorSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %s  %s\n\n", titleStyle.Render("SET MODE"), dimStyle.Render("current: "+snap.Mode))
	width := max(50, m.width-4)
	for i, opt := range orchestratorModes {
		line := fmt.Sprintf("  %-9s %s", opt.Value, opt.Description)
		if i == m.orchestratorModeCursor {
			b.WriteString(lipgloss.NewStyle().Background(panel).Bold(true).Foreground(cyan).Render(pad(line, width)) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	switch {
	case m.orchestratorModeBusy:
		b.WriteString(warnStyle.Render("\n  ◌ working…\n"))
	case m.orchestratorModeConfirm:
		fmt.Fprintf(&b, "\n  Confirm switch to %s: press Enter again; Esc cancels.\n", orchestratorModes[m.orchestratorModeCursor].Value)
	default:
		b.WriteString("\n  ↑/↓ select  Enter arm  Enter again confirm  Esc cancel\n")
	}
	if m.orchestratorModeMessage != "" {
		style := okStyle
		if m.orchestratorModeMessageIsError {
			style = errStyle
		}
		b.WriteString("  " + style.Render(m.orchestratorModeMessage) + "\n")
	}
	return b.String() + "\n"
}

// orchestratorLimitsView renders the two subscription plan usage bars. A nil
// window (no reading yet) draws no bar at all, since an empty bar would read
// as "plenty left" rather than "unknown".
func orchestratorLimitsView(limits *widget.OrchestratorLimits) string {
	if limits == nil || (limits.Weekly == nil && limits.FiveHour == nil) {
		return ""
	}
	label := "PLAN USAGE"
	if limits.PlanType != "" {
		label += "  " + dimStyle.Render(limits.PlanType)
	}
	out := "  " + titleStyle.Render(label) + "\n\n"
	out += orchestratorUsageBar("WEEKLY", limits.Weekly)
	out += orchestratorUsageBar("5-HOUR", limits.FiveHour)
	return out + "\n"
}
func orchestratorUsageBar(label string, w *widget.OrchestratorUsageWindow) string {
	if w == nil {
		return fmt.Sprintf("  %-7s %s\n", label, dimStyle.Render("no reading"))
	}
	resets := dimStyle.Render("resets now")
	if until := time.Until(time.Unix(w.ResetsAt, 0)); until > 0 {
		resets = dimStyle.Render("resets in " + duration(until.Seconds()))
	}
	return fmt.Sprintf("  %-7s %s  %s  %s\n", label, bar(w.UsedPercent, 24), percentText(w.UsedPercent), resets)
}

// orchestratorDiskView renders the daemon host's overall disk usage. A nil
// disk (no reading yet) draws nothing, for the same reason a nil usage
// window draws no bar.
func orchestratorDiskView(disk *widget.OrchestratorDisk) string {
	if disk == nil {
		return ""
	}
	percent := 0.0
	if disk.TotalBytes > 0 {
		percent = float64(disk.UsedBytes) / float64(disk.TotalBytes) * 100
	}
	out := "  " + titleStyle.Render("HOST DISK") + "\n\n"
	out += fmt.Sprintf("  %-7s %s  %s  %s used / %s total\n", "DISK", bar(percent, 24), percentText(percent), bytes(uint64(max(0, disk.UsedBytes))), bytes(uint64(max(0, disk.TotalBytes))))
	return out + "\n"
}

// orchestratorAgentDetail renders every field of one agent: the live "now
// doing" activity line with its age, the issue and pull request links in
// full (so a reader can copy them by hand), CPU/memory as a point-in-time
// reading plus a live trend, and the spark subagent tree when the daemon
// reports one.
func (m Model) orchestratorAgentDetail(sel widget.OrchestratorAgent) string {
	cardWidth := max(48, min(76, m.width-8))
	accent := orchestratorAccent(sel)

	// The CPU/memory series only holds the readings collected since this
	// agent tab was opened; it is empty right after selecting a new agent.
	points := m.agentUsage[sel.Issue]
	cpuValues := make([]float64, len(points))
	maxRSS := int64(0)
	for i, p := range points {
		cpuValues[i] = math.Min(100, p.CPUPercent)
		if p.RSSBytes > maxRSS {
			maxRSS = p.RSSBytes
		}
	}
	memTrend := ""
	if maxRSS > 0 {
		memValues := make([]float64, len(points))
		for i, p := range points {
			memValues[i] = float64(p.RSSBytes) / float64(maxRSS) * 100
		}
		memTrend = sparkValues(memValues)
	}

	var b strings.Builder

	// Identity card: which issue, what state, what it is called.
	badge := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("● " + strings.ToUpper(or(sel.State, "unknown")))
	issue := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(fmt.Sprintf("ISSUE #%d", sel.Issue))
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Render(or(sel.Title, "title pending"))
	meta := dimStyle.Render(fmt.Sprintf("%s  •  cycle %d  •  %s elapsed",
		or(sel.Branch, "no branch"), sel.Cycle,
		elapsedDuration(time.Duration(sel.ElapsedSeconds)*time.Second)))
	b.WriteString(orchestratorCard(issue+"  "+badge+"\n"+title+"\n"+meta, accent, cardWidth))

	// Live usage card: the numbers that move while you watch.
	cpuStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)
	memStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
	cpuLine := cpuStyle.Render("CPU") + " " + bar(sel.CPUPercent, 12) + " " + percentText(sel.CPUPercent) +
		dimStyle.Render("  ") + lipgloss.NewStyle().Foreground(accent).Render(sparkValues(cpuValues))
	memLine := memStyle.Render("RAM "+bytes(uint64(max(0, sel.RSSBytes)))) +
		dimStyle.Render("  ") + memStyle.Render(memTrend) +
		dimStyle.Render(fmt.Sprintf("  •  pid %d", sel.PID))
	wk := "—"
	if sel.WeeklyPercentUsed != nil {
		wk = fmt.Sprintf("%.1f%% of the week", *sel.WeeklyPercentUsed)
	}
	costLine := dimStyle.Render(fmt.Sprintf("%d in / %d out  •  %d turns  •  ~$%.3f  •  %s",
		sel.InputTokens, sel.OutputTokens, sel.Turns, sel.CostUSD, wk))
	b.WriteString(orchestratorCard(cpuLine+"\n"+memLine+"\n"+costLine, accent, cardWidth))

	// Location card: the links and paths a reader copies by hand.
	pr := dimStyle.Render("no pull request yet")
	if url := orchestratorPRURL(m.orchestrator.Repo, sel.PRNumber); url != "" {
		pr = lipgloss.NewStyle().Foreground(cyan).Render(url)
	}
	where := lipgloss.NewStyle().Foreground(cyan).Render(orchestratorIssueURL(m.orchestrator.Repo, sel.Issue)) + "\n" + pr
	tree := or(sel.Worktree, "no worktree")
	if sel.WorktreeDiskBytes != nil {
		tree += fmt.Sprintf("  •  %s on disk", bytes(uint64(max(0, *sel.WorktreeDiskBytes))))
	}
	where += "\n" + dimStyle.Render(tree)
	b.WriteString(orchestratorCard(where, muted, cardWidth))

	// What it is doing right now, and why it stopped if it did.
	// The tail: what the agent has been doing, newest last. One line is a
	// snapshot; several lines show whether it is making progress.
	if tail := m.agentTail[sel.Issue]; len(tail) > 0 {
		head := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("ACTIVITY")
		if sel.ActivityAgeSeconds != nil {
			head += dimStyle.Render(fmt.Sprintf("  last change %s ago",
				elapsedDuration(time.Duration(*sel.ActivityAgeSeconds)*time.Second)))
		}
		var t strings.Builder
		t.WriteString(head)
		for i, line := range tail {
			if i == len(tail)-1 {
				t.WriteString("\n" + lipgloss.NewStyle().Foreground(accent).Render(spinnerFrame(m.agentTick)+" ") + line)
			} else {
				t.WriteString("\n" + dimStyle.Render("  "+line))
			}
		}
		b.WriteString(orchestratorCard(t.String(), accent, cardWidth))
	} else if sel.LastActivity != nil {
		b.WriteString(orchestratorCard(
			lipgloss.NewStyle().Bold(true).Foreground(accent).Render("NOW")+"\n"+*sel.LastActivity,
			accent, cardWidth))
	}
	if sel.LastError != "" {
		b.WriteString(orchestratorCard(
			lipgloss.NewStyle().Bold(true).Foreground(red).Render("LAST ERROR")+"\n"+sel.LastError, red, cardWidth))
	}

	b.WriteString(m.orchestratorTasksCard(sel, cardWidth))
	b.WriteString(orchestratorChildrenView(sel))
	return b.String()
}

// orchestratorAccent picks the card colour from the agent's state, so the
// pane reads at a glance: green is working, yellow is under review, red
// stopped, cyan finished.
func orchestratorAccent(sel widget.OrchestratorAgent) lipgloss.Color {
	if sel.LastError != "" || sel.State == "blocked" || sel.State == "failed" {
		return red
	}
	switch sel.State {
	case "auditing", "revising":
		return yellow
	case "done", "merged":
		return cyan
	default:
		return green
	}
}

// orchestratorCard wraps content in the same bordered card the runners tab
// uses, so the two widgets read as one design.
func orchestratorCard(content string, accent lipgloss.Color, width int) string {
	card := lipgloss.NewStyle().Width(width).Border(lipgloss.RoundedBorder()).
		BorderLeft(true).BorderForeground(accent).Padding(0, 1).Render(content)
	return lipgloss.NewStyle().MarginLeft(2).Render(card) + "\n"
}

// orchestratorTasksCard renders the agent's self-tracked checklist as a card
// with a progress bar. sel.Tasks is nil while the daemon does not report a
// checklist; that case must render nothing, because "not reported" is a
// different fact from "an empty checklist".
func (m Model) orchestratorTasksCard(sel widget.OrchestratorAgent, width int) string {
	if sel.Tasks == nil {
		return ""
	}
	head := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render("CHECKLIST")
	if len(sel.Tasks) == 0 {
		return orchestratorCard(head+"\n"+dimStyle.Render("no checklist tracked"), muted, width)
	}
	done := 0
	for _, t := range sel.Tasks {
		if t.Done {
			done++
		}
	}
	percent := float64(done) / float64(len(sel.Tasks)) * 100
	var b strings.Builder
	b.WriteString(head + "  " + bar(percent, 12) + dimStyle.Render(fmt.Sprintf("  %d/%d done", done, len(sel.Tasks))))
	for _, t := range sel.Tasks {
		if t.Done {
			b.WriteString("\n" + okStyle.Render("☑ ") +
				lipgloss.NewStyle().Strikethrough(true).Foreground(muted).Render(t.Text))
		} else {
			b.WriteString("\n" + dimStyle.Render("☐ ") + t.Text)
		}
	}
	return orchestratorCard(b.String(), cyan, width)
}

// orchestratorChildrenView renders the spark subagent tree under one agent.
// sel.Children is nil while the daemon does not report children yet; that
// case must render nothing at all, not an empty heading or a zero count,
// since a missing report and a report of zero children are different facts.
func orchestratorChildrenView(sel widget.OrchestratorAgent) string {
	if sel.Children == nil {
		return ""
	}
	if len(sel.Children) == 0 {
		return fmt.Sprintf("  %-14s %s\n", "SUBAGENTS", dimStyle.Render("none launched"))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %-14s %d running · %d done · %d failed\n", "SUBAGENTS", sel.ChildrenRunning, sel.ChildrenDone, sel.ChildrenFailed)
	for _, c := range sel.Children {
		state := fmt.Sprintf("%-8s", c.State)
		switch c.State {
		case "failed":
			state = errStyle.Render(state)
		case "running":
			state = okStyle.Render(state)
		default:
			state = dimStyle.Render(state)
		}
		fmt.Fprintf(&b, "      %s %-30s %-8s %d in / %d out\n", state, truncate(c.Task, 30), elapsedDuration(time.Duration(c.ElapsedSeconds)*time.Second), c.InputTokens, c.OutputTokens)
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
		if inline := emitDesktopImage(b, cols, desktopImageRows(m.height)); inline != "" {
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
	if inline := emitDesktopImage(b, cols, desktopImageRows(m.height)); inline != "" {
		return inline
	}
	img, err := png.Decode(stdbuf.NewReader(b))
	if err != nil {
		return ""
	}
	return ansiFrame(img, cols)
}

func (m Model) rememberDesktopFrameSize(index int, b []byte) {
	cfg, err := png.DecodeConfig(stdbuf.NewReader(b))
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		m.desktopFrameSize[index] = image.Point{X: cfg.Width, Y: cfg.Height}
	}
}

func (m Model) desktopImageCells(index int) (int, int) {
	cols := max(40, min(110, m.width-4))
	if d := m.desktopForServer(index); d != nil {
		switch d.Quality {
		case "speed":
			cols = max(40, min(80, m.width-4))
		case "quality":
			cols = max(40, min(180, m.width-4))
		}
	}
	return cols, desktopImageRows(m.height)
}

// desktopImageOrigin derives the rendered image's terminal-cell origin from
// the same detail view used for drawing, so headers/tabs/scrolling stay in sync.
func (m Model) desktopImageOrigin(index int) (int, int, bool) {
	if index < 0 || index >= len(m.cfg.Servers) || m.desktopFrames[index] == "" {
		return 0, 0, false
	}
	body := m.detailView()
	pos := strings.Index(body, m.desktopFrames[index])
	if pos < 0 {
		return 0, 0, false
	}
	return 0, lipgloss.Height(m.header()) + lipgloss.Height(body[:pos]) - m.detailScroll, true
}

func desktopImageRows(height int) int {
	return max(8, min(40, height-12))
}
func isRemoteDesktopKey(key string) bool {
	// These keys belong to the TUI desktop-tab controls. Keep them out of
	// the remote input path so Enter actually opens the persistent stream.
	if key == "q" || key == "ctrl+c" || key == "esc" || key == "tab" || key == "8" || key == "c" || key == "enter" || key == "return" || key == "right" || key == "l" || key == "left" || key == "h" {
		return false
	}
	return key != "" && key != "up" && key != "down" && key != "left" && key != "right" || key == "up" || key == "down" || key == "left" || key == "right"
}

func leavesDesktopTab(key string) bool {
	if key == "8" {
		return false
	}
	return key == "tab" || key == "1" || key == "2" || key == "3" || key == "4" || key == "5" || key == "6" || key == "7" || key == "9" || key == "d" || key == "o" || key == "s" || key == "esc" || key == "left" || key == "h" || key == "x" || key == "q" || key == "ctrl+c"
}
func (m Model) sendDesktopKey(index int, combo string) tea.Cmd {
	return func() tea.Msg {
		if stream := m.desktopStreams[index]; stream != nil {
			_ = stream.Key(combo)
			return nil
		}
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
func (m Model) sendDesktopClipboard(index int) tea.Cmd {
	return func() tea.Msg {
		text, err := localClipboard()
		if err != nil || text == "" {
			return nil
		}
		if stream := m.desktopStreams[index]; stream != nil {
			_ = stream.ClipboardSet(text)
			return nil
		}
		// Clipboard writes are supported by the persistent stream. The legacy
		// screenshot/control endpoint has no clipboard route, so do not invent
		// one or silently broaden its mutation surface.
		return nil
	}
}
func (m Model) sendDesktopClick(index, x, y int, right bool) tea.Cmd {
	return func() tea.Msg {
		remoteX, remoteY, ok := m.desktopRemotePoint(index, x, y)
		if !ok {
			return nil
		}
		if stream := m.desktopStreams[index]; stream != nil {
			_ = stream.Click(remoteX, remoteY, right)
			return nil
		}
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
		_ = right
		_ = desktopclient.Click(context.Background(), *d, token, remoteX, remoteY)
		return nil
	}
}

func (m Model) desktopRemotePoint(index, x, y int) (int, int, bool) {
	cols, rows := m.desktopImageCells(index)
	ox, oy, ok := m.desktopImageOrigin(index)
	if !ok {
		return 0, 0, false
	}
	relX, relY := x-ox, y-oy
	if relX < 0 || relY < 0 || relX >= cols || relY >= rows {
		return 0, 0, false
	}
	size := m.desktopFrameSize[index]
	if size.X <= 0 || size.Y <= 0 {
		size = image.Point{X: 1280, Y: 800}
	}
	return min(size.X-1, relX*size.X/max(1, cols)), min(size.Y-1, relY*size.Y/max(1, rows)), true
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

// openURL opens one web URL in the OS default browser. It runs no
// user-supplied command; it only ever passes url to "open" or "xdg-open",
// the same pattern openDesktop uses for a vnc:// URI. An empty url is a
// no-op, so a caller with no real link (for example no pull request yet)
// can never launch a wrong or half-built page.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		if url == "" {
			return nil
		}
		if runtime.GOOS == "darwin" {
			_ = exec.Command("open", url).Run()
		} else {
			_ = exec.Command("xdg-open", url).Run()
		}
		return nil
	}
}
func setLocalClipboard(text string) {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		_ = cmd.Run()
		return
	}
	if _, err := exec.LookPath("wl-copy"); err == nil {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return
		}
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return
		}
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		_ = cmd.Run()
	}
}
func localClipboard() (string, error) {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("pbpaste").Output()
		return string(out), err
	}
	if _, err := exec.LookPath("wl-paste"); err == nil {
		out, err := exec.Command("wl-paste", "--no-newline").Output()
		if err == nil {
			return string(out), nil
		}
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
		if err == nil {
			return string(out), nil
		}
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		out, err := exec.Command("xsel", "--clipboard", "--output").Output()
		return string(out), err
	}
	return "", fmt.Errorf("no clipboard reader available")
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

// sparkValues renders a compact trend line for a series already scaled to a
// 0-100 range. It mirrors spark()'s character ramp and window size; a
// sibling function is needed because spark() is tied to []metrics.Sample,
// while this caller's series (per-agent CPU/memory) has no Sample type.
func sparkValues(values []float64) string {
	chars := []rune("▁▂▃▄▅▆▇█")
	if len(values) == 0 {
		return ""
	}
	start := max(0, len(values)-18)
	var b strings.Builder
	for _, v := range values[start:] {
		i := int(math.Round(math.Max(0, math.Min(100, v)) / 100 * 7))
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
