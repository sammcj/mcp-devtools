// Package cli provides a direct command-line interface to mcp-devtools tools,
// bypassing the MCP server entirely. Tools are invoked in-process via the
// existing registry, so no server or network round-trip is needed.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// OutputFormat controls how tool results are rendered.
type OutputFormat string

const (
	OutputText OutputFormat = "text"
	OutputJSON OutputFormat = "json"
)

// Runner executes CLI commands against the tool registry.
type Runner struct {
	logger *logrus.Logger
	cache  *sync.Map
	output OutputFormat
}

// NewRunner creates a Runner that uses the given logger, cache, and output format.
func NewRunner(logger *logrus.Logger, cache *sync.Map, output OutputFormat) *Runner {
	return &Runner{logger: logger, cache: cache, output: output}
}

// ListTools prints all enabled tools with their descriptions.
func (r *Runner) ListTools() error {
	tools := registry.GetEnabledTools()

	type entry struct {
		name string
		desc string
	}
	entries := make([]entry, 0, len(tools))
	for _, t := range tools {
		def := t.Definition()
		entries = append(entries, entry{name: def.Name, desc: firstLine(def.Description)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	if r.output == OutputJSON {
		type jsonEntry struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		out := make([]jsonEntry, len(entries))
		for i, e := range entries {
			out[i] = jsonEntry{Name: e.name, Description: e.desc}
		}
		return writeJSON(os.Stdout, out)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\n", e.name, e.desc)
	}
	return w.Flush()
}

// HelpTool prints the schema and usage information for a single tool.
func (r *Runner) HelpTool(name string) error {
	resolved, found := resolveTool(name)
	if !found {
		return fmt.Errorf("unknown tool: %s", name)
	}
	tool, ok := registry.GetTool(resolved)
	if !ok {
		return fmt.Errorf("unknown tool: %s", name)
	}

	def := tool.Definition()

	if r.output == OutputJSON {
		return writeJSON(os.Stdout, def)
	}

	fmt.Fprintf(os.Stdout, "Tool: %s\n\n", def.Name)
	if def.Description != "" {
		fmt.Fprintf(os.Stdout, "%s\n\n", def.Description)
	}

	props := def.InputSchema.Properties
	required := toSet(def.InputSchema.Required)

	if len(props) == 0 {
		fmt.Fprintln(os.Stdout, "No parameters.")
		return nil
	}

	fmt.Fprintln(os.Stdout, "Parameters:")

	// Sort parameter names for stable output
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	slices.Sort(names)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, pName := range names {
		pVal := props[pName]
		pMap, ok := pVal.(map[string]any)
		if !ok {
			continue
		}

		pType, _ := pMap["type"].(string)
		pDesc, _ := pMap["description"].(string)

		reqMark := ""
		if required[pName] {
			reqMark = " (required)"
		}

		enumVals := formatEnum(pMap)

		fmt.Fprintf(w, "  --%s\t%s\t%s%s%s\n", toFlagName(pName), pType, firstLine(pDesc), reqMark, enumVals)
	}
	return w.Flush()
}

// RunTool executes a tool by name with the given arguments.
// args can be:
//   - A single JSON string: '{"key": "value"}'
//   - Flag-style arguments: --key=value --flag
//   - Mixed: --key=value '{"other": "json"}'  (flags take precedence)
func (r *Runner) RunTool(ctx context.Context, name string, args []string) error {
	resolved, found := resolveTool(name)
	if !found {
		return fmt.Errorf("unknown tool: %s (run 'mcp-devtools cli list' to see available tools)", name)
	}
	tool, ok := registry.GetTool(resolved)
	if !ok {
		return fmt.Errorf("unknown tool: %s (run 'mcp-devtools cli list' to see available tools)", name)
	}

	def := tool.Definition()

	params, err := parseArgs(args, def)
	if err != nil {
		return fmt.Errorf("argument error: %w", err)
	}

	result, err := tool.Execute(ctx, r.logger, r.cache, params)
	if err != nil {
		return fmt.Errorf("tool error: %w", err)
	}

	return r.renderResult(result)
}

// parseArgs converts CLI arguments into a map[string]any suitable for tool.Execute().
// Supports JSON input, --key=value flags, and --flag (boolean true).
func parseArgs(args []string, def mcp.Tool) (map[string]any, error) {
	params := make(map[string]any)

	// Build schema lookups for type coercion and flag→param name resolution
	schema := buildSchemaInfo(def)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// JSON object argument
		if strings.HasPrefix(arg, "{") {
			var obj map[string]any
			if err := json.Unmarshal([]byte(arg), &obj); err != nil {
				return nil, fmt.Errorf("invalid JSON argument: %w", err)
			}
			// JSON values merge in (earlier flags take precedence)
			for k, v := range obj {
				if _, exists := params[k]; !exists {
					params[k] = v
				}
			}
			continue
		}

		// Flag-style argument
		if strings.HasPrefix(arg, "--") {
			key, val, err := parseFlag(arg, args, &i, schema)
			if err != nil {
				return nil, err
			}
			params[key] = val
			continue
		}

		return nil, fmt.Errorf("unexpected argument: %s (use --key=value flags or pass a JSON object)", arg)
	}

	return params, nil
}

// schemaInfo holds resolved schema information for argument parsing.
type schemaInfo struct {
	// typeMap maps actual parameter names to their JSON Schema types
	typeMap map[string]string
	// flagToParam maps kebab-case flag names to actual parameter names
	flagToParam map[string]string
}

// parseFlag parses a single --key=value or --key value or --flag (bool true).
func parseFlag(arg string, args []string, idx *int, schema schemaInfo) (string, any, error) {
	stripped := strings.TrimPrefix(arg, "--")

	// --key=value
	if flagName, rawVal, found := strings.Cut(stripped, "="); found {
		paramName := schema.resolveParam(flagName)
		return paramName, coerceValue(rawVal, schema.typeMap[paramName]), nil
	}

	// --flag (boolean shorthand) or --key value
	flagName := stripped
	paramName := schema.resolveParam(flagName)

	// If the schema says this is a boolean, treat bare --flag as true
	if schema.typeMap[paramName] == "boolean" {
		return paramName, true, nil
	}

	// Otherwise consume the next arg as the value
	*idx++
	if *idx >= len(args) {
		return "", nil, fmt.Errorf("flag --%s requires a value", flagName)
	}
	return paramName, coerceValue(args[*idx], schema.typeMap[paramName]), nil
}

// resolveParam converts a kebab-case flag name to the actual parameter name
// by checking against known schema property names. Falls back to snake_case.
func (s schemaInfo) resolveParam(flagName string) string {
	if actual, ok := s.flagToParam[flagName]; ok {
		return actual
	}
	// Fallback: kebab to snake_case
	return strings.ReplaceAll(flagName, "-", "_")
}

// buildSchemaInfo extracts parameter types and builds a flag→param name mapping
// from the tool definition. Handles both snake_case and camelCase parameter names.
func buildSchemaInfo(def mcp.Tool) schemaInfo {
	info := schemaInfo{
		typeMap:     make(map[string]string, len(def.InputSchema.Properties)),
		flagToParam: make(map[string]string, len(def.InputSchema.Properties)),
	}
	for name, prop := range def.InputSchema.Properties {
		if pm, ok := prop.(map[string]any); ok {
			if t, ok := pm["type"].(string); ok {
				info.typeMap[name] = t
			}
		}
		// Map the kebab-case version of this param name back to the original
		kebab := toFlagName(name)
		info.flagToParam[kebab] = name
	}
	return info
}

// coerceValue converts a string value to the appropriate Go type based on JSON Schema type.
func coerceValue(raw, schemaType string) any {
	switch schemaType {
	case "number", "integer":
		// Try integer first
		var i int64
		if _, err := fmt.Sscanf(raw, "%d", &i); err == nil && fmt.Sprintf("%d", i) == raw {
			return i
		}
		// Try float
		var f float64
		if _, err := fmt.Sscanf(raw, "%g", &f); err == nil && fmt.Sprintf("%g", f) == raw {
			return f
		}
		return raw
	case "boolean":
		switch strings.ToLower(raw) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
		return raw
	case "array":
		// Try JSON array
		var arr []any
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			return arr
		}
		// Comma-separated fallback
		return strings.Split(raw, ",")
	case "object":
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			return obj
		}
		return raw
	default:
		return raw
	}
}

// renderResult formats a CallToolResult for terminal output.
func (r *Runner) renderResult(result *mcp.CallToolResult) error {
	if result == nil {
		return nil
	}

	if r.output == OutputJSON {
		return writeJSON(os.Stdout, result)
	}

	// Text mode: extract text content
	for _, content := range result.Content {
		switch c := content.(type) {
		case mcp.TextContent:
			fmt.Fprintln(os.Stdout, c.Text)
		default:
			// Non-text content: render as JSON
			data, err := json.MarshalIndent(c, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stdout, "%+v\n", c)
			} else {
				fmt.Fprintln(os.Stdout, string(data))
			}
		}
	}

	if result.IsError {
		return fmt.Errorf("tool returned an error")
	}
	return nil
}

// resolveTool looks up a tool by name, trying the name as-is first,
// then with hyphens converted to underscores (since CLI users naturally
// type kebab-case but tools are registered with snake_case names).
func resolveTool(name string) (string, bool) {
	if _, ok := registry.GetTool(name); ok {
		return name, true
	}
	// Try kebab → snake_case
	snakeName := strings.ReplaceAll(name, "-", "_")
	if snakeName != name {
		if _, ok := registry.GetTool(snakeName); ok {
			return snakeName, true
		}
	}
	return name, false
}

// --- helpers ---

func writeJSON(w *os.File, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func firstLine(s string) string {
	if before, _, found := strings.Cut(s, "\n"); found {
		return before
	}
	return s
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// toFlagName converts camelCase or snake_case to kebab-case for CLI flags.
func toFlagName(s string) string {
	s = strings.ReplaceAll(s, "_", "-")
	var out strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(r + 32) // toLower
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func formatEnum(pMap map[string]any) string {
	enumRaw, ok := pMap["enum"]
	if !ok {
		return ""
	}
	arr, ok := enumRaw.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	vals := make([]string, len(arr))
	for i, v := range arr {
		vals[i] = fmt.Sprint(v)
	}
	return " [" + strings.Join(vals, "|") + "]"
}
