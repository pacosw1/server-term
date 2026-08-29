package widget

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// CIPJob is one job of a pipeline run. The cip daemon leaves its Go fields
// untagged, so the JSON keys are the capitalized Go names.
type CIPJob struct {
	Name    string `json:"Name"`
	Status  string `json:"Status"`
	LogPath string `json:"LogPath"`
	// Needs names the jobs this job waits for. It gives the edges of the
	// dependency graph.
	Needs      []string  `json:"Needs"`
	StartedAt  time.Time `json:"StartedAt"`
	FinishedAt time.Time `json:"FinishedAt"`
	StepsTotal int       `json:"StepsTotal"`
	StepsDone  int       `json:"StepsDone"`
	// Finished says whether FinishedAt holds a real time. FetchCIPRun sets
	// it. Callers must read it instead of testing FinishedAt.
	Finished bool `json:"finished"`
}

// Started reports whether the job began. A pending job has no start time.
func (j CIPJob) Started() bool { return !j.StartedAt.IsZero() }

// Duration is how long the job took, or how long it has run so far. A job
// that did not start yet takes zero time. Without that guard a pending job
// would report the age of the Go zero time, which is two thousand years.
func (j CIPJob) Duration(now time.Time) time.Duration {
	if !j.Started() {
		return 0
	}
	end := now
	if j.Finished {
		end = j.FinishedAt
	}
	d := end.Sub(j.StartedAt)
	if d < 0 {
		return 0
	}
	return d
}

// CIPRunDetail is one run and the jobs it contains.
type CIPRunDetail struct {
	SchemaVersion int       `json:"schema_version"`
	Name          string    `json:"name"`
	At            time.Time `json:"at"`
	Run           CIPRun    `json:"run"`
	Jobs          []CIPJob  `json:"jobs"`
	Error         string    `json:"error,omitempty"`
}

// CountByStatus is the number of jobs in one state, for example "running".
func (d CIPRunDetail) CountByStatus(status string) int {
	n := 0
	for _, job := range d.Jobs {
		if job.Status == status {
			n++
		}
	}
	return n
}

// cipRunBody is the GET /runs/{id} body.
type cipRunBody struct {
	Run  CIPRun   `json:"run"`
	Jobs []CIPJob `json:"jobs"`
}

// FetchCIPRun reads only the authenticated GET /runs/{id} endpoint. It never
// starts, stops, or cancels a run. It fails closed the same way FetchCIP
// does, so a fault never shows as an empty but healthy graph.
func FetchCIPRun(ctx context.Context, provider config.Widget, token string, id int) CIPRunDetail {
	// Record the run before anything can fail, so an error result still
	// names the run it belongs to and a caller can match it to the run the
	// reader looks at.
	result := CIPRunDetail{SchemaVersion: 1, Name: provider.Name, At: time.Now(), Run: CIPRun{ID: id}}
	if strings.TrimSpace(token) == "" {
		result.Error = "no token configured: set token_env or token_file"
		return result
	}
	base := strings.TrimRight(provider.Endpoint, "/")
	var body cipRunBody
	if err := cipGet(ctx, fmt.Sprintf("%s/runs/%d", base, id), token, &body); err != nil {
		result.Error = err.Error()
		return result
	}
	result.At = time.Now()
	body.Run.Finished = !body.Run.FinishedAt.IsZero()
	for i := range body.Jobs {
		body.Jobs[i].Finished = !body.Jobs[i].FinishedAt.IsZero()
	}
	result.Run, result.Jobs = body.Run, body.Jobs
	return result
}

// CIPJobColumns groups the jobs into dependency columns. A job with no
// dependency goes in the first column. A job goes one column past the
// deepest job it waits for. The columns are what the graph draws from left
// to right.
//
// The daemon controls this data, so the layout must survive bad input. A
// need that names no known job is ignored. A cycle cannot loop forever and
// cannot drop a job, because a hidden job is worse than an odd order.
func CIPJobColumns(jobs []CIPJob) [][]CIPJob {
	if len(jobs) == 0 {
		return nil
	}
	names := make([]string, len(jobs))
	needs := make([][]string, len(jobs))
	for i, job := range jobs {
		names[i], needs[i] = job.Name, job.Needs
	}
	columns := make([][]CIPJob, 0)
	for _, group := range dependencyColumns(names, needs) {
		column := make([]CIPJob, 0, len(group))
		for _, i := range group {
			column = append(column, jobs[i])
		}
		columns = append(columns, column)
	}
	return columns
}

// dependencyColumns groups items into columns by dependency depth. An item
// with no dependency goes in the first column. An item goes one column past
// the deepest item it waits for. It returns the indexes of the items, so
// both the job graph and the stage flow can share one layout.
//
// The daemon controls this data, so the layout must survive bad input. A
// need that names no known item is ignored. A cycle cannot loop forever and
// cannot drop an item, because a hidden item is worse than an odd order.
func dependencyColumns(names []string, needs [][]string) [][]int {
	if len(names) == 0 {
		return nil
	}
	index := make(map[string]int, len(names))
	for i, name := range names {
		index[name] = i
	}
	const (
		unvisited = -1
		visiting  = -2
	)
	depths := make([]int, len(names))
	for i := range depths {
		depths[i] = unvisited
	}
	// depthOf walks the dependencies of one item. The visiting mark breaks a
	// cycle: an item that depends on itself counts as a root instead of
	// recurring forever.
	var depthOf func(i int) int
	depthOf = func(i int) int {
		if depths[i] == visiting {
			return 0
		}
		if depths[i] != unvisited {
			return depths[i]
		}
		depths[i] = visiting
		depth := 0
		for _, need := range needs[i] {
			j, ok := index[need]
			if !ok || j == i {
				continue
			}
			if d := depthOf(j) + 1; d > depth {
				depth = d
			}
		}
		depths[i] = depth
		return depth
	}
	widest := 0
	for i := range names {
		if d := depthOf(i); d > widest {
			widest = d
		}
	}
	columns := make([][]int, widest+1)
	// Keep the daemon's order inside a column, so the graph does not
	// reshuffle between two reads of the same run.
	for i := range names {
		columns[depths[i]] = append(columns[depths[i]], i)
	}
	return columns
}
