package acceleratorprobe

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Reading struct {
	Kind, Name  string
	Utilization float64
	Known       bool
}

type Sampler interface {
	Sample(context.Context, time.Duration) ([]Reading, error)
}

func DefaultSampler() Sampler {
	if runtime.GOOS == "darwin" {
		return DarwinSampler{}
	}
	return LinuxSampler{}
}

func Run(path string, interval time.Duration, sampler Sampler, stop <-chan struct{}) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if sampler == nil {
		sampler = DefaultSampler()
	}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), interval+5*time.Second)
		readings, err := sampler.Sample(ctx, interval)
		cancel()
		if err == nil && len(readings) > 0 {
			if err := writeAtomic(path, readings); err != nil {
				return err
			}
		}
		select {
		case <-stop:
			return nil
		default:
		}
	}
}

func writeAtomic(path string, readings []Reading) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	var data strings.Builder
	for _, r := range readings {
		name := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(r.Name)
		fmt.Fprintf(&data, "accelerator\t%s\t%s\t%t\t%.2f\n", r.Kind, name, r.Known, clamp(r.Utilization))
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".accelerators-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	// The file contains only allowlisted device labels and percentages. 0644
	// lets the deliberately unprivileged agent read it on both Linux and macOS.
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(data.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

type LinuxSampler struct{ Command string }

func (s LinuxSampler) Sample(ctx context.Context, window time.Duration) ([]Reading, error) {
	command := s.Command
	if command == "" {
		command = "/usr/bin/perf"
	}
	seconds := window.Seconds()
	if seconds < .1 {
		seconds = .1
	}
	out, err := exec.CommandContext(ctx, command, "stat", "-x", "\t", "-a", "-e", "i915/software-gt-awake-time/", "sleep", strconv.FormatFloat(seconds, 'f', 3, 64)).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("perf i915 sampler: %w: %s", err, bytes.TrimSpace(out))
	}
	active, err := parsePerfAwake(out)
	if err != nil {
		return nil, err
	}
	return []Reading{{Kind: "GPU", Name: "Intel integrated GPU", Known: true, Utilization: clamp(active)}}, nil
}

func parsePerfAwake(data []byte) (float64, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 4 || strings.TrimSpace(fields[1]) != "ns" || !strings.Contains(fields[2], "software-gt-awake-time") {
			continue
		}
		active, err1 := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
		elapsed, err2 := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
		if err1 == nil && err2 == nil && elapsed > 0 {
			return active / elapsed * 100, nil
		}
	}
	return 0, fmt.Errorf("i915 awake-time counter not found in perf output")
}

type DarwinSampler struct{ Command string }

var residencyPattern = regexp.MustCompile(`(?m)^\s*(GPU|ANE) HW active residency:\s*([0-9]+(?:\.[0-9]+)?)%`)

func (s DarwinSampler) Sample(ctx context.Context, window time.Duration) ([]Reading, error) {
	command := s.Command
	if command == "" {
		command = "/usr/bin/powermetrics"
	}
	ms := window.Milliseconds()
	if ms < 100 {
		ms = 100
	}
	out, err := exec.CommandContext(ctx, command, "-n", "1", "-i", strconv.FormatInt(ms, 10), "-s", "gpu_power,ane_power").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("powermetrics: %w: %s", err, bytes.TrimSpace(out))
	}
	return parsePowermetrics(out)
}

func parsePowermetrics(data []byte) ([]Reading, error) {
	values := map[string]float64{}
	for _, match := range residencyPattern.FindAllSubmatch(data, -1) {
		values[string(match[1])], _ = strconv.ParseFloat(string(match[2]), 64)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("no GPU or ANE active residency in powermetrics output")
	}
	readings := make([]Reading, 0, 2)
	if v, ok := values["GPU"]; ok {
		readings = append(readings, Reading{Kind: "GPU", Name: "Apple integrated GPU", Known: true, Utilization: v})
	} else {
		readings = append(readings, Reading{Kind: "GPU", Name: "Apple integrated GPU"})
	}
	if v, ok := values["ANE"]; ok {
		readings = append(readings, Reading{Kind: "NPU", Name: "Apple Neural Engine", Known: true, Utilization: v})
	} else {
		// On releases which expose only the ANE power rail, zero watts is not
		// proof of zero activity. Preserve detection without inventing a percent.
		readings = append(readings, Reading{Kind: "NPU", Name: "Apple Neural Engine"})
	}
	return readings, nil
}
