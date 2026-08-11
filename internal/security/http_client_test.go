package security

import (
	"testing"
	"time"
)

func TestResolveHTTPTimeout(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"unset", "", defaultHTTPTimeout},
		{"seconds", "5", 5 * time.Second},
		{"zero", "0", defaultHTTPTimeout},
		{"negative", "-3", defaultHTTPTimeout},
		{"not a number", "30s", defaultHTTPTimeout},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveHTTPTimeout(tc.raw); got != tc.want {
				t.Errorf("resolveHTTPTimeout(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// A request whose context has no deadline of its own must still be bounded by
// the client, which the previous per-call http.Client{} did not do.
func TestSafeHTTPClientHasTimeout(t *testing.T) {
	if timeout := safeHTTPClient().Timeout; timeout <= 0 {
		t.Fatalf("shared client has no timeout, got %v", timeout)
	}
}
