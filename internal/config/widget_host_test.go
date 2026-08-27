package config

import (
	"testing"
	"time"
)

func TestWidgetHostAddressUsesEndpointHost(t *testing.T) {
	w := Widget{Endpoint: "http://10.0.0.1:7844"}
	if got := w.HostAddress(); got != "10.0.0.1" {
		t.Fatalf("HostAddress = %q, want 10.0.0.1", got)
	}
}

func TestWidgetHostAddressPrefersExplicitHost(t *testing.T) {
	w := Widget{Host: "ci-box", Endpoint: "http://127.0.0.1:7844"}
	if got := w.HostAddress(); got != "ci-box" {
		t.Fatalf("HostAddress = %q, want ci-box", got)
	}
}

func TestValidateAcceptsTheSupportedWidgetTypes(t *testing.T) {
	for _, kind := range []string{"nvr", "orchestrator", "cip"} {
		c := Config{
			RefreshInterval: time.Second,
			HistorySize:     60,
			Servers:         []Server{{Name: "a", Address: "x"}},
			Widgets:         []Widget{{Name: "w", Type: kind, Endpoint: "http://h:8080", TokenEnv: "T"}},
		}
		if err := c.Validate(); err != nil {
			t.Errorf("Validate rejected type %q: %v", kind, err)
		}
	}
}

func TestValidateRejectsAnUnknownWidgetType(t *testing.T) {
	c := Config{
		RefreshInterval: time.Second,
		HistorySize:     60,
		Servers:         []Server{{Name: "a", Address: "x"}},
		Widgets:         []Widget{{Name: "w", Type: "wat", Endpoint: "http://h:8080", TokenEnv: "T"}},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("Validate accepted an unknown widget type")
	}
}

// A cip widget must still require exactly one token source, so a token can
// never sit in the YAML file.
func TestValidateRequiresOneTokenSourceForCIP(t *testing.T) {
	for _, w := range []Widget{
		{Name: "w", Type: "cip", Endpoint: "http://h:8080"},
		{Name: "w", Type: "cip", Endpoint: "http://h:8080", TokenEnv: "T", TokenFile: "/f"},
	} {
		c := Config{
			RefreshInterval: time.Second,
			HistorySize:     60,
			Servers:         []Server{{Name: "a", Address: "x"}},
			Widgets:         []Widget{w},
		}
		if err := c.Validate(); err == nil {
			t.Errorf("Validate accepted %+v", w)
		}
	}
}
