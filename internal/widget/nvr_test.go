package widget

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

func TestFetchNVRNormalizesStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stats" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected request: %s %s", r.URL, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"streams":[{"live":true,"drops_per_sec":2},{"live":false,"drops_per_sec":3}],"storage":{"OldestSegmentMS":12,"TotalBytes":99},"disk":{"TotalBytes":1000,"FreeBytes":400},"process":{"CPUPercent":12.5,"RSSBytes":77}}`))
	}))
	defer srv.Close()
	got := FetchNVR(context.Background(), config.Widget{Name: "nvr", Type: "nvr", Endpoint: srv.URL}, "secret")
	if got.Error != "" || !got.Healthy || got.LiveStreams != 1 || got.TotalStreams != 2 || got.DropsPerSec != 5 || got.DiskFree != 400 || got.StorageBytes != 99 || got.CPUPercent != 12.5 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}


// nvrd reports its OWN cpu under CPUPercent and the cost of its ffmpeg
// children separately, with total_cpu_percent covering both. The daemon's own
// figure badly understates it -- an ffmpeg is forked per thumbnail, motion
// sample and snapshot job -- so the widget must prefer the total.
func TestFetchNVRPrefersTheProcessTreeTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"streams":[{"live":true}],"process":{"CPUPercent":141.7,"RSSBytes":100,"total_cpu_percent":394.0,"total_rss_bytes":300}}`))
	}))
	defer srv.Close()
	got := FetchNVR(context.Background(), config.Widget{Name: "nvr", Endpoint: srv.URL}, "t")
	if got.CPUPercent != 394.0 {
		t.Errorf("CPUPercent = %v, want the 394.0 total, not the daemon's own 141.7", got.CPUPercent)
	}
	if got.RSSBytes != 300 {
		t.Errorf("RSSBytes = %v, want the 300 total", got.RSSBytes)
	}
}

// An older daemon that reports no total must still show its own usage rather
// than a misleading zero.
func TestFetchNVRFallsBackToTheDaemonsOwnUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"streams":[{"live":true}],"process":{"CPUPercent":141.7,"RSSBytes":100}}`))
	}))
	defer srv.Close()
	got := FetchNVR(context.Background(), config.Widget{Name: "nvr", Endpoint: srv.URL}, "t")
	if got.CPUPercent != 141.7 || got.RSSBytes != 100 {
		t.Errorf("cpu=%v rss=%v, want the daemon's own 141.7/100", got.CPUPercent, got.RSSBytes)
	}
}
