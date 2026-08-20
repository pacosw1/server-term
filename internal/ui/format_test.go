package ui

import (
	"testing"
	"time"
)

func TestElapsedDurationIncludesSeconds(t *testing.T) {
	for _, test := range []struct {
		input time.Duration
		want  string
	}{
		{47*time.Second + 900*time.Millisecond, "47s"},
		{2*time.Minute + 13*time.Second, "2m13s"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s"},
		{-time.Second, "0s"},
	} {
		if got := elapsedDuration(test.input); got != test.want {
			t.Errorf("elapsedDuration(%s) = %q, want %q", test.input, got, test.want)
		}
	}
}
