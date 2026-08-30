package collector

import (
	"os/exec"
	"strings"
	"testing"
)

// The collector's exit status decides whether a host shows as online, so the
// script must exit 0 whenever collection actually succeeded.
//
// Regression: the final statement used to be
//
//	[ -r /run/servterm/runner-jobs.jsonl ] && while ... done < ...
//
// On a host without that optional CI-runner file the && list evaluates false.
// Being the last statement, its status of 1 became the script's status, so a
// healthy non-runner server was reported offline with an empty error.
func TestLinuxScriptLastStatementExitsZeroWhenRunnerJobsFileAbsent(t *testing.T) {
	lines := strings.Split(strings.TrimRight(linuxScript, "\n"), "\n")
	last := lines[len(lines)-1]

	if !strings.Contains(last, "runner-jobs.jsonl") {
		t.Fatalf("last statement is no longer the runner-jobs probe; update this test: %q", last)
	}

	// Run it standalone under the same flags the real script uses, against a
	// path that does not exist, mirroring any non-CI-runner host.
	script := "set -eu\n" + strings.ReplaceAll(last, "/run/servterm/runner-jobs.jsonl", "/nonexistent/runner-jobs.jsonl")
	cmd := exec.Command("sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	_ = cmd.Run()

	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("final statement exited %d when the runner-jobs file is absent; want 0.\nstatement: %s", code, last)
	}
}
