package collector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
	"strconv"
	"strings"
	"time"
)

func Parse(raw string) (metrics.Sample, error) {
	s := metrics.Sample{At: time.Now(), Online: true}
	scan := bufio.NewScanner(strings.NewReader(raw))
	seenCPU := false
	for scan.Scan() {
		line := scan.Text()
		key, val, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		switch key {
		case "hostname":
			s.Hostname = val
		case "os":
			s.OS = val
		case "kernel":
			s.Kernel = val
		case "uptime":
			s.UptimeSeconds = f64(val)
		case "cpu":
			p := fieldsU64(val)
			if len(p) >= 5 {
				for _, v := range p {
					s.CPUTotal += v
				}
				s.CPUIdle = p[3] + p[4]
				seenCPU = true
			}
		case "cpu_percent":
			s.CPUPercent = f64(val)
			seenCPU = true
		case "core":
			p := fieldsU64(val)
			if len(p) >= 5 {
				var total uint64
				for _, v := range p {
					total += v
				}
				s.CoreTotal = append(s.CoreTotal, total)
				s.CoreIdle = append(s.CoreIdle, p[3]+p[4])
			}
		case "cores":
			s.Cores = int(u64(val))
		case "load":
			p := strings.Fields(val)
			if len(p) >= 3 {
				s.Load1, s.Load5, s.Load15 = f64(p[0]), f64(p[1]), f64(p[2])
			}
		case "mem_total":
			s.MemTotal = u64(val) * 1024
		case "mem_total_bytes":
			s.MemTotal = u64(val)
		case "mem_available":
			s.MemAvailable = u64(val) * 1024
		case "mem_available_bytes":
			s.MemAvailable = u64(val)
		case "swap_total":
			s.SwapTotal = u64(val) * 1024
		case "swap_total_bytes":
			s.SwapTotal = u64(val)
		case "swap_free":
			s.SwapFree = u64(val) * 1024
		case "swap_free_bytes":
			s.SwapFree = u64(val)
		case "net":
			p := strings.Fields(val)
			if len(p) == 2 {
				s.NetRx, s.NetTx = u64(p[0]), u64(p[1])
			}
		case "net_info":
			p := strings.Fields(val)
			if len(p) >= 3 {
				s.NetworkInterface, s.NetworkType, s.NetworkLinkMbps = p[0], p[1], int(u64(p[2]))
			}
		case "net_errors":
			p := fieldsU64(val)
			if len(p) >= 4 {
				s.NetRxErrors, s.NetTxErrors, s.NetRxDrops, s.NetTxDrops = p[0], p[1], p[2], p[3]
			}
		case "energy_uj":
			s.EnergyMicrojoules = u64(val)
		case "power_watts":
			s.PowerWatts = f64(val)
			s.PowerKnown = s.PowerWatts >= 0
		case "battery_percent":
			s.BatteryPercent = f64(val)
			s.BatteryKnown = true
		case "battery_charging":
			s.BatteryCharging = strings.TrimSpace(val) == "true"
		case "pressure_cpu":
			s.PressureCPU = f64(val)
		case "pressure_memory":
			s.PressureMemory = f64(val)
		case "pressure_io":
			s.PressureIO = f64(val)
		case "disk":
			p := strings.Split(val, "\t")
			if len(p) >= 5 {
				s.Disks = append(s.Disks, metrics.Disk{Device: p[0], FSType: p[1], Total: u64(p[2]), Used: u64(p[3]), Mount: p[4]})
			}
		case "device":
			p := strings.Fields(val)
			if len(p) >= 3 {
				s.Devices = append(s.Devices, metrics.BlockDevice{Name: p[0], Kind: p[1], Size: u64(p[2])})
			}
		case "accelerator":
			p := strings.Split(val, "\t")
			if len(p) >= 4 {
				s.Accelerators = append(s.Accelerators, metrics.Accelerator{Kind: p[0], Name: p[1], UtilizationKnown: p[2] == "true", Utilization: f64(p[3])})
			}
		case "process":
			p := strings.Fields(val)
			if len(p) >= 6 {
				s.Processes = append(s.Processes, metrics.Process{PID: int(u64(p[0])), User: p[1], Command: p[2], CPU: f64(p[3]), Memory: f64(p[4]), RSS: u64(p[5]) * 1024})
			}
		case "runners":
			p := strings.Fields(val)
			if len(p) >= 6 {
				s.Runners = metrics.RunnerStats{Listeners: int(u64(p[0])), ActiveJobs: int(u64(p[1])), CPU: f64(p[2]), Memory: f64(p[3]), RSS: u64(p[4]) * 1024, Processes: int(u64(p[5]))}
			}
		case "runner_ticks":
			s.Runners.CPUTicks = u64(val)
		case "runner_job":
			var job metrics.RunnerJob
			if json.Unmarshal([]byte(val), &job) == nil {
				s.RunnerJobs = append(s.RunnerJobs, job)
			}
		}
	}
	if err := scan.Err(); err != nil {
		return s, err
	}
	if !seenCPU || s.MemTotal == 0 {
		return s, fmt.Errorf("host returned incomplete metrics")
	}
	return s, nil
}
func u64(v string) uint64  { n, _ := strconv.ParseUint(strings.TrimSpace(v), 10, 64); return n }
func f64(v string) float64 { n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64); return n }
func fieldsU64(v string) []uint64 {
	f := strings.Fields(v)
	out := make([]uint64, 0, len(f))
	for _, x := range f {
		out = append(out, u64(x))
	}
	return out
}
