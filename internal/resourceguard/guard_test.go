package resourceguard

import "testing"

func TestNextLimitsBacksOffUnderPressure(t *testing.T) {
	got := NextLimits(Limits{85, 90}, Observation{AvailablePercent: 8, MemorySomeAvg10: 12, CPUUtilization: 96, CPUSomeAvg10: 22}, false)
	if got != (Limits{83, 85}) {
		t.Fatalf("got %+v", got)
	}
}

func TestNextLimitsRecoversSlowlyOnlyOnRecoveryTick(t *testing.T) {
	calm := Observation{AvailablePercent: 40, MemorySomeAvg10: 0.2, MemoryFullAvg10: 0, CPUUtilization: 30, CPUSomeAvg10: 1}
	if got := NextLimits(Limits{70, 70}, calm, false); got != (Limits{70, 70}) {
		t.Fatalf("unexpected early recovery: %+v", got)
	}
	if got := NextLimits(Limits{70, 70}, calm, true); got != (Limits{70.5, 71}) {
		t.Fatalf("got %+v", got)
	}
}

func TestNextLimitsClampsAtSafetyBounds(t *testing.T) {
	pressure := Observation{AvailablePercent: 1, MemoryFullAvg10: 50, CPUUtilization: 100}
	if got := NextLimits(Limits{60, 60}, pressure, false); got != (Limits{60, 60}) {
		t.Fatalf("lower clamp failed: %+v", got)
	}
	calm := Observation{AvailablePercent: 50}
	if got := NextLimits(Limits{85, 90}, calm, true); got != (Limits{85, 90}) {
		t.Fatalf("upper clamp failed: %+v", got)
	}
}
