package metrics

import "time"

type Disk struct {
	Mount, Device, FSType string
	Total, Used           uint64
}
type BlockDevice struct {
	Name, Kind string
	Size       uint64
}
type Accelerator struct {
	Kind, Name       string
	Utilization      float64
	UtilizationKnown bool
}

// Temperature is one thermal sensor reading. Label names the sensor as the
// host names it, because only the host knows what the sensor measures.
type Temperature struct {
	Label   string
	Celsius float64
}

// DiskIO is the byte counter of one block device. ReadRate and WriteRate
// are bytes per second, derived from two samples; they stay at zero until a
// second sample exists, so a first reading never shows invented traffic.
type DiskIO struct {
	Device                string
	ReadBytes, WriteBytes uint64
	ReadRate, WriteRate   float64
}

// NetInterface is one network interface. The counters are the host's own
// totals; the rates are derived the same way as DiskIO.
type NetInterface struct {
	Name                                 string
	Rx, Tx                               uint64
	RxRate, TxRate                       float64
	RxErrors, TxErrors, RxDrops, TxDrops uint64
}

type Process struct {
	PID     int
	User    string
	Command string
	CPU     float64
	Memory  float64
	RSS     uint64
}
type RunnerStats struct {
	Listeners  int
	ActiveJobs int
	CPU        float64
	Memory     float64
	RSS        uint64
	Processes  int
	CPUTicks   uint64
}
type RunnerJob struct {
	WorkerPID                                                      int `json:"worker_pid"`
	Runner, Repository, Workflow, Job, RunID, RunNumber, ServerURL string
	StartedAt                                                      time.Time
	CPUTicks, RSS                                                  uint64
	Processes                                                      int
	CPU                                                            float64
}
type Sample struct {
	At                                               time.Time
	Online                                           bool
	Error                                            string
	Latency                                          time.Duration
	Hostname, OS, Kernel                             string
	UptimeSeconds                                    float64
	CPUTotal, CPUIdle                                uint64
	CPUPercent                                       float64
	CoreTotal, CoreIdle                              []uint64
	CorePercent                                      []float64
	Cores                                            int
	Load1, Load5, Load15                             float64
	MemTotal, MemAvailable, SwapTotal, SwapFree      uint64
	NetRx, NetTx                                     uint64
	NetRxRate, NetTxRate                             float64
	NetworkInterface, NetworkType                    string
	NetworkLinkMbps                                  int
	NetRxErrors, NetTxErrors, NetRxDrops, NetTxDrops uint64
	// EnergyMicrojoules is a monotonically increasing platform energy counter
	// (when available). PowerWatts is either a direct platform reading or a
	// value derived from two energy samples.
	EnergyMicrojoules                       uint64
	PowerWatts                              float64
	PowerKnown                              bool
	BatteryPercent                          float64
	BatteryKnown, BatteryCharging           bool
	PressureCPU, PressureMemory, PressureIO float64
	Disks                                   []Disk
	Devices                                 []BlockDevice
	Accelerators                            []Accelerator
	Processes                               []Process
	Temperatures                            []Temperature
	DiskIO                                  []DiskIO
	Interfaces                              []NetInterface
	Runners                                 RunnerStats
	RunnerJobs                              []RunnerJob
}

// WireSample is the versioned payload exchanged between agents and the TUI.
// Keeping the envelope versioned lets newer clients safely detect incompatible agents.
type WireSample struct {
	Version int    `json:"version"`
	NodeID  string `json:"node_id"`
	Sample  Sample `json:"sample"`
}

func Percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}
func Derive(previous *Sample, current *Sample) {
	if previous == nil || !previous.Online {
		return
	}
	dt := current.At.Sub(previous.At).Seconds()
	if dt <= 0 {
		return
	}
	if current.CPUTotal >= previous.CPUTotal && current.CPUIdle >= previous.CPUIdle {
		total := current.CPUTotal - previous.CPUTotal
		idle := current.CPUIdle - previous.CPUIdle
		if total > 0 && idle <= total {
			current.CPUPercent = float64(total-idle) / float64(total) * 100
			if current.Runners.CPUTicks >= previous.Runners.CPUTicks && current.Cores > 0 {
				delta := current.Runners.CPUTicks - previous.Runners.CPUTicks
				current.Runners.CPU = float64(delta) / float64(total) * 100 * float64(current.Cores)
			}
		}
	}
	if len(current.CoreTotal) == len(previous.CoreTotal) {
		current.CorePercent = make([]float64, len(current.CoreTotal))
		for i := range current.CoreTotal {
			if current.CoreTotal[i] < previous.CoreTotal[i] || current.CoreIdle[i] < previous.CoreIdle[i] {
				continue
			}
			total := current.CoreTotal[i] - previous.CoreTotal[i]
			idle := current.CoreIdle[i] - previous.CoreIdle[i]
			if total > 0 && idle <= total {
				current.CorePercent[i] = float64(total-idle) / float64(total) * 100
			}
		}
	}
	if current.NetRx >= previous.NetRx {
		current.NetRxRate = float64(current.NetRx-previous.NetRx) / dt
	}
	if current.NetTx >= previous.NetTx {
		current.NetTxRate = float64(current.NetTx-previous.NetTx) / dt
	}
	// A device is matched by its own name, because the host can add or drop
	// a device between two samples. A device with no earlier reading, and a
	// counter that went backwards after a restart, both keep the rate at
	// zero rather than showing invented traffic.
	oldDiskIO := map[string]DiskIO{}
	for _, d := range previous.DiskIO {
		oldDiskIO[d.Device] = d
	}
	for i := range current.DiskIO {
		d := &current.DiskIO[i]
		old, ok := oldDiskIO[d.Device]
		if !ok {
			continue
		}
		if d.ReadBytes >= old.ReadBytes {
			d.ReadRate = float64(d.ReadBytes-old.ReadBytes) / dt
		}
		if d.WriteBytes >= old.WriteBytes {
			d.WriteRate = float64(d.WriteBytes-old.WriteBytes) / dt
		}
	}
	oldInterfaces := map[string]NetInterface{}
	for _, n := range previous.Interfaces {
		oldInterfaces[n.Name] = n
	}
	for i := range current.Interfaces {
		n := &current.Interfaces[i]
		old, ok := oldInterfaces[n.Name]
		if !ok {
			continue
		}
		if n.Rx >= old.Rx {
			n.RxRate = float64(n.Rx-old.Rx) / dt
		}
		if n.Tx >= old.Tx {
			n.TxRate = float64(n.Tx-old.Tx) / dt
		}
	}
	if current.EnergyMicrojoules >= previous.EnergyMicrojoules {
		delta := current.EnergyMicrojoules - previous.EnergyMicrojoules
		if delta > 0 {
			current.PowerWatts = float64(delta) / 1_000_000 / dt
			current.PowerKnown = true
		}
	}
	oldJobs := map[int]RunnerJob{}
	for _, job := range previous.RunnerJobs {
		oldJobs[job.WorkerPID] = job
	}
	for i := range current.RunnerJobs {
		job := &current.RunnerJobs[i]
		old, ok := oldJobs[job.WorkerPID]
		if ok && job.CPUTicks >= old.CPUTicks {
			job.CPU = float64(job.CPUTicks-old.CPUTicks) / dt
		}
	}
}
