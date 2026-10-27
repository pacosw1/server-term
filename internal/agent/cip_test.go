package agent

import (
	"net/http/httptest"
	"testing"
)

func TestCIPStatus(t *testing.T) {
	for _, test := range []struct{ screen, debug, state string }{
		{"Select login method:", "https://claude.ai/code/session_old", "blocked"},
		{"CIP exited (1)", "", "failed"},
		{"WARNING: Loading development channels", "", "approval"},
		{"/rc active", "https://claude.ai/code/session_test", "connected"},
		{"", "[remote-bridge] Created session cse_abc123\n[bridge:repl] handleStateChange state=connected", "connected"},
		{"", "[bridge:repl] handleStateChange state=connected\n[bridge:repl] handleStateChange state=reconnecting", "starting"},
		{"", "https://claude.ai/code/session_test", "linked"},
		{"Remote Control disconnected", "https://claude.ai/code/session_old", "blocked"},
	} {
		if got := readCIPState("cip-gui-claude-12345678", test.screen, test.debug); got.State != test.state {
			t.Errorf("%q: got %s want %s", test.screen, got.State, test.state)
		}
	}
}
func TestCIPRequiresAuthentication(t *testing.T) {
	s := NewServer("test", "secret", nil, nil)
	s.DesktopEnabled = true
	for _, method := range []string{"POST", "GET"} {
		path := "/v1/cip/sessions"
		if method == "GET" {
			path += "/cip-gui-claude-12345678"
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
}

func TestLocalCIPApproval(t *testing.T) {
	prompt := "WARNING: Loading development channels\nChannels: server:cip\n❯ 1. I am using this for local development\n2. Exit"
	if !localCIPPrompt(prompt) {
		t.Fatal("exact local CIP prompt was refused")
	}
	for _, extra := range []string{"\nserver:other", "\nplugin:other"} {
		if localCIPPrompt(prompt + extra) {
			t.Fatal("unexpected channel was accepted")
		}
	}
	if localCIPPrompt("Do you trust this folder?") {
		t.Fatal("unrelated prompt accepted")
	}
}
