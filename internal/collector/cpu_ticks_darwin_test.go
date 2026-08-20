//go:build darwin && cgo

package collector

import "testing"

func TestDarwinCPUTicks(t *testing.T) {
	totals, idles, ok := darwinCPUTicks()
	if !ok {
		t.Fatal("Mach host_processor_info did not return CPU ticks")
	}
	if len(totals) != len(idles) || len(totals) < 1 {
		t.Fatalf("invalid core arrays: totals=%d idles=%d", len(totals), len(idles))
	}
	for i := range totals {
		if totals[i] == 0 || idles[i] > totals[i] {
			t.Fatalf("invalid CPU %d ticks: total=%d idle=%d", i, totals[i], idles[i])
		}
	}
}
