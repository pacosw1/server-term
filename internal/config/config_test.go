package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "config.yaml")
	if err := os.WriteFile(p, []byte("servers:\n  - name: local\n    address: localhost\n    transport: local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.RefreshInterval.String() != "3s" || c.HistorySize != 60 {
		t.Fatalf("defaults not applied: %+v", c)
	}
}

func TestRejectsUnknownAndDuplicate(t *testing.T) {
	for _, body := range []string{
		"wat: true\nservers:\n - name: a\n   address: x\n",
		"servers:\n - name: a\n   address: x\n - name: a\n   address: y\n",
	} {
		p := filepath.Join(t.TempDir(), "c.yaml")
		_ = os.WriteFile(p, []byte(body), 0600)
		if _, err := Load(p); err == nil {
			t.Fatalf("expected error for %s", strings.TrimSpace(body))
		}
	}
}
