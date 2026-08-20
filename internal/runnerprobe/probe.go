package runnerprobe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/metrics"
)

type proc struct {
	pid, ppid  int
	name       string
	ticks, rss uint64
	started    time.Time
}

var allowed = map[string]bool{"RUNNER_NAME": true, "GITHUB_REPOSITORY": true, "GITHUB_WORKFLOW": true, "GITHUB_JOB": true, "GITHUB_RUN_ID": true, "GITHUB_RUN_NUMBER": true, "GITHUB_SERVER_URL": true}

func Snapshot() ([]metrics.RunnerJob, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	procs := map[int]proc{}
	boot := bootTime()
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		p, err := readProc(pid, boot)
		if err == nil {
			procs[pid] = p
		}
	}
	children := map[int][]int{}
	for pid, p := range procs {
		children[p.ppid] = append(children[p.ppid], pid)
	}
	out := []metrics.RunnerJob{}
	for pid, p := range procs {
		if p.name != "Runner.Worker" {
			continue
		}
		ids := descendants(pid, children)
		ids = append(ids, pid)
		fields := map[string]string{}
		var ticks, rss uint64
		started := p.started
		for _, id := range ids {
			q := procs[id]
			ticks += q.ticks
			rss += q.rss
			for k, v := range safeEnv(id) {
				if fields[k] == "" {
					fields[k] = v
				}
			}
		}
		job := metrics.RunnerJob{WorkerPID: pid, Runner: fields["RUNNER_NAME"], Repository: fields["GITHUB_REPOSITORY"], Workflow: fields["GITHUB_WORKFLOW"], Job: fields["GITHUB_JOB"], RunID: fields["GITHUB_RUN_ID"], RunNumber: fields["GITHUB_RUN_NUMBER"], ServerURL: fields["GITHUB_SERVER_URL"], StartedAt: started, CPUTicks: ticks, RSS: rss, Processes: len(ids)}
		out = append(out, job)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Runner < out[j].Runner })
	return out, nil
}
func Write(path string, jobs []metrics.RunnerJob) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".runner-jobs-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	enc := json.NewEncoder(tmp)
	for _, job := range jobs {
		if err := enc.Encode(job); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Chmod(0640); err != nil {
		tmp.Close()
		return err
	}
	if group, err := user.LookupGroup("servterm"); err == nil {
		gid, _ := strconv.Atoi(group.Gid)
		_ = tmp.Chown(0, gid)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func readProc(pid int, boot time.Time) (proc, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return proc{}, err
	}
	end := bytes.LastIndexByte(b, ')')
	if end < 0 {
		return proc{}, errors.New("invalid stat")
	}
	start := bytes.IndexByte(b, '(')
	rest := strings.Fields(string(b[end+1:]))
	if start < 0 || len(rest) < 22 {
		return proc{}, errors.New("short stat")
	}
	ppid, _ := strconv.Atoi(rest[1])
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	startTicks, _ := strconv.ParseUint(rest[19], 10, 64)
	rssPages, _ := strconv.ParseUint(rest[21], 10, 64)
	return proc{pid: pid, ppid: ppid, name: string(b[start+1 : end]), ticks: utime + stime, rss: rssPages * uint64(os.Getpagesize()), started: boot.Add(time.Duration(startTicks) * time.Second / 100)}, nil
}
func safeEnv(pid int) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return out
	}
	for _, item := range bytes.Split(b, []byte{0}) {
		k, v, ok := bytes.Cut(item, []byte{'='})
		if ok && allowed[string(k)] {
			out[string(k)] = string(v)
		}
	}
	return out
}
func descendants(root int, children map[int][]int) []int {
	out := []int{}
	queue := append([]int(nil), children[root]...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		out = append(out, id)
		queue = append(queue, children[id]...)
	}
	return out
}
func bootTime() time.Time {
	f, err := os.Open("/proc/stat")
	if err == nil {
		defer f.Close()
		scan := bufio.NewScanner(f)
		for scan.Scan() {
			var sec int64
			if _, err := fmt.Sscanf(scan.Text(), "btime %d", &sec); err == nil {
				return time.Unix(sec, 0)
			}
		}
	}
	return time.Now()
}

func Run(path string, interval time.Duration, stop <-chan struct{}) error {
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		jobs, err := Snapshot()
		if err != nil {
			return err
		}
		if err := Write(path, jobs); err != nil {
			return err
		}
		select {
		case <-stop:
			return nil
		case <-ticker.C:
		}
	}
}
