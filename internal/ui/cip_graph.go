package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/franciscosainzwilliams/server-term/internal/widget"
)

// cipSpinnerMarks turn one after the other to show that a job still runs.
var cipSpinnerMarks = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧"}

// cipSpinnerFrames is the length of one full turn of the spinner.
const cipSpinnerFrames = 8

// cipSpinner is the spinner mark for one frame. The caller owns the frame
// counter, so a test can pin the animation to an exact frame.
func cipSpinner(frame int) string {
	n := len(cipSpinnerMarks)
	return cipSpinnerMarks[((frame%n)+n)%n]
}

// ansiPattern matches the style codes that lipgloss writes into a line.
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// stripANSI removes the style codes, to leave the text a reader sees.
func stripANSI(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// cipRunIDAtLine reads the run id from one rendered row of the run list. It
// accepts only a row that starts with "#" and a number, so a heading or a
// storage row can never be mistaken for a run.
func cipRunIDAtLine(line string) (int, bool) {
	text := strings.TrimSpace(stripANSI(line))
	if !strings.HasPrefix(text, "#") {
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

// clampLine cuts one line to the width of the panel. It measures the text a
// reader sees, so the style codes do not count against the width. A line
// that must be cut loses its style, because a cut in the middle of a style
// code would corrupt the rest of the screen.
func clampLine(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	return truncate(stripANSI(s), width)
}

// clampBlock cuts every line of a block to the width of the panel.
func clampBlock(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = clampLine(line, width)
	}
	return strings.Join(lines, "\n")
}

// cipJobMark is the state mark of one job. A running job turns while the
// run is live. A run that already ended is static: a spinner there would
// suggest that work continues.
func cipJobMark(status string, frame int, static bool) string {
	switch status {
	case "success":
		return okStyle.Render("✓")
	case "failed":
		return errStyle.Render("✗")
	case "running":
		if static {
			return warnStyle.Render("•")
		}
		return warnStyle.Render(cipSpinner(frame))
	case "skipped":
		return dimStyle.Render("–")
	default:
		return dimStyle.Render("·")
	}
}

// cipJobStyle colors a job box by its state.
func cipJobStyle(status string) lipgloss.Style {
	switch status {
	case "success":
		return lipgloss.NewStyle().Foreground(green)
	case "failed":
		return lipgloss.NewStyle().Foreground(red)
	case "running":
		return lipgloss.NewStyle().Foreground(yellow)
	default:
		return dimStyle
	}
}

// cipJobSteps is the step progress of one job, for example "1/3". It is
// empty when the daemon reports no step total.
func cipJobSteps(job widget.CIPJob) string {
	if job.StepsTotal <= 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", job.StepsDone, job.StepsTotal)
}

// cipJobTime is how long a job took, or how long it has run. It is empty
// for a job that did not start, because a job that waits has no age.
func cipJobTime(job widget.CIPJob, now time.Time) string {
	if !job.Started() {
		return ""
	}
	return job.Duration(now).Truncate(time.Second).String()
}

// cipNodeBox draws one node of a graph: a mark and a name on the first
// line, and a short detail on the second. The job graph and the stage flow
// share it, so both read the same way.
func cipNodeBox(mark, name, foot string, style lipgloss.Style, inner int) string {
	head := mark + " " + truncate(name, inner-2)
	body := pad(head, inner) + "\n" + dimStyle.Render(pad(truncate(foot, inner), inner))
	return style.Border(lipgloss.RoundedBorder()).Render(body)
}

// cipJoinColumns puts the columns side by side with a connector between
// them. It reports false when the result is wider than the panel, so the
// caller can fall back to a compact form.
func cipJoinColumns(columns []string, width int) (string, bool) {
	blocks := make([]string, 0, len(columns)*2)
	for i, column := range columns {
		blocks = append(blocks, column)
		if i < len(columns)-1 {
			blocks = append(blocks, dimStyle.Render(" ──▶ "))
		}
	}
	// Two spaces of margin keep the graph in line with the other panels.
	joined := indentBlock(lipgloss.JoinHorizontal(lipgloss.Center, blocks...), 2)
	return joined, lipgloss.Width(joined) <= width
}

// cipJobBox draws one job as a node of the graph.
func cipJobBox(job widget.CIPJob, now time.Time, frame, inner int, static bool) string {
	foot := strings.TrimSpace(cipJobSteps(job) + "  " + cipJobTime(job, now))
	if foot == "" {
		foot = job.Status
	}
	return cipNodeBox(cipJobMark(job.Status, frame, static), job.Name, foot,
		cipJobStyle(job.Status), inner)
}

// cipGraphView draws the pipeline as boxes in dependency columns, with a
// connector between the columns. A narrow terminal gets a compact list
// instead, so the panel stays readable rather than wrapping into noise.
//
// It fails closed: a read error replaces the graph with the reason, because
// an empty graph reads as a pipeline with nothing to do.
func cipGraphView(detail widget.CIPRunDetail, now time.Time, frame, width int) string {
	if detail.Error != "" {
		return clampBlock("  "+titleStyle.Render("PIPELINE")+"\n\n  "+errStyle.Render("unavailable: "+detail.Error), width) + "\n"
	}
	head := cipGraphHeader(detail, now, width)
	if len(detail.Jobs) == 0 {
		return head + clampBlock(dimStyle.Render("  No job exists for this run yet."), width) + "\n"
	}
	// A run that ended never changes again, so it draws without animation.
	static := detail.Run.Status != "running"
	columns := widget.CIPJobColumns(detail.Jobs)
	inner := cipNodeWidth(detail.Jobs)
	blocks := make([]string, 0, len(columns))
	for _, column := range columns {
		boxes := make([]string, 0, len(column))
		for _, job := range column {
			boxes = append(boxes, cipJobBox(job, now, frame, inner, static))
		}
		blocks = append(blocks, lipgloss.JoinVertical(lipgloss.Left, boxes...))
	}
	graph, fits := cipJoinColumns(blocks, width)
	if !fits {
		return head + cipGraphCompact(columns, now, frame, width, static)
	}
	return head + graph + "\n"
}

// cipGraphHeader names the run the graph belongs to.
func cipGraphHeader(detail widget.CIPRunDetail, now time.Time, width int) string {
	run := detail.Run
	if run.ID == 0 {
		return "  " + titleStyle.Render("PIPELINE") + "\n\n"
	}
	where := run.Branch
	if sha := run.ShortSHA(); sha != "" {
		where += "@" + sha
	}
	line := fmt.Sprintf("  %s  %s  %s  %s  %s",
		titleStyle.Render("PIPELINE"), titleStyle.Render(fmt.Sprintf("#%d", run.ID)),
		run.Repo, dimStyle.Render(where),
		cipRunStatusText(run, now))
	return clampLine(line, width) + "\n\n"
}

// cipRunStatusText is the state of a run with the time it took, or the time
// it has run so far.
func cipRunStatusText(run widget.CIPRun, now time.Time) string {
	age := run.Duration(now).Truncate(time.Second).String()
	if !run.Finished {
		age += " so far"
	}
	switch run.Status {
	case "failed":
		return errStyle.Render(run.Status) + "  " + dimStyle.Render(age)
	case "running":
		return warnStyle.Render(run.Status) + "  " + dimStyle.Render(age)
	default:
		return okStyle.Render(run.Status) + "  " + dimStyle.Render(age)
	}
}

// cipNodeWidth is the inner width of every node. One width for all of them
// keeps the columns in line.
func cipNodeWidth(jobs []widget.CIPJob) int {
	widest := 0
	for _, job := range jobs {
		if n := len([]rune(job.Name)); n > widest {
			widest = n
		}
	}
	return min(18, max(10, widest+3))
}

// cipGraphCompact is the narrow-terminal form of the graph. It keeps every
// job and its state, and gives up only the boxes and the connectors.
func cipGraphCompact(columns [][]widget.CIPJob, now time.Time, frame, width int, static bool) string {
	out := ""
	for _, column := range columns {
		for _, job := range column {
			line := fmt.Sprintf("  %s %s %s %s", cipJobMark(job.Status, frame, static),
				job.Name, cipJobSteps(job), cipJobTime(job, now))
			out += clampLine(strings.TrimRight(line, " "), width) + "\n"
		}
	}
	return out
}

// indentBlock moves a whole block to the right by n spaces.
func indentBlock(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
