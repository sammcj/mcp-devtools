package unit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/security"
)

// blockingServer returns a server that never responds until the client goes away,
// so any successful return proves the request was cancelled rather than completed.
func blockingServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the body so the server notices a client disconnect on POSTs.
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func assertCancelled(t *testing.T, err error, started time.Time) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, request was not cancelled")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a context error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("cancellation took %v, context was not honoured promptly", elapsed)
	}
}

func TestSafeHTTPGetHonoursContextCancellation(t *testing.T) {
	srv := blockingServer(t)
	ops := security.NewOperations("test-tool")

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := ops.SafeHTTPGet(ctx, srv.URL)
	assertCancelled(t, err, start)
}

func TestSafeHTTPGetHonoursContextDeadline(t *testing.T) {
	srv := blockingServer(t)
	ops := security.NewOperations("test-tool")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ops.SafeHTTPGet(ctx, srv.URL)
	assertCancelled(t, err, start)
}

func TestSafeHTTPPostHonoursContextCancellation(t *testing.T) {
	srv := blockingServer(t)
	ops := security.NewOperations("test-tool")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ops.SafeHTTPPost(ctx, srv.URL, strings.NewReader(`{"a":1}`))
	assertCancelled(t, err, start)
}

func TestSafeHTTPGetWithHeadersHonoursContextCancellation(t *testing.T) {
	srv := blockingServer(t)
	ops := security.NewOperations("test-tool")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ops.SafeHTTPGetWithHeaders(ctx, srv.URL, map[string]string{"X-Test": "1"})
	assertCancelled(t, err, start)
}

func TestSafeHTTPPostWithHeadersHonoursContextCancellation(t *testing.T) {
	srv := blockingServer(t)
	ops := security.NewOperations("test-tool")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ops.SafeHTTPPostWithHeaders(ctx, srv.URL, strings.NewReader(`{"a":1}`), map[string]string{"X-Test": "1"})
	assertCancelled(t, err, start)
}

// A request that completes normally must still pass headers through and return
// the exact response bytes, so cancellation support hasn't changed the happy path.
func TestSafeHTTPGetWithHeadersSendsHeaders(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Test")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ops := security.NewOperations("test-tool")
	resp, err := ops.SafeHTTPGetWithHeaders(t.Context(), srv.URL, map[string]string{"X-Test": "sentinel"})
	if err != nil {
		t.Fatalf("SafeHTTPGetWithHeaders failed: %v", err)
	}
	if got != "sentinel" {
		t.Errorf("header not sent, server saw %q", got)
	}
	if string(resp.Content) != "ok" {
		t.Errorf("unexpected body %q", resp.Content)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status %d", resp.StatusCode)
	}
}
