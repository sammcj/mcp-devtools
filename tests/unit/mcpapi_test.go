package unit

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sammcj/mcp-devtools/internal/mcpapi"
)

// marshalTool round-trips a tool through JSON so assertions compare wire output
// rather than Go struct internals.
func marshalTool(t *testing.T, tool mcpapi.Tool) map[string]any {
	t.Helper()
	b, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal tool: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal tool: %v", err)
	}
	return got
}

func inputSchemaJSON(t *testing.T, tool mcpapi.Tool) map[string]any {
	t.Helper()
	got := marshalTool(t, tool)
	schema, ok := got["inputSchema"].(map[string]any)
	if !ok {
		t.Fatalf("inputSchema missing or not an object: %#v", got["inputSchema"])
	}
	return schema
}

func TestNewToolAlwaysHasObjectInputSchema(t *testing.T) {
	// The SDK panics on registration if the input schema is nil or not an
	// object, so a tool with no parameters must still carry one.
	tool := mcpapi.NewTool("no_params", mcpapi.WithDescription("takes nothing"))

	schema := mcpapi.InputSchemaOf(tool)
	if schema == nil {
		t.Fatal("input schema is not a *jsonschema.Schema")
	}
	if schema.Type != "object" {
		t.Fatalf("input schema type = %q, want \"object\"", schema.Type)
	}

	got := marshalTool(t, tool)
	if got["name"] != "no_params" {
		t.Errorf("name = %v, want no_params", got["name"])
	}
	if got["description"] != "takes nothing" {
		t.Errorf("description = %v, want %q", got["description"], "takes nothing")
	}
}

func TestPropertyTypesAndConstraints(t *testing.T) {
	tool := mcpapi.NewTool("demo",
		mcpapi.WithString("query", mcpapi.Required(), mcpapi.Description("the query"), mcpapi.MaxLength(100)),
		mcpapi.WithString("provider", mcpapi.DefaultString("brave"), mcpapi.Enum("brave", "duckduckgo")),
		mcpapi.WithNumber("count", mcpapi.DefaultNumber(10)),
		mcpapi.WithBoolean("verbose", mcpapi.DefaultBool(false)),
	)

	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "the query",
				"maxLength":   float64(100),
			},
			"provider": map[string]any{
				"type":    "string",
				"default": "brave",
				"enum":    []any{"brave", "duckduckgo"},
			},
			"count":   map[string]any{"type": "number", "default": float64(10)},
			"verbose": map[string]any{"type": "boolean", "default": false},
		},
		"required": []any{"query"},
	}

	if got := inputSchemaJSON(t, tool); !reflect.DeepEqual(got, want) {
		t.Errorf("input schema mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestRequiredCollectsEveryRequiredProperty(t *testing.T) {
	tool := mcpapi.NewTool("demo",
		mcpapi.WithString("a", mcpapi.Required()),
		mcpapi.WithString("b"),
		mcpapi.WithNumber("c", mcpapi.Required()),
	)

	schema := mcpapi.InputSchemaOf(tool)
	if schema == nil {
		t.Fatal("input schema is not a *jsonschema.Schema")
	}
	want := []string{"a", "c"}
	if !reflect.DeepEqual(schema.Required, want) {
		t.Errorf("required = %v, want %v", schema.Required, want)
	}
}

func TestArrayItems(t *testing.T) {
	tool := mcpapi.NewTool("demo",
		mcpapi.WithArray("tags", mcpapi.WithStringItems()),
		mcpapi.WithArray("deps", mcpapi.Items(map[string]any{"type": "object"})),
	)

	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"deps": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
		},
	}

	if got := inputSchemaJSON(t, tool); !reflect.DeepEqual(got, want) {
		t.Errorf("input schema mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestNestedObjectProperties(t *testing.T) {
	// Mirrors the shape used by the filesystem and github tools, where an
	// "options" object carries a raw JSON Schema fragment per key.
	tool := mcpapi.NewTool("demo",
		mcpapi.WithObject("options",
			mcpapi.Description("opts"),
			mcpapi.Properties(map[string]any{
				"path":  map[string]any{"type": "string", "description": "a path"},
				"limit": map[string]any{"type": "number", "description": "a limit", "default": 30},
				"paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}),
		),
	)

	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"options": map[string]any{
				"type":        "object",
				"description": "opts",
				"properties": map[string]any{
					"path":  map[string]any{"type": "string", "description": "a path"},
					"limit": map[string]any{"type": "number", "description": "a limit", "default": float64(30)},
					"paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
		},
	}

	if got := inputSchemaJSON(t, tool); !reflect.DeepEqual(got, want) {
		t.Errorf("input schema mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestAnnotations(t *testing.T) {
	tool := mcpapi.NewTool("demo",
		mcpapi.WithReadOnlyHintAnnotation(true),
		mcpapi.WithDestructiveHintAnnotation(false),
		mcpapi.WithIdempotentHintAnnotation(true),
		mcpapi.WithOpenWorldHintAnnotation(false),
	)

	got := marshalTool(t, tool)
	want := map[string]any{
		"readOnlyHint":    true,
		"destructiveHint": false,
		"idempotentHint":  true,
		"openWorldHint":   false,
	}
	if !reflect.DeepEqual(got["annotations"], want) {
		t.Errorf("annotations = %#v, want %#v", got["annotations"], want)
	}
}

// The SDK models destructiveHint and openWorldHint as *bool, where nil means
// true. The shim must take an address rather than leaving them unset.
func TestDefaultTrueAnnotationsAreExplicit(t *testing.T) {
	tool := mcpapi.NewTool("demo",
		mcpapi.WithDestructiveHintAnnotation(false),
		mcpapi.WithOpenWorldHintAnnotation(false),
	)

	if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Error("DestructiveHint should be an explicit false")
	}
	if tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
		t.Error("OpenWorldHint should be an explicit false")
	}
}

func TestResultConstructors(t *testing.T) {
	text := mcpapi.NewToolResultText("hello")
	if text.IsError {
		t.Error("text result should not be an error")
	}
	tc, ok := mcpapi.AsTextContent(text.Content[0])
	if !ok {
		t.Fatalf("content is not text: %T", text.Content[0])
	}
	if tc.Text != "hello" {
		t.Errorf("text = %q, want hello", tc.Text)
	}

	errRes := mcpapi.NewToolResultError("boom")
	if !errRes.IsError {
		t.Error("error result should set IsError")
	}
	if etc, ok := mcpapi.AsTextContent(errRes.Content[0]); !ok || etc.Text != "boom" {
		t.Errorf("error content = %#v", errRes.Content[0])
	}

	jsonRes, err := mcpapi.NewToolResultJSON(map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("NewToolResultJSON: %v", err)
	}
	if jtc, ok := mcpapi.AsTextContent(jsonRes.Content[0]); !ok || jtc.Text != `{"a":1}` {
		t.Errorf("json content = %#v", jsonRes.Content[0])
	}
	if jsonRes.StructuredContent == nil {
		t.Error("json result should set StructuredContent")
	}
}

// A tool built by the shim must survive registration, which is where the SDK
// enforces its input schema rules.
func TestShimToolRegistersWithSDK(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	tool := mcpapi.NewTool("demo", mcpapi.WithString("query", mcpapi.Required()))
	srv.AddTool(&tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcpapi.NewToolResultText("ok"), nil
	})
}
