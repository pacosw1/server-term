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
		_, _ = w.Write([]byte(`{"streams":[{"live":true,"drops_per_sec":2},{"live":false,"drops_per_sec":3}],"storage":{"OldestSegmentMS":12,"TotalBytes":99},"disk":{"TotalBytes":1000,"FreeBytes":400},"process":{"cpu_percent":12.5,"rss_bytes":77}}`))
	}))
	defer srv.Close()
	got := FetchNVR(context.Background(), config.Widget{Name: "nvr", Type: "nvr", Endpoint: srv.URL}, "secret")
	if got.Error != "" || !got.Healthy || got.LiveStreams != 1 || got.TotalStreams != 2 || got.DropsPerSec != 5 || got.DiskFree != 400 || got.StorageBytes != 99 || got.CPUPercent != 12.5 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}
