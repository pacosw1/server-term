package ui

import "testing"

func TestSpinnerFrame(t *testing.T) {
	// The frame must change over time, or it does not read as motion.
	if spinnerFrame(0) == spinnerFrame(1) {
		t.Fatal("consecutive ticks render the same frame")
	}
	// It must cycle, not run off the end of the strip.
	if spinnerFrame(0) != spinnerFrame(len(spinnerFrames)) {
		t.Fatal("the spinner does not cycle")
	}
	if spinnerFrame(-3) == "" {
		t.Fatal("a negative tick must still render a frame")
	}
}

func TestAppendTail(t *testing.T) {
	t.Run("keeps a new line", func(t *testing.T) {
		got := appendTail(nil, "bun test", 3)
		if len(got) != 1 || got[0] != "bun test" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("does not repeat the line it already holds", func(t *testing.T) {
		got := appendTail([]string{"bun test"}, "bun test", 3)
		if len(got) != 1 {
			t.Fatalf("a repeated line was appended: %v", got)
		}
	})

	t.Run("drops the oldest past the cap", func(t *testing.T) {
		got := appendTail([]string{"a", "b", "c"}, "d", 3)
		if len(got) != 3 || got[0] != "b" || got[2] != "d" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("ignores an empty line, which means the daemon said nothing", func(t *testing.T) {
		got := appendTail([]string{"a"}, "", 3)
		if len(got) != 1 || got[0] != "a" {
			t.Fatalf("got %v", got)
		}
	})
}

func TestOrchestratorRefreshInterval(t *testing.T) {
	// Watching the agents tab must refresh faster than the background rate,
	// and the background rate must never be faster than the tab rate.
	focused := orchestratorRefresh(true)
	background := orchestratorRefresh(false)
	if focused >= background {
		t.Fatalf("focused %v is not faster than background %v", focused, background)
	}
}
