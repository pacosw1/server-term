package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed claude-computer-use.py
var computerUseSetup string

const cipSocket = "servterm-gui"

var cipName = regexp.MustCompile(`^cip-gui-(claude|codex)-[a-f0-9]{8}$`)
var remoteSession = regexp.MustCompile(`\[remote-bridge\] Created session cse_([A-Za-z0-9]+)`)
var remoteState = regexp.MustCompile(`\[bridge:repl\] handleStateChange state=([a-z]+)`)
var remoteLink = regexp.MustCompile(`https://claude\.ai/code/session_[A-Za-z0-9_-]+`)

type cipRequest struct {
	Name          string `json:"name"`
	Directory     string `json:"directory"`
	Repo          string `json:"repo"`
	Harness       string `json:"harness"`
	AcceptChannel bool   `json:"acceptChannel"`
}
type cipState struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
}

func cipBinary(name string) string {
	home, _ := os.UserHomeDir()
	for _, dir := range []string{filepath.Join(home, ".local/bin"), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return name
}
func cipQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func cipRoot(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(filepath.Join(home, "code"))
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[2:])
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("folder must be within %s", root)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("project folder is not a directory")
	}
	return resolved, nil
}
func cipLog(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".servterm", "sessions", name+".log")
}
func cipRun(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, cipBinary("tmux"), append([]string{"-L", cipSocket}, args...)...).CombinedOutput()
}
func (s *Server) cipStart(w http.ResponseWriter, r *http.Request) {
	var in cipRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in); err != nil {
		http.Error(w, "Invalid launch request", 400)
		return
	}
	if !cipName.MatchString(in.Name) || (in.Harness != "claude" && in.Harness != "codex") || !strings.HasPrefix(in.Name, "cip-gui-"+in.Harness+"-") || strings.ContainsAny(in.Repo, "\x00\r\n") {
		http.Error(w, "Invalid session name or assistant", 400)
		return
	}
	directory, err := cipRoot(in.Directory)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if _, err := cipRun(ctx, "has-session", "-t", in.Name); err == nil {
		json.NewEncoder(w).Encode(cipState{Name: in.Name, State: "created", Message: "Session already exists"})
		return
	}
	if in.Harness == "claude" {
		authCommand := exec.CommandContext(ctx, cipBinary("claude"), "auth", "status")
		authCommand.Dir = directory
		out, err := authCommand.Output()
		var auth struct {
			LoggedIn   bool   `json:"loggedIn"`
			AuthMethod string `json:"authMethod"`
		}
		_ = json.Unmarshal(out, &auth)
		if err != nil || !auth.LoggedIn || auth.AuthMethod != "claude.ai" {
			http.Error(w, "Claude subscription sign-in is unavailable to the Mac agent. Sign in to Claude Code with your Claude account, then retry.", 409)
			return
		}
	}
	if in.Harness == "claude" {
		setup := exec.CommandContext(ctx, cipBinary("python3"), "-c", computerUseSetup)
		setup.Dir = directory
		if out, err := setup.CombinedOutput(); err != nil {
			http.Error(w, "Computer-use setup failed: "+string(out), 500)
			return
		}
	}
	log := cipLog(in.Name)
	if err := os.MkdirAll(filepath.Dir(log), 0700); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// A folder with no .cip/pipeline.json and no repo named is not a CIP
	// repo: cip would exit at once, so start the assistant itself, still
	// with bypass permissions and Remote Control.
	plain := false
	if in.Repo == "" {
		if _, err := os.Stat(filepath.Join(directory, ".cip", "pipeline.json")); err != nil {
			plain = true
		}
	}
	var args []string
	switch {
	case plain && in.Harness == "claude":
		args = []string{cipBinary("claude"), "--dangerously-skip-permissions", "--chrome", "--remote-control", in.Name, "--debug-file", log}
	case plain:
		args = []string{cipBinary("codex"), "--dangerously-bypass-approvals-and-sandbox"}
	default:
		args = []string{cipBinary("cip"), in.Harness, "--no-worktree", "--permissions", "bypass", "--local-mcp"}
		if in.Repo != "" {
			args = append(args, "--repo", in.Repo)
		}
		if in.Harness == "claude" {
			args = append(args, "--", "--chrome", "--remote-control", in.Name, "--debug-file", log)
		}
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = cipQuote(arg)
	}
	home, _ := os.UserHomeDir()
	command := "umask 077; unset ANTHROPIC_API_KEY ANTHROPIC_AUTH_TOKEN ANTHROPIC_BASE_URL CLAUDE_CODE_USE_BEDROCK CLAUDE_CODE_USE_VERTEX CLAUDE_CODE_USE_FOUNDRY; export PATH=" + cipQuote(filepath.Join(home, ".local/bin")+":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin") + "; " + strings.Join(quoted, " ") + "; result=$?; printf '\\nCIP exited (%s). Press Enter to close.\\n' \"$result\"; read done"
	out, err := cipRun(ctx, "new-session", "-d", "-x", "120", "-y", "40", "-s", in.Name, "-c", directory, "/bin/sh -c "+cipQuote(command))
	if err != nil {
		http.Error(w, "Could not create session: "+string(out), 500)
		return
	}
	if in.Harness == "claude" && in.AcceptChannel && !plain {
		go acceptLocalCIPChannel(in.Name)
	}
	json.NewEncoder(w).Encode(cipState{Name: in.Name, State: "created", Message: "Session created; checking Remote Control"})
}
func readCIPState(name, screen, debug string) cipState {
	state := cipState{Name: name, State: "starting", Message: "Waiting for Claude Remote Control"}
	if strings.Contains(screen, "Select login method:") {
		state.State = "blocked"
		state.Message = "Claude needs sign-in in this session"
		return state
	}
	if strings.Contains(screen, "no MCP server configured with that name") {
		state.State = "blocked"
		state.Message = "CIP channel configuration failed"
		return state
	}
	if strings.Contains(screen, "CIP exited (") {
		state.State = "failed"
		state.Message = "CIP exited. Open the terminal for the error."
		return state
	}
	if strings.Contains(screen, "Remote Control failed") || strings.Contains(screen, "Remote Control disconnected") {
		state.State = "blocked"
		state.Message = "Remote Control is disconnected. Open the terminal for details."
		return state
	}
	if strings.Contains(screen, "WARNING: Loading development channels") {
		state.State = "approval"
		state.Message = "CIP channel approval is waiting in the terminal"
		return state
	}
	if strings.Contains(screen, "trust this") || strings.Contains(screen, "Trust this") {
		state.State = "approval"
		state.Message = "Project trust approval is waiting in the terminal"
		return state
	}
	link := remoteLink.FindString(screen)
	if link == "" {
		link = remoteLink.FindString(debug)
	}
	if link == "" {
		if match := remoteSession.FindStringSubmatch(debug); len(match) > 1 {
			link = "https://claude.ai/code/session_" + match[1]
		}
	}
	if link != "" {
		state.URL = link
		state.State = "linked"
		state.Message = "Remote Control link available"
	}
	if states := remoteState.FindAllStringSubmatch(debug, -1); len(states) > 0 {
		latest := states[len(states)-1][1]
		if latest == "connected" {
			state.State = "connected"
			state.Message = "Remote Control connected"
		} else {
			state.State = "starting"
			state.Message = "Remote Control: " + latest
		}
	}
	if strings.Contains(screen, "/rc active") || strings.Contains(screen, "/remote-control is active") {
		state.State = "connected"
		state.Message = "Remote Control connected"
	}
	if strings.HasPrefix(name, "cip-gui-codex-") {
		state.State = "running"
		state.Message = "Codex terminal started; Claude Remote Control does not apply"
	}
	return state
}
func (s *Server) cipStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !cipName.MatchString(name) {
		http.Error(w, "Invalid session", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	out, err := cipRun(ctx, "capture-pane", "-p", "-S", "-100", "-t", name+":0.0")
	if err != nil {
		http.Error(w, "Session is unavailable", 404)
		return
	}
	var debug []byte
	if file, err := os.Open(cipLog(name)); err == nil {
		defer file.Close()
		if info, err := file.Stat(); err == nil && info.Size() > 262144 {
			_, _ = file.Seek(-262144, io.SeekEnd)
		}
		debug, _ = io.ReadAll(io.LimitReader(file, 262144))
	}
	state := readCIPState(name, string(out), string(debug))
	// Keep the verified link after the terminal scrollback and debug tail advance.
	linkFile := cipLog(name) + ".url"
	if state.URL != "" {
		_ = os.WriteFile(linkFile, []byte(state.URL), 0600)
	} else if state.State == "connected" || state.State == "starting" {
		if cached, err := os.ReadFile(linkFile); err == nil {
			state.URL = remoteLink.FindString(string(cached))
		}
	}
	json.NewEncoder(w).Encode(state)
}

// This assistance runs locally before the user needs to attach over SSH.
// Only CIP's exact local-channel approval is eligible.
func acceptLocalCIPChannel(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		out, err := cipRun(ctx, "capture-pane", "-p", "-t", name+":0.0")
		if err != nil {
			return
		}
		if localCIPPrompt(string(out)) {
			_, _ = cipRun(ctx, "send-keys", "-t", name+":0.0", "Enter")
			return
		}
		if strings.Contains(string(out), "CIP exited (") {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
func localCIPPrompt(screen string) bool {
	expected := map[string]bool{"WARNING: Loading development channels": false, "Channels: server:cip": false, "❯ 1. I am using this for local development": false, "2. Exit": false}
	for _, line := range strings.Split(screen, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "plugin:") || (strings.Contains(line, "server:") && line != "Channels: server:cip") {
			return false
		}
		if _, ok := expected[line]; ok {
			expected[line] = true
		}
	}
	for _, found := range expected {
		if !found {
			return false
		}
	}
	return true
}
