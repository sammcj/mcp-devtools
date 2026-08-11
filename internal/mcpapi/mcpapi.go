// Package mcpapi adapts the official MCP Go SDK to the builder-style API this
// codebase was written against, so tool definitions did not have to change when
// the project moved off mark3labs/mcp-go for MCP spec 2026-07-28.
//
// Everything here produces official SDK types: Tool, CallToolResult and
// TextContent are aliases, and input schemas are *jsonschema.Schema.
package mcpapi

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SDK types re-exported so tools only import this package.
type (
	Tool            = mcp.Tool
	CallToolResult  = mcp.CallToolResult
	CallToolRequest = mcp.CallToolRequest
	Content         = mcp.Content
	TextContent     = mcp.TextContent
	ToolAnnotations = mcp.ToolAnnotations
	Schema          = jsonschema.Schema
)

// ToolOption configures a Tool.
type ToolOption func(*Tool)

// property is a JSON Schema for a single parameter plus whether the parameter
// is required. The SDK records required-ness on the parent object schema, so it
// cannot live on the property schema itself.
type property struct {
	schema   *jsonschema.Schema
	required bool
}

// PropertyOption configures a single parameter of a tool's input schema.
type PropertyOption func(*property)

// NewTool builds a tool with an object input schema. The SDK panics on
// registration if the input schema is nil or is not an object, so it is always
// populated here.
func NewTool(name string, opts ...ToolOption) Tool {
	t := Tool{
		Name: name,
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{},
		},
		Annotations: &mcp.ToolAnnotations{},
	}
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

// InputSchemaOf returns a tool's input schema, or nil if it is not a
// *jsonschema.Schema (which only happens for tools built outside this package).
func InputSchemaOf(t Tool) *jsonschema.Schema {
	s, _ := t.InputSchema.(*jsonschema.Schema)
	return s
}

// objectSchema returns the tool's input schema, replacing it if a caller has
// substituted something other than a *jsonschema.Schema.
func objectSchema(t *Tool) *jsonschema.Schema {
	s, ok := t.InputSchema.(*jsonschema.Schema)
	if !ok || s == nil {
		s = &jsonschema.Schema{Type: "object"}
		t.InputSchema = s
	}
	if s.Properties == nil {
		s.Properties = map[string]*jsonschema.Schema{}
	}
	return s
}

func annotations(t *Tool) *mcp.ToolAnnotations {
	if t.Annotations == nil {
		t.Annotations = &mcp.ToolAnnotations{}
	}
	return t.Annotations
}

// WithDescription sets the tool description.
func WithDescription(description string) ToolOption {
	return func(t *Tool) { t.Description = description }
}

// WithTitle sets the tool's human-readable display name.
func WithTitle(title string) ToolOption {
	return func(t *Tool) { t.Title = title }
}

func withProperty(name, typ string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		p := &property{schema: &jsonschema.Schema{Type: typ}}
		for _, opt := range opts {
			opt(p)
		}
		s := objectSchema(t)
		s.Properties[name] = p.schema
		if p.required && !slices.Contains(s.Required, name) {
			s.Required = append(s.Required, name)
		}
	}
}

// WithString adds a string parameter.
func WithString(name string, opts ...PropertyOption) ToolOption {
	return withProperty(name, "string", opts...)
}

// WithNumber adds a number parameter.
func WithNumber(name string, opts ...PropertyOption) ToolOption {
	return withProperty(name, "number", opts...)
}

// WithBoolean adds a boolean parameter.
func WithBoolean(name string, opts ...PropertyOption) ToolOption {
	return withProperty(name, "boolean", opts...)
}

// WithArray adds an array parameter.
func WithArray(name string, opts ...PropertyOption) ToolOption {
	return withProperty(name, "array", opts...)
}

// WithObject adds an object parameter.
func WithObject(name string, opts ...PropertyOption) ToolOption {
	return withProperty(name, "object", opts...)
}

// Required marks a parameter as required.
func Required() PropertyOption {
	return func(p *property) { p.required = true }
}

// Description sets a parameter description.
func Description(desc string) PropertyOption {
	return func(p *property) { p.schema.Description = desc }
}

// Enum restricts a string parameter to a fixed set of values.
func Enum(values ...string) PropertyOption {
	return func(p *property) {
		vals := make([]any, len(values))
		for i, v := range values {
			vals[i] = v
		}
		p.schema.Enum = vals
	}
}

// MaxLength sets the maximum length of a string parameter.
func MaxLength(maxLen int) PropertyOption {
	return func(p *property) { p.schema.MaxLength = new(maxLen) }
}

// MinLength sets the minimum length of a string parameter.
func MinLength(minLen int) PropertyOption {
	return func(p *property) { p.schema.MinLength = new(minLen) }
}

// Minimum sets the inclusive lower bound of a number parameter.
func Minimum(minVal float64) PropertyOption {
	return func(p *property) { p.schema.Minimum = new(minVal) }
}

// Maximum sets the inclusive upper bound of a number parameter.
func Maximum(maxVal float64) PropertyOption {
	return func(p *property) { p.schema.Maximum = new(maxVal) }
}

// DefaultString sets the default value of a string parameter.
func DefaultString(value string) PropertyOption {
	return func(p *property) { p.schema.Default = mustMarshal(value) }
}

// DefaultNumber sets the default value of a number parameter.
func DefaultNumber[T int | int64 | float64](value T) PropertyOption {
	return func(p *property) { p.schema.Default = mustMarshal(value) }
}

// DefaultBool sets the default value of a boolean parameter.
func DefaultBool(value bool) PropertyOption {
	return func(p *property) { p.schema.Default = mustMarshal(value) }
}

// DefaultArray sets the default value of an array parameter.
func DefaultArray[T any](value []T) PropertyOption {
	return func(p *property) { p.schema.Default = mustMarshal(value) }
}

// Properties sets the properties of an object parameter. Each value is a JSON
// Schema fragment, either a *jsonschema.Schema or a map[string]any.
func Properties(props map[string]any) PropertyOption {
	return func(p *property) {
		if p.schema.Properties == nil {
			p.schema.Properties = map[string]*jsonschema.Schema{}
		}
		for name, raw := range props {
			p.schema.Properties[name] = toSchema(raw)
		}
	}
}

// Items sets the element schema of an array parameter.
func Items(schema any) PropertyOption {
	return func(p *property) { p.schema.Items = toSchema(schema) }
}

// WithStringItems declares an array parameter's elements as strings.
func WithStringItems(opts ...PropertyOption) PropertyOption {
	return func(p *property) {
		item := &property{schema: &jsonschema.Schema{Type: "string"}}
		for _, opt := range opts {
			opt(item)
		}
		p.schema.Items = item.schema
	}
}

// WithReadOnlyHintAnnotation declares whether the tool modifies its environment.
func WithReadOnlyHintAnnotation(value bool) ToolOption {
	return func(t *Tool) { annotations(t).ReadOnlyHint = value }
}

// WithIdempotentHintAnnotation declares whether repeated calls with the same
// arguments have any additional effect.
func WithIdempotentHintAnnotation(value bool) ToolOption {
	return func(t *Tool) { annotations(t).IdempotentHint = value }
}

// WithDestructiveHintAnnotation declares whether the tool may perform
// destructive updates.
func WithDestructiveHintAnnotation(value bool) ToolOption {
	return func(t *Tool) { annotations(t).DestructiveHint = &value }
}

// WithOpenWorldHintAnnotation declares whether the tool interacts with external
// entities.
func WithOpenWorldHintAnnotation(value bool) ToolOption {
	return func(t *Tool) { annotations(t).OpenWorldHint = &value }
}

// WithTitleAnnotation sets the annotation title.
func WithTitleAnnotation(title string) ToolOption {
	return func(t *Tool) { annotations(t).Title = title }
}

// NewToolResultText returns a successful result carrying a single text block.
func NewToolResultText(text string) *CallToolResult {
	return &CallToolResult{Content: []Content{&TextContent{Text: text}}}
}

// NewToolResultJSON returns a successful result carrying data as JSON, both as
// text and as structured content.
func NewToolResultJSON[T any](data T) (*CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal JSON: %w", err)
	}
	return &CallToolResult{
		Content:           []Content{&TextContent{Text: string(b)}},
		StructuredContent: data,
	}, nil
}

// NewToolResultError returns a tool-level failure. This is not a protocol
// error: the message is delivered to the model so it can self-correct.
func NewToolResultError(text string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{&TextContent{Text: text}},
		IsError: true,
	}
}

// NewToolResultErrorFromErr returns a tool-level failure built from an error.
func NewToolResultErrorFromErr(text string, err error) *CallToolResult {
	if text == "" {
		return NewToolResultError(err.Error())
	}
	return NewToolResultError(fmt.Sprintf("%s: %v", text, err))
}

// AsTextContent reports whether content is a text block, returning it if so.
func AsTextContent(content any) (*TextContent, bool) {
	tc, ok := content.(*TextContent)
	return tc, ok
}

// toSchema converts a JSON Schema fragment to a *jsonschema.Schema. Fragments
// are static literals in tool definitions, so a malformed one is a programming
// error and panics at init rather than producing a silently broken tool.
func toSchema(v any) *jsonschema.Schema {
	switch s := v.(type) {
	case nil:
		return nil
	case *jsonschema.Schema:
		return s
	case jsonschema.Schema:
		return &s
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("mcpapi: cannot marshal schema fragment %#v: %w", v, err))
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(b, &s); err != nil {
		panic(fmt.Errorf("mcpapi: invalid schema fragment %s: %w", b, err))
	}
	return &s
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("mcpapi: cannot marshal default value %#v: %w", v, err))
	}
	return b
}
