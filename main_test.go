package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sammcj/mcp-devtools/internal/mcpapi"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// fakeTool is a minimal tools.Tool used to drive newToolHandler in tests.
type fakeTool struct {
	name   string
	result *mcpapi.CallToolResult
	err    error
}

func (f *fakeTool) Definition() mcpapi.Tool {
	return mcpapi.NewTool(f.name, mcpapi.WithDescription("fake tool for handler tests"))
}

func (f *fakeTool) Execute(_ context.Context, _ *logrus.Logger, _ *sync.Map, _ map[string]any) (*mcpapi.CallToolResult, error) {
	return f.result, f.err
}

func quietLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.FatalLevel)
	l.SetOutput(io.Discard)
	return l
}

func toolResultText(result *mcpapi.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	text, ok := mcpapi.AsTextContent(result.Content[0])
	if !ok {
		return ""
	}
	return text.Text
}

// A tool whose Execute returns an error must surface as an isError result with
// nil Go error, otherwise the SDK responds with a JSON-RPC protocol error
// that clients treat as a server crash.
func TestNewToolHandler_ExecutionFailureReturnsIsError(t *testing.T) {
	const name = "fake_exec_failure_tool"
	registry.RegisterProxiedTool(&fakeTool{name: name, err: errors.New("missing required parameter: expression")})

	handler := newToolHandler(name, "http", quietLogger())
	req := &mcpapi.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: name, Arguments: json.RawMessage(`{}`)}}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned a Go error (becomes JSON-RPC -32603), want nil: %v", err)
	}
	if result == nil {
		t.Fatal("handler returned nil result")
	}
	if !result.IsError {
		t.Error("expected result.IsError = true for an execution failure")
	}
	if msg := toolResultText(result); !strings.Contains(msg, "missing required parameter") {
		t.Errorf("expected message to include the underlying cause, got: %q", msg)
	}
}

// An unknown tool name must also return an isError result rather than a Go error.
func TestNewToolHandler_ToolNotFoundReturnsIsError(t *testing.T) {
	handler := newToolHandler("definitely_not_registered_tool", "http", quietLogger())
	req := &mcpapi.CallToolRequest{Params: &mcp.CallToolParamsRaw{}}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned a Go error, want nil: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected isError result for unknown tool, got: %+v", result)
	}
}

// A successful execution must pass the tool's result through unchanged with a
// nil error and IsError unset.
func TestNewToolHandler_SuccessPassesResultThrough(t *testing.T) {
	const name = "fake_success_tool"
	want := mcpapi.NewToolResultText("ok")
	registry.RegisterProxiedTool(&fakeTool{name: name, result: want})

	handler := newToolHandler(name, "http", quietLogger())
	req := &mcpapi.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: name, Arguments: json.RawMessage(`{}`)}}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected a non-error result, got: %+v", result)
	}
	if got := toolResultText(result); got != "ok" {
		t.Errorf("expected result text %q, got %q", "ok", got)
	}
}
