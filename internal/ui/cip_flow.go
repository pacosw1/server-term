package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/franciscosainzwilliams/server-term/internal/widget"
)

// cipGatedMark shows a stage that waits for a person or a clock. It must
// never look like the running spinner: a gated stage does no work.
const cipGatedMark = "⏸"

// cipPromotionIDAtLine reads the promotion id from one rendered row. The
// row starts with "P" and a number, which no other row does, so a run row
// or a storage row can never be mistaken for a promotion.
func cipPromotionIDAtLine(line string) (int, bool) {
	text := strings.TrimSpace(stripANSI(line))
	// The selection bar puts a mark before the id.
	text = strings.TrimSpace(strings.TrimPrefix(text, "▸"))
	if !strings.HasPrefix(text, "P") {
		return 0, false
	}
	digits := text[1:]
	for i, r := range digits {
		if r < '0' || r > '9' {
			digits = digits[:i]
			break
		}
	}
	if digits == "" {
		return 0, false
	}
	id, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return id, true
}

// cipStageMark is the state mark of one stage. A gated stage gets its own
// mark, because it waits for a person and does no work. Only a running
// stage turns, and only while the promotion is live.
func cipStageMark(state string, frame int, static bool) string {
	switch state {
	case "passed":
		return okStyle.Render("✓")
	case "failed":
		return errStyle.Render("✗")
	case "running":
		if static {
			return warnStyle.Render("•")
		}
		return warnStyle.Render(cipSpinner(frame))
	case "gated":
		return lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(cipGatedMark)
	case "superseded":
		return dimStyle.Render("–")
	default:
		return dimStyle.Render("·")
	}
}

// cipStageStyle colors a stage box by its state.
func cipStageStyle(state string) lipgloss.Style {
	switch state {
	case "passed":
		return lipgloss.NewStyle().Foreground(green)
	case "failed":
		return lipgloss.NewStyle().Foreground(red)
	case "running":
		return lipgloss.NewStyle().Foreground(yellow)
	case "gated":
		return lipgloss.NewStyle().Foreground(cyan)
	default:
		return dimStyle
	}
}

// cipStageDetail is the short line under a stage name. A gated stage says
// why it waits, which is the reason a reader opens this view at all.
func cipStageDetail(stage widget.CIPStage, spec widget.CIPSpec, now time.Time) string {
	if stage.State == "gated" {
		reason := "gated"
		if gate, ok := spec.GateFor(stage.Stage, stage.GateIdx); ok {
			reason = gate.Describe()
		}
		// Show the wait only once it is worth a word. A gate that just
		// started would otherwise read "0s".
		if waited := stage.WaitingFor(now); waited >= time.Minute {
			reason += " " + waited.Truncate(time.Minute).String()
		}
		return reason
	}
	if stage.HasRun() {
		return fmt.Sprintf("%s #%d", stage.State, stage.RunID)
	}
	return stage.State
}

// cipStageNote is the extra word a stage row needs, for a table that
// already shows the state and the run in their own columns. It is empty
// when the stage has nothing more to say.
func cipStageNote(stage widget.CIPStage, spec widget.CIPSpec, now time.Time) string {
	if stage.State == "gated" {
		reason := "gated"
		if gate, ok := spec.GateFor(stage.Stage, stage.GateIdx); ok {
			reason = gate.Describe()
		}
		if waited := stage.WaitingFor(now); waited >= time.Minute {
			reason += " for " + waited.Truncate(time.Minute).String()
		}
		return reason
	}
	if stage.Approved() && stage.ApprovedBy != "" {
		note := "approved by " + stage.ApprovedBy
		if stage.ApproveReason != "" {
			note += ": " + stage.ApproveReason
		}
		return note
	}
	return ""
}

// cipStageFlowView draws the promotion as stage boxes in dependency
// columns, left to right, with a connector between the columns. This is the
// flow the reader watches: which stage passed, which one runs, and which
// one waits for a person.
//
// A narrow terminal gets a compact list instead, so the pane stays readable
// rather than wrapping into noise.
func cipStageFlowView(entry widget.CIPPromotionEntry, spec widget.CIPSpec, now time.Time, frame, width int) string {
	promotion := entry.Promotion
	head := cipPromotionHeader(promotion, width)
	if len(entry.Stages) == 0 {
		return head + clampBlock(dimStyle.Render("  This promotion has no stage yet."), width) + "\n"
	}
	// Only a live promotion animates. One that ended never changes again.
	static := promotion.State != "active"
	columns := widget.CIPStageColumns(entry.Stages, spec)
	inner := cipStageNodeWidth(entry.Stages, spec, now)
	blocks := make([]string, 0, len(columns))
	for _, column := range columns {
		boxes := make([]string, 0, len(column))
		for _, stage := range column {
			boxes = append(boxes, cipNodeBox(cipStageMark(stage.State, frame, static), stage.Stage,
				cipStageDetail(stage, spec, now), cipStageStyle(stage.State), inner))
		}
		blocks = append(blocks, lipgloss.JoinVertical(lipgloss.Left, boxes...))
	}
	flow, fits := cipJoinColumns(blocks, width)
	if !fits {
		return head + cipStageFlowCompact(columns, spec, now, frame, width, static)
	}
	return head + flow + "\n"
}

// cipPromotionHeader names the promotion the flow belongs to.
func cipPromotionHeader(promotion widget.CIPPromotion, width int) string {
	if promotion.ID == 0 {
		return "  " + titleStyle.Render("PROMOTION") + "\n\n"
	}
	where := promotion.Branch
	if sha := promotion.ShortSHA(); sha != "" {
		where += "@" + sha
	}
	line := fmt.Sprintf("  %s  %s  %s  %s  %s",
		titleStyle.Render("PROMOTION"), titleStyle.Render(fmt.Sprintf("P%d", promotion.ID)),
		promotion.Repo, dimStyle.Render(where), cipPromotionStateText(promotion.State))
	return clampLine(line, width) + "\n\n"
}

// cipPromotionStateText colors the state of the whole promotion.
func cipPromotionStateText(state string) string {
	switch state {
	case "failed":
		return errStyle.Render(state)
	case "passed":
		return okStyle.Render(state)
	case "active":
		return warnStyle.Render(state)
	default:
		return dimStyle.Render(state)
	}
}

// cipStageNodeWidth is the inner width of every stage box. One width for
// all of them keeps the columns in line. The gate reason drives it, because
// that text is the longest and the most worth reading.
func cipStageNodeWidth(stages []widget.CIPStage, spec widget.CIPSpec, now time.Time) int {
	widest := 0
	for _, stage := range stages {
		if n := len([]rune(stage.Stage)); n > widest {
			widest = n
		}
		if n := len([]rune(cipStageDetail(stage, spec, now))); n > widest {
			widest = n
		}
	}
	return min(24, max(12, widest+3))
}

// cipStageFlowCompact is the narrow-terminal form of the flow. It keeps
// every stage and its reason, and gives up only the boxes and connectors.
func cipStageFlowCompact(columns [][]widget.CIPStage, spec widget.CIPSpec, now time.Time, frame, width int, static bool) string {
	widest := 0
	for _, column := range columns {
		for _, stage := range column {
			if n := len([]rune(stage.Stage)); n > widest {
				widest = n
			}
		}
	}
	out := ""
	for _, column := range columns {
		for _, stage := range column {
			line := fmt.Sprintf("  %s %s  %s", cipStageMark(stage.State, frame, static),
				pad(stage.Stage, widest), dimStyle.Render(cipStageDetail(stage, spec, now)))
			out += clampLine(strings.TrimRight(line, " "), width) + "\n"
		}
	}
	return out
}
