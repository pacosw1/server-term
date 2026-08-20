package acceleratorprobe

import "testing"

func TestParsePerfAwake(t *testing.T) {
	v, err := parsePerfAwake([]byte("250000000\tns\ti915/software-gt-awake-time/\t1000000000\t100.00\t\t\n"))
	if err != nil || v != 25 {
		t.Fatalf("got %.2f, %v", v, err)
	}
}

func TestParsePowermetrics(t *testing.T) {
	r, err := parsePowermetrics([]byte("GPU HW active residency: 12.50%\nANE HW active residency: 3.25%\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 2 || r[0].Utilization != 12.5 || r[1].Kind != "NPU" || r[1].Utilization != 3.25 {
		t.Fatalf("unexpected: %#v", r)
	}
}

func TestParsePowermetricsDoesNotInventMissingANEResidency(t *testing.T) {
	r, err := parsePowermetrics([]byte("GPU HW active residency: 0.94%\nANE Power: 0 mW\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 2 || r[0].Utilization != .94 || r[1].Kind != "NPU" || r[1].Known {
		t.Fatalf("unexpected: %#v", r)
	}
}
