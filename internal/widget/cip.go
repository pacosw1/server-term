package widget

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// maxCIPRuns is the number of runs the snapshot keeps. The daemon holds far
// more. The widget shows only the newest few, so the panel stays readable.
// The counts still cover every run the daemon returned.
const maxCIPRuns = 8

// cipLowDiskPercent is the free share below which the host counts as
// degraded. The box's root partition filled to 95% once, so a low disk must
// show as a fault and not as a healthy widget.
const cipLowDiskPercent = 5.0

// CIPRun is one pipeline run. The cip daemon leaves its Go fields untagged,
// so the JSON keys are the capitalized Go names. The keys must match exactly
// or every field reads as its zero value.
type CIPRun struct {
	ID       int    `json:"ID"`
	Pipeline string `json:"Pipeline"`
	Backend  string `json:"Backend"`
	Repo     string `json:"Repo"`
	SHA      string `json:"SHA"`
	Branch   string `json:"Branch"`
	Trigger  string `json:"Trigger"`
	Stage    string `json:"Stage"`
	// Status is running, success, or failed.
	Status    string    `json:"Status"`
	StartedAt time.Time `json:"StartedAt"`
	// FinishedAt is the Go zero time while a run is still going. Callers must
	// read Finished instead of testing this field, and must never show the
	// zero time as a date.
	FinishedAt time.Time `json:"FinishedAt"`
	// Finished says whether FinishedAt holds a real time. FetchCIP sets it.
	Finished bool `json:"finished"`
}

// Duration is how long the run took, or how long it has run so far. A clock
// skew cannot make it negative, because a negative age reads as a fault that
// is not there.
func (r CIPRun) Duration(now time.Time) time.Duration {
	end := now
	if r.Finished {
		end = r.FinishedAt
	}
	d := end.Sub(r.StartedAt)
	if d < 0 {
		return 0
	}
	return d
}

// ShortSHA is the first 7 characters of the commit, which is enough to find
// the commit and short enough for one line.
func (r CIPRun) ShortSHA() string {
	if len(r.SHA) > 7 {
		return r.SHA[:7]
	}
	return r.SHA
}

// Line is one reader-facing row: the repository, the status, the duration,
// and the branch with the short commit. A running row shows the elapsed time
// instead of a finish time.
func (r CIPRun) Line(now time.Time) string {
	age := r.Duration(now).Truncate(time.Second).String()
	if !r.Finished {
		age += " so far"
	}
	where := r.Branch
	if sha := r.ShortSHA(); sha != "" {
		where += "@" + sha
	}
	return fmt.Sprintf("#%-4d %-24s %-8s %-14s %s", r.ID, r.Repo, r.Status, age, where)
}

// CIPRepoStorage is the disk one pipeline occupies, split by the kind of
// data. The daemon tags these keys in lower camel case.
type CIPRepoStorage struct {
	Name          string `json:"name"`
	CacheBytes    int64  `json:"cacheBytes"`
	RepoBytes     int64  `json:"repoBytes"`
	LogBytes      int64  `json:"logBytes"`
	ArtifactBytes int64  `json:"artifactBytes"`
	WorkBytes     int64  `json:"workBytes"`
}

// TotalBytes is the whole disk this pipeline occupies.
func (r CIPRepoStorage) TotalBytes() int64 {
	return r.CacheBytes + r.RepoBytes + r.LogBytes + r.ArtifactBytes + r.WorkBytes
}

// cipStorage is the GET /storage body.
type cipStorage struct {
	Repos          []CIPRepoStorage `json:"repos"`
	CIPBytes       int64            `json:"cipBytes"`
	DiskFreeBytes  int64            `json:"diskFreeBytes"`
	DiskTotalBytes int64            `json:"diskTotalBytes"`
}

// CIPSnapshot is the stable, provider-neutral subset exposed to callers.
// Unknown fields from the cip daemon are deliberately ignored so its API can
// evolve independently.
type CIPSnapshot struct {
	SchemaVersion int       `json:"schema_version"`
	Name          string    `json:"name"`
	At            time.Time `json:"at"`
	Healthy       bool      `json:"healthy"`
	// Running, Succeeded, and Failed count every run the daemon returned, not
	// only the newest few kept in Runs.
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	// Runs holds the newest runs, newest first.
	Runs           []CIPRun         `json:"runs"`
	Repos          []CIPRepoStorage `json:"repos"`
	CIPBytes       int64            `json:"cip_bytes"`
	DiskFreeBytes  int64            `json:"disk_free_bytes"`
	DiskTotalBytes int64            `json:"disk_total_bytes"`
	Error          string           `json:"error,omitempty"`
}

// DiskFreePercent is the free share of the filesystem that holds cip. It is
// 0 when the daemon reports no total, which a caller must read as unknown.
func (s CIPSnapshot) DiskFreePercent() float64 {
	if s.DiskTotalBytes <= 0 {
		return 0
	}
	return float64(s.DiskFreeBytes) / float64(s.DiskTotalBytes) * 100
}

// SummaryLine is the one-line state of the daemon, for a list of widgets.
func (s CIPSnapshot) SummaryLine() string {
	if s.Error != "" {
		return "error: " + s.Error
	}
	return fmt.Sprintf("running %d  failed %d  success %d  cip %s  disk %s free of %s (%.0f%%)",
		s.Running, s.Failed, s.Succeeded, cipBytes(s.CIPBytes),
		cipBytes(s.DiskFreeBytes), cipBytes(s.DiskTotalBytes), s.DiskFreePercent())
}

// RunLines is one row for each run the snapshot kept, newest first. A failed
// read shows no rows, because an empty table must not look like real data.
func (s CIPSnapshot) RunLines(now time.Time) []string {
	if s.Error != "" {
		return nil
	}
	lines := make([]string, 0, len(s.Runs))
	for _, run := range s.Runs {
		lines = append(lines, run.Line(now))
	}
	return lines
}

// StorageLines is one row for each pipeline, then the daemon total and the
// filesystem. The reader watches these rows because the root partition
// filled up once. A failed read shows no rows, because a zeroed row reads as
// a real measurement.
func (s CIPSnapshot) StorageLines() []string {
	if s.Error != "" {
		return nil
	}
	lines := make([]string, 0, len(s.Repos)+2)
	for _, repo := range s.Repos {
		lines = append(lines, fmt.Sprintf("%-24s %10s", repo.Name, cipBytes(repo.TotalBytes())))
	}
	lines = append(lines, fmt.Sprintf("%-24s %10s", "cip total", cipBytes(s.CIPBytes)))
	lines = append(lines, fmt.Sprintf("%-24s %10s free of %s (%.0f%%)", "filesystem",
		cipBytes(s.DiskFreeBytes), cipBytes(s.DiskTotalBytes), s.DiskFreePercent()))
	return lines
}

// FetchCIP reads only the authenticated GET /runs and GET /storage
// endpoints. It never starts, stops, or cancels a pipeline run.
//
// It fails closed. A missing token, a request error, a non-200 reply, or a
// body it cannot decode all set Error and leave Healthy false, so a fault
// never shows as an empty but healthy panel.
func FetchCIP(ctx context.Context, provider config.Widget, token string) CIPSnapshot {
	result := CIPSnapshot{SchemaVersion: 1, Name: provider.Name, At: time.Now()}
	// Send no request at all without a token. An unauthenticated read would
	// only earn a 401 and would report a confusing reason.
	if strings.TrimSpace(token) == "" {
		result.Error = "no token configured: set token_env or token_file"
		return result
	}
	base := strings.TrimRight(provider.Endpoint, "/")
	var runs []CIPRun
	if err := cipGet(ctx, base+"/runs", token, &runs); err != nil {
		result.Error = err.Error()
		return result
	}
	var storage cipStorage
	if err := cipGet(ctx, base+"/storage", token, &storage); err != nil {
		result.Error = err.Error()
		return result
	}
	result.At = time.Now()
	for i := range runs {
		// The daemon writes an unfinished run as the Go zero time. Mark it as
		// unfinished so no caller renders that as a date.
		runs[i].Finished = !runs[i].FinishedAt.IsZero()
		switch runs[i].Status {
		case "running":
			result.Running++
		case "success":
			result.Succeeded++
		case "failed":
			result.Failed++
		}
	}
	if len(runs) > maxCIPRuns {
		runs = runs[:maxCIPRuns]
	}
	result.Runs = runs
	result.Repos = storage.Repos
	result.CIPBytes = storage.CIPBytes
	result.DiskFreeBytes, result.DiskTotalBytes = storage.DiskFreeBytes, storage.DiskTotalBytes
	// An idle daemon with no runs is healthy. A nearly full disk is not,
	// because that is what stops the next run.
	result.Healthy = result.DiskTotalBytes <= 0 || result.DiskFreePercent() >= cipLowDiskPercent
	return result
}

// cipGet does one authenticated read and decodes the body into out. The
// error text never contains the token, so a log line or a JSON dump of the
// snapshot stays safe to share.
func cipGet(ctx context.Context, url, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// A wrong or missing token gets a 401 or a 403 with no body. Name the
	// token, because that is the fault the reader must fix.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("cip %s: %s — check the widget token", cipPath(url), resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cip %s: %s", cipPath(url), resp.Status)
	}
	// Decode into out only after the status check, so a partial body can
	// never leave a caller holding a half-filled snapshot that reads healthy.
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("cip %s: %w", cipPath(url), err)
	}
	return nil
}

// cipPath is the endpoint name for an error message, for example "/runs".
func cipPath(url string) string {
	if i := strings.LastIndex(url, "/"); i >= 0 {
		return url[i:]
	}
	return url
}

// cipBytes formats a byte count for a reader.
func cipBytes(v int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	f, i := float64(v), 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}
