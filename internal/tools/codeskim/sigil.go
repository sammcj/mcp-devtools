//go:build cgo && (darwin || (linux && amd64))

package codeskim

import (
	"fmt"
	"slices"
	"strings"
)

// FormatSigil converts a FileResult to compressed sigil notation optimised for LLMs
// Sigil meanings:
//   - ! import/module
//   - $ class/type
//   - # function/method
//   - < extends
//   - & implements
//   - -> calls (outgoing)
//   - ★n connectivity rating
func FormatSigil(result *FileResult) string {
	if result == nil {
		return ""
	}

	var sb strings.Builder

	// Header with file path and language
	fmt.Fprintf(&sb, "# %s [%s]\n", result.Path, result.Language)

	// If we have graph data, use it
	if result.Graph != nil {
		// Imports
		if len(result.Graph.Imports) > 0 {
			for _, imp := range result.Graph.Imports {
				fmt.Fprintf(&sb, "!%s ", imp)
			}
			sb.WriteString("\n")
		}

		// Classes
		for _, class := range result.Graph.Classes {
			sb.WriteString(formatClassSigil(class))
		}

		// Top-level functions (not in classes)
		for _, fn := range result.Graph.Functions {
			// Check if this function is a method of a class
			isMethod := false
			for _, class := range result.Graph.Classes {
				if slices.Contains(class.Methods, fn.Name) {
					isMethod = true
					break
				}
			}

			if !isMethod {
				sb.WriteString(formatFunctionSigil(fn, ""))
			}
		}
	} else {
		// No graph data - just include the transformed code
		sb.WriteString(result.Transformed)
	}

	return sb.String()
}

// formatClassSigil formats a class in sigil notation
func formatClassSigil(class ClassInfo) string {
	var sb strings.Builder

	// Class name with inheritance
	fmt.Fprintf(&sb, "$%s", class.Name)

	if class.Extends != "" {
		fmt.Fprintf(&sb, " < %s", class.Extends)
	}

	if len(class.Implements) > 0 {
		for _, impl := range class.Implements {
			fmt.Fprintf(&sb, " & %s", impl)
		}
	}

	sb.WriteString("\n")

	// Methods (indented)
	for _, method := range class.Methods {
		fmt.Fprintf(&sb, "  #%s()\n", method)
	}

	return sb.String()
}

// formatFunctionSigil formats a function in sigil notation
func formatFunctionSigil(fn FunctionInfo, indent string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s#%s()", indent, fn.Name)

	// Calls
	if len(fn.Calls) > 0 {
		sb.WriteString(" ->")
		for _, call := range fn.Calls {
			fmt.Fprintf(&sb, " #%s", call)
		}
	}

	// Connectivity rating (★)
	if fn.Connectivity > 0 {
		fmt.Fprintf(&sb, " ★%d", fn.Connectivity)
	}

	sb.WriteString("\n")

	return sb.String()
}

// FormatSigilResponse formats an entire SkimResponse in sigil notation
func FormatSigilResponse(response *SkimResponse) string {
	if response == nil || len(response.Files) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("# MCP DevTools - Compressed Codebase\n")
	fmt.Fprintf(&sb, "# %d files\n\n", len(response.Files))

	// Format each file
	for _, file := range response.Files {
		if file.Error != "" {
			fmt.Fprintf(&sb, "# %s [ERROR: %s]\n\n", file.Path, file.Error)
			continue
		}

		sb.WriteString(FormatSigil(&file))
		sb.WriteString("\n")
	}

	return sb.String()
}
