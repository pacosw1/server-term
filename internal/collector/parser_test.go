package collector

import "testing"

func TestParse(t *testing.T) {
	raw := "hostname\tbox\nos\tDebian\nkernel\t6.1\nuptime\t42.5\ncpu\t100 0 50 800 10 0 0 0\ncore\t25 0 10 200 2\ncores\t4\nload\t0.1 0.2 0.3\nmem_total\t1000\nmem_available\t400\nnet\t123 456\npressure_cpu\t1.25\ndisk\t/dev/sda1\text4\t1000\t250\t/\ndevice\tsda ssd 1000\naccelerator\tGPU\tExample GPU\ttrue\t42\nprocess\t42 root worker 88.5 2.1 1024\nrunners\t8 1 209.5 2.0 2048 12\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.Hostname != "box" || s.Cores != 4 || s.MemTotal != 1024000 || len(s.Disks) != 1 || len(s.Devices) != 1 || len(s.Accelerators) != 1 || s.Accelerators[0].Utilization != 42 || !s.Accelerators[0].UtilizationKnown || len(s.CoreTotal) != 1 || len(s.Processes) != 1 || s.Processes[0].PID != 42 || s.Runners.Listeners != 8 || s.Runners.ActiveJobs != 1 {
		t.Fatalf("unexpected sample: %+v", s)
	}
}
func TestParseRejectsIncomplete(t *testing.T) {
	if _, err := Parse("hostname\tbox\n"); err == nil {
		t.Fatal("expected incomplete error")
	}
}
