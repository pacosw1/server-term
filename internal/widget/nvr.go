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

// NVRSnapshot is the stable, provider-neutral subset exposed to callers.
// Unknown fields from nvrd are deliberately ignored so its richer stats API
// can evolve independently.
type NVRSnapshot struct {
	SchemaVersion int       `json:"schema_version"`
	Name          string    `json:"name"`
	At            time.Time `json:"at"`
	Healthy       bool      `json:"healthy"`
	LiveStreams   int       `json:"live_streams"`
	TotalStreams  int       `json:"total_streams"`
	CPUPercent    float64   `json:"cpu_percent"`
	RSSBytes      int64     `json:"rss_bytes"`
	DiskTotal     int64     `json:"disk_total_bytes"`
	DiskFree      int64     `json:"disk_free_bytes"`
	StorageBytes  int64     `json:"storage_bytes"`
	OldestMS      int64     `json:"oldest_segment_ms"`
	DropsPerSec   int64     `json:"drops_per_sec"`
	Error         string    `json:"error,omitempty"`
}

type nvrStats struct {
	Streams []struct {
		Live        bool  `json:"live"`
		DropsPerSec int64 `json:"drops_per_sec"`
	} `json:"streams"`
	Storage struct {
		OldestSegmentMS int64 `json:"OldestSegmentMS"`
		TotalBytes      int64 `json:"TotalBytes"`
	} `json:"storage"`
	Disk struct {
		TotalBytes int64 `json:"TotalBytes"`
		FreeBytes  int64 `json:"FreeBytes"`
	} `json:"disk"`
	Process struct {
		// nvrd leaves these two untagged, so they marshal as Go field names.
		// Go's case-insensitive fallback does not bridge "CPUPercent" to a
		// "cpu_percent" tag, so the keys must match exactly or both read zero.
		CPUPercent float64 `json:"CPUPercent"`
		RSSBytes   int64   `json:"RSSBytes"`

		// nvrd forks an ffmpeg per thumbnail, motion sample and snapshot job.
		// Its own CPU therefore understates it badly -- measured at 141% while
		// the tree was 394% -- so these totals are preferred when present.
		TotalCPUPercent float64 `json:"total_cpu_percent"`
		TotalRSSBytes   int64   `json:"total_rss_bytes"`
	} `json:"process"`
}

// FetchNVR reads only the authenticated GET /api/stats endpoint. It never
// invokes NVR actions or accepts arbitrary plugin code.
func FetchNVR(ctx context.Context, provider config.Widget, token string) NVRSnapshot {
	result := NVRSnapshot{SchemaVersion: 1, Name: provider.Name, At: time.Now()}
	base := strings.TrimRight(provider.Endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/stats", nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("nvr stats: %s", resp.Status)
		return result
	}
	var stats nvrStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		result.Error = err.Error()
		return result
	}
	result.At = time.Now()
	result.TotalStreams = len(stats.Streams)
	for _, stream := range stats.Streams {
		if stream.Live {
			result.LiveStreams++
		}
		result.DropsPerSec += stream.DropsPerSec
	}
	result.Healthy = result.TotalStreams == 0 || result.LiveStreams > 0
	// Prefer the whole process tree; fall back to the daemon's own figures
	// for an older nvrd that does not report a total, so the widget shows
	// real usage rather than a misleading zero.
	result.CPUPercent, result.RSSBytes = stats.Process.CPUPercent, stats.Process.RSSBytes
	if stats.Process.TotalCPUPercent > 0 {
		result.CPUPercent = stats.Process.TotalCPUPercent
	}
	if stats.Process.TotalRSSBytes > 0 {
		result.RSSBytes = stats.Process.TotalRSSBytes
	}
	result.DiskTotal, result.DiskFree = stats.Disk.TotalBytes, stats.Disk.FreeBytes
	result.StorageBytes, result.OldestMS = stats.Storage.TotalBytes, stats.Storage.OldestSegmentMS
	return result
}
