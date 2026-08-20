package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/franciscosainzwilliams/server-term/internal/agent"
)

func TestOpenStoreProtectsDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := agent.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("database mode = %04o, want 0600", got)
	}
}
