package metrics

import (
	"math"
	"testing"
	"time"
)

func TestDeriveRates(t *testing.T) {
	start := time.Unix(100, 0)
	previous := Sample{At: start, Online: true, CPUTotal: 1000, CPUIdle: 800, CoreTotal: []uint64{100}, CoreIdle: []uint64{80}, NetRx: 100, NetTx: 200, EnergyMicrojoules: 1_000_000}
	current := Sample{At: start.Add(2 * time.Second), Online: true, CPUTotal: 1200, CPUIdle: 900, CoreTotal: []uint64{200}, CoreIdle: []uint64{120}, NetRx: 2100, NetTx: 1200, EnergyMicrojoules: 5_000_000}
	Derive(&previous, &current)
	if math.Abs(current.CPUPercent-50) > 0.01 || current.NetRxRate != 1000 || current.NetTxRate != 500 || current.PowerWatts != 2 || !current.PowerKnown || len(current.CorePercent) != 1 || current.CorePercent[0] != 60 {
		t.Fatalf("unexpected derived metrics: %+v", current)
	}
}
