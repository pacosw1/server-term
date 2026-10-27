package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDesktopRelayRequiresOptInAndToken(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		token   string
		want    int
	}{{"disabled", false, "secret", 404}, {"missing token", true, "", 401}, {"wrong token", true, "wrong", 401}} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer("test", "secret", nil, nil)
			s.DesktopEnabled = tc.enabled
			r := httptest.NewRequest(http.MethodGet, "/v1/desktop/vnc", nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
}
