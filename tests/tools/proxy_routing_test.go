package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/types"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy/upstream"
)

// upstreamServer answers any JSON-RPC POST, echoing which server was reached so
// a routing test can tell the two apart.
func upstreamServer(t *testing.T, label string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		result := map[string]any{"served_by": label}
		if req.Method == "tools/list" {
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": label}}}
		}

		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		if err != nil {
			t.Errorf("failed to marshal response: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(body); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func connectedManager(t *testing.T, urls map[string]string) *upstream.Manager {
	t.Helper()

	cfg := &types.ProxyConfig{CacheDir: t.TempDir()}
	for name, url := range urls {
		cfg.Upstreams = append(cfg.Upstreams, types.UpstreamConfig{Name: name, URL: url})
	}

	manager := upstream.NewManager(cfg)
	if err := manager.Connect(t.Context()); err != nil {
		t.Fatalf("failed to connect upstreams: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })

	return manager
}

// A caller that already knows the upstream must be able to say so. Routing by
// tool name alone cannot work once two upstreams are configured, because an
// upstream tool's own name carries no prefix.
func TestExecuteToolOnRoutesToTheNamedUpstream(t *testing.T) {
	alpha := upstreamServer(t, "alpha")
	beta := upstreamServer(t, "beta")

	manager := connectedManager(t, map[string]string{"alpha": alpha.URL, "beta": beta.URL})

	for _, name := range []string{"alpha", "beta"} {
		response, err := manager.ExecuteToolOn(t.Context(), name, "echo", map[string]any{})
		if err != nil {
			t.Fatalf("ExecuteToolOn(%q) failed: %v", name, err)
		}

		var result struct {
			ServedBy string `json:"served_by"`
		}
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}
		if result.ServedBy != name {
			t.Errorf("call for upstream %q was served by %q", name, result.ServedBy)
		}
	}
}

func TestExecuteToolOnRejectsAnUnknownUpstream(t *testing.T) {
	alpha := upstreamServer(t, "alpha")
	manager := connectedManager(t, map[string]string{"alpha": alpha.URL})

	if _, err := manager.ExecuteToolOn(t.Context(), "nonexistent", "echo", map[string]any{}); err == nil {
		t.Fatal("ExecuteToolOn accepted an upstream that is not configured")
	}
}

// The name-based route stays ambiguous with several upstreams configured, which
// is the reason ExecuteToolOn exists; this pins that so the two do not get
// quietly merged back together.
func TestExecuteToolByNameIsAmbiguousAcrossUpstreams(t *testing.T) {
	alpha := upstreamServer(t, "alpha")
	beta := upstreamServer(t, "beta")

	manager := connectedManager(t, map[string]string{"alpha": alpha.URL, "beta": beta.URL})

	if _, err := manager.ExecuteTool(t.Context(), "echo", map[string]any{}); err == nil {
		t.Fatal("an unprefixed tool name resolved despite two upstreams being configured")
	}

	response, err := manager.ExecuteTool(t.Context(), fmt.Sprintf("%s:%s", "beta", "echo"), map[string]any{})
	if err != nil {
		t.Fatalf("a prefixed tool name failed to route: %v", err)
	}
	var result struct {
		ServedBy string `json:"served_by"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if result.ServedBy != "beta" {
		t.Errorf("prefixed call routed to %q", result.ServedBy)
	}
}
