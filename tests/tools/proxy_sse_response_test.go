package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/upstream"
)

// sseServer answers a POST the way a 2026-07-28 streamable HTTP server does:
// a text/event-stream body carrying the JSON-RPC response as a data event.
// The official go-sdk server does exactly this, so mcp-devtools proxying
// another mcp-devtools exercises this path.
func sseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, body); err != nil {
			t.Errorf("failed to write SSE body: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func sendToolsList(t *testing.T, serverURL string) (*upstream.Message, error) {
	t.Helper()

	transport := upstream.NewHTTPTransport(&upstream.Config{ServerURL: serverURL})
	t.Cleanup(func() { _ = transport.Close() })

	params, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}

	return transport.SendReceive(t.Context(), &upstream.Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  params,
	})
}

func TestSendReceiveDecodesSSEResponse(t *testing.T) {
	srv := sseServer(t, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[]}}\n\n")

	response, err := sendToolsList(t, srv.URL)
	if err != nil {
		t.Fatalf("SendReceive failed on an SSE response: %v", err)
	}
	if response.ID == nil {
		t.Fatal("response has no id")
	}
	if len(response.Result) == 0 {
		t.Fatalf("response carries no result: %+v", response)
	}
}

// A stream may carry comments, retry directives and unrelated events before the
// message, and a data payload may be split across several data: lines.
func TestSendReceiveHandlesSSEPreambleAndFoldedData(t *testing.T) {
	srv := sseServer(t, ": keep-alive\nretry: 1000\n\nevent: ping\ndata: {}\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\n"+
		"data: \"result\":{\"tools\":[]}}\n\n")

	response, err := sendToolsList(t, srv.URL)
	if err != nil {
		t.Fatalf("SendReceive failed on a folded SSE response: %v", err)
	}
	if len(response.Result) == 0 {
		t.Fatalf("response carries no result: %+v", response)
	}
}

// A plain JSON response must keep working; servers may choose either.
func TestSendReceiveStillDecodesJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`); err != nil {
			t.Errorf("failed to write JSON body: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	response, err := sendToolsList(t, srv.URL)
	if err != nil {
		t.Fatalf("SendReceive failed on a JSON response: %v", err)
	}
	if len(response.Result) == 0 {
		t.Fatalf("response carries no result: %+v", response)
	}
}

// An event stream that closes without a JSON-RPC message must report that
// clearly rather than surfacing a decoder error about empty input.
func TestSendReceiveReportsEmptySSEStream(t *testing.T) {
	srv := sseServer(t, ": keep-alive\n\n")

	if _, err := sendToolsList(t, srv.URL); err == nil {
		t.Fatal("SendReceive accepted an event stream carrying no message")
	}
}

// A stream carries whatever the upstream emits while the call runs. A progress
// notification arriving before the response must not be mistaken for it: the
// notification has no id and no result, so returning it strands the caller with
// an empty response instead of the tool's output.
func TestSendReceiveSkipsNotificationsBeforeTheResponse(t *testing.T) {
	srv := sseServer(t, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{\"progress\":1}}\n\n"+
		"event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"level\":\"info\"}}\n\n"+
		"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[]}}\n\n")

	response, err := sendToolsList(t, srv.URL)
	if err != nil {
		t.Fatalf("SendReceive failed with notifications ahead of the response: %v", err)
	}
	if len(response.Result) == 0 {
		t.Fatalf("a notification was returned instead of the response: %+v", response)
	}
}

// A response to some other request is not ours to consume either.
func TestSendReceiveSkipsAResponseToAnotherRequest(t *testing.T) {
	srv := sseServer(t, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":\"other\",\"result\":{\"wrong\":true}}\n\n"+
		"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[]}}\n\n")

	response, err := sendToolsList(t, srv.URL)
	if err != nil {
		t.Fatalf("SendReceive failed: %v", err)
	}

	var result struct {
		Wrong bool `json:"wrong"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if result.Wrong {
		t.Fatal("the response to a different request was returned")
	}
}

// A JSON-RPC error for this request ends the read like any other response.
func TestSendReceiveReturnsAnErrorResponseFromSSE(t *testing.T) {
	srv := sseServer(t, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n"+
		"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"no such tool\"}}\n\n")

	response, err := sendToolsList(t, srv.URL)
	if err != nil {
		t.Fatalf("SendReceive failed: %v", err)
	}
	if response.Error == nil {
		t.Fatalf("the error response was not returned: %+v", response)
	}
	if response.Error.Code != -32601 {
		t.Errorf("unexpected error code: %d", response.Error.Code)
	}
}

// Notifications with no response behind them must not look like success.
func TestSendReceiveReportsAStreamOfOnlyNotifications(t *testing.T) {
	srv := sseServer(t, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n")

	if _, err := sendToolsList(t, srv.URL); err == nil {
		t.Fatal("SendReceive accepted a stream that never answered the request")
	}
}
