package config

import "testing"

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
