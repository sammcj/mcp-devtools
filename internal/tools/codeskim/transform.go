package codeskim

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

const (
	// MaxASTDepth prevents stack overflow from deeply nested code
	MaxASTDepth = 500
	// MaxASTNodes prevents memory exhaustion from large ASTs
	MaxASTNodes = 100000
)

// Transform transforms source code by removing implementation details
func Transform(ctx context.Context, source string, lang Language, isTSX bool) (string, error) {
	// Get the appropriate tree-sitter language
	var tsLang *sitter.Language
	if lang == LanguageTypeScript && isTSX {
		tsLang = GetTreeSitterLanguageForTSX()
	} else {
		tsLang = GetTreeSitterLanguage(lang)
	}
	if tsLang == nil {
		return "", fmt.Errorf("failed to get tree-sitter language for %s", lang)
	}

	// Create parser
	parser := sitter.NewParser()
	parser.SetLanguage(tsLang)

	// Parse source code
	sourceBytes := []byte(source)
	tree, err := parser.ParseCtx(ctx, nil, sourceBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse source code: %w", err)
	}
	defer tree.Close()

	if tree.RootNode() == nil {
		return "", fmt.Errorf("failed to parse source code: no root node")
	}

	// Transform by stripping function bodies
	return transformStructure(sourceBytes, tree, lang)
}

// transformStructure strips function/method bodies while preserving structure
func transformStructure(source []byte, tree *sitter.Tree, lang Language) (string, error) {
	nodeTypes := GetNodeTypes(lang)
	bodyTypes := GetBodyNodeTypes(lang)

	// Map to store byte ranges to replace: (start, end) -> replacement
	replacements := make(map[[2]uint32]string)

	// Recursively collect body nodes to replace
	if err := collectBodyReplacements(tree.RootNode(), nodeTypes, bodyTypes, replacements, 0); err != nil {
		return "", err
	}

	// Check node count limit
	if len(replacements) > MaxASTNodes {
		return "", fmt.Errorf("too many AST nodes: %d (max: %d) - possible malicious input", len(replacements), MaxASTNodes)
	}

	// Build output by replacing bodies
	return buildOutput(source, replacements)
}

// collectBodyReplacements recursively finds function/method bodies to replace
func collectBodyReplacements(node *sitter.Node, nodeTypes NodeTypes, bodyTypes []string, replacements map[[2]uint32]string, depth int) error {
	// Prevent stack overflow
	if depth > MaxASTDepth {
		return fmt.Errorf("maximum AST depth exceeded: %d (possible deeply nested code)", MaxASTDepth)
	}

	nodeType := node.Type()

	// Check if this is a function/method/class with a body
	if nodeType == nodeTypes.Function || nodeType == nodeTypes.Method || nodeType == "arrow_function" || nodeType == "function_expression" {
		bodyNode := findBodyNode(node, bodyTypes)
		if bodyNode != nil {
			start := bodyNode.StartByte()
			end := bodyNode.EndByte()
			replacements[[2]uint32{start, end}] = " { /* ... */ }"
		}
	}

	// Recursively process children
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child != nil {
			if err := collectBodyReplacements(child, nodeTypes, bodyTypes, replacements, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

// findBodyNode finds the body node of a function/method
func findBodyNode(node *sitter.Node, bodyTypes []string) *sitter.Node {
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := child.Type()
		if slices.Contains(bodyTypes, childType) {
			return child
		}
	}
	return nil
}

// buildOutput constructs the output string by applying replacements
func buildOutput(source []byte, replacements map[[2]uint32]string) (string, error) {
	if len(replacements) == 0 {
		return string(source), nil
	}

	// Sort replacements by start position
	type replacement struct {
		start       uint32
		end         uint32
		replacement string
	}

	sortedReplacements := make([]replacement, 0, len(replacements))
	for key, val := range replacements {
		sortedReplacements = append(sortedReplacements, replacement{
			start:       key[0],
			end:         key[1],
			replacement: val,
		})
	}

	sort.Slice(sortedReplacements, func(i, j int) bool {
		return sortedReplacements[i].start < sortedReplacements[j].start
	})

	// Build result
	var result strings.Builder
	result.Grow(len(source) + len(replacements)*20) // Preallocate with buffer

	lastPos := uint32(0)

	for _, r := range sortedReplacements {
		// Validate ranges
		if r.end < r.start {
			return "", fmt.Errorf("invalid AST range: start=%d end=%d", r.start, r.end)
		}
		if r.end > uint32(len(source)) {
			return "", fmt.Errorf("AST range exceeds source length: end=%d len=%d", r.end, len(source))
		}

		// Skip overlapping replacements (nested functions handled by parent)
		if r.start < lastPos {
			continue
		}

		// Copy everything before this replacement
		result.Write(source[lastPos:r.start])
		// Add replacement
		result.WriteString(r.replacement)
		lastPos = r.end
	}

	// Copy remaining source
	result.Write(source[lastPos:])

	return result.String(), nil
}
