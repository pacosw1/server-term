package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTokenFileOverridesEnvironment(t *testing.T) {
	t.Setenv("SERVTERM_AGENT_TOKEN", "environment-token")
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("file-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	token, err := LoadToken(path)
	if err != nil || token != "file-token" {
		t.Fatalf("token=%q err=%v", token, err)
	}
}

func TestLoadTokenRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadToken(path); err == nil {
		t.Fatal("expected empty token file error")
	}
}
