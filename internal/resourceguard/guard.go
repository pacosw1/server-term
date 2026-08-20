package resourceguard

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	minPercent       = 60.0
	maxMemoryPercent = 85.0
	maxCPUPercent    = 90.0
)

type Observation struct {
	AvailablePercent float64
	MemorySomeAvg10  float64
	MemoryFullAvg10  float64
	CPUUtilization   float64
	CPUSomeAvg10     float64
}

type Limits struct {
	MemoryPercent float64
	CPUPercent    float64
}

// NextLimits backs off promptly under pressure and recovers only on a calm
// recovery tick. Hard enforcement remains the responsibility of the slice.
func NextLimits(current Limits, o Observation, recover bool) Limits {
	if current.MemoryPercent == 0 {
		current.MemoryPercent = maxMemoryPercent
	}
	if current.CPUPercent == 0 {
		current.CPUPercent = maxCPUPercent
	}

	if o.AvailablePercent < 10 || o.MemorySomeAvg10 >= 10 || o.MemoryFullAvg10 >= 2 {
		current.MemoryPercent -= 2
	} else if recover && o.AvailablePercent > 20 && o.MemorySomeAvg10 < 2 && o.MemoryFullAvg10 < 0.5 {
		current.MemoryPercent += 0.5
	}
	if o.CPUSomeAvg10 >= 20 || o.CPUUtilization >= 95 {
		current.CPUPercent -= 5
	} else if recover && o.CPUSomeAvg10 < 5 && o.CPUUtilization < 80 {
		current.CPUPercent++
	}

	current.MemoryPercent = clamp(current.MemoryPercent, minPercent, maxMemoryPercent)
	current.CPUPercent = clamp(current.CPUPercent, minPercent, maxCPUPercent)
	return current
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}

type cpuSample struct{ busy, total uint64 }

func Run(ctx context.Context, cgroup string, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("interval must be positive")
	}
	totalMemory, err := memoryTotal()
	if err != nil {
		return err
	}
	cores := float64(runtimeCores())
	if cores < 1 {
		return errors.New("could not determine logical CPU count")
	}
	previousCPU, err := readCPU()
	if err != nil {
		return err
	}
	limits := Limits{MemoryPercent: maxMemoryPercent, CPUPercent: maxCPUPercent}
	if err := writeLimits(cgroup, totalMemory, cores, limits); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ticks := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			observation, currentCPU, err := observe(totalMemory, previousCPU)
			if err != nil {
				slog.Warn("resource observation failed; retaining limits", "error", err)
				continue
			}
			previousCPU = currentCPU
			ticks++
			next := NextLimits(limits, observation, ticks%6 == 0)
			changed := next != limits
			if changed {
				if err := writeLimits(cgroup, totalMemory, cores, next); err != nil {
					slog.Error("could not update runner limits; retaining last known limits", "error", err)
					continue
				}
				limits = next
			}
			if changed || ticks%6 == 0 {
				slog.Info("runner resource boundary", "memory_high_percent", limits.MemoryPercent, "cpu_quota_percent", limits.CPUPercent, "memory_available_percent", observation.AvailablePercent, "memory_psi_some", observation.MemorySomeAvg10, "memory_psi_full", observation.MemoryFullAvg10, "cpu_utilization", observation.CPUUtilization, "cpu_psi_some", observation.CPUSomeAvg10)
			}
		}
	}
}

func observe(totalMemory uint64, previous cpuSample) (Observation, cpuSample, error) {
	available, err := memoryAvailable()
	if err != nil {
		return Observation{}, cpuSample{}, err
	}
	memorySome, memoryFull, err := readPSI("/proc/pressure/memory")
	if err != nil {
		return Observation{}, cpuSample{}, err
	}
	cpuSome, _, err := readPSI("/proc/pressure/cpu")
	if err != nil {
		return Observation{}, cpuSample{}, err
	}
	current, err := readCPU()
	if err != nil {
		return Observation{}, cpuSample{}, err
	}
	deltaTotal := current.total - previous.total
	deltaBusy := current.busy - previous.busy
	utilization := 0.0
	if deltaTotal > 0 {
		utilization = 100 * float64(deltaBusy) / float64(deltaTotal)
	}
	return Observation{100 * float64(available) / float64(totalMemory), memorySome, memoryFull, utilization, cpuSome}, current, nil
}

func writeLimits(cgroup string, totalMemory uint64, cores float64, limits Limits) error {
	memoryHigh := uint64(float64(totalMemory) * limits.MemoryPercent / 100)
	quota := uint64(cores * limits.CPUPercent / 100 * 100000)
	if err := os.WriteFile(filepath.Join(cgroup, "memory.high"), []byte(strconv.FormatUint(memoryHigh, 10)), 0); err != nil {
		return fmt.Errorf("write memory.high: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cgroup, "cpu.max"), []byte(fmt.Sprintf("%d 100000", quota)), 0); err != nil {
		return fmt.Errorf("write cpu.max: %w", err)
	}
	return nil
}

func memoryTotal() (uint64, error)     { return meminfoValue("MemTotal") }
func memoryAvailable() (uint64, error) { return meminfoValue("MemAvailable") }

func meminfoValue(key string) (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.TrimSuffix(fields[0], ":") == key {
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			return kb * 1024, err
		}
	}
	return 0, fmt.Errorf("%s not found in /proc/meminfo", key)
}

func readPSI(path string) (some, full float64, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		value := 0.0
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "avg10=") {
				value, err = strconv.ParseFloat(strings.TrimPrefix(field, "avg10="), 64)
				if err != nil {
					return 0, 0, err
				}
			}
		}
		switch fields[0] {
		case "some":
			some = value
		case "full":
			full = value
		}
	}
	return some, full, nil
}

func readCPU() (cpuSample, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}, err
	}
	defer f.Close()
	var label string
	values := make([]uint64, 10)
	if _, err := fmt.Fscan(f, &label, &values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7], &values[8], &values[9]); err != nil {
		return cpuSample{}, err
	}
	if label != "cpu" {
		return cpuSample{}, errors.New("aggregate cpu line missing from /proc/stat")
	}
	total := uint64(0)
	for _, value := range values {
		total += value
	}
	idle := values[3] + values[4]
	return cpuSample{busy: total - idle, total: total}, nil
}

func runtimeCores() int {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if len(line) > 3 && strings.HasPrefix(line, "cpu") && line[3] >= '0' && line[3] <= '9' {
			count++
		}
	}
	return count
}
