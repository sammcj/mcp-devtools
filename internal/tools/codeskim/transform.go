package codeskim

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	sitter "github.com/smacker/go-tree-sitter"
)

const (
	// MaxASTDepth prevents stack overflow from deeply nested code
	MaxASTDepth = 500
	// MaxASTNodes prevents memory exhaustion from large ASTs
	MaxASTNodes = 100000
)

// TransformResult contains transformation output and metadata
type TransformResult struct {
	Transformed   string
	MatchedItems  int
	TotalItems    int
	FilteredItems int
}

// Transform transforms source code by removing implementation details
func Transform(ctx context.Context, source string, lang Language, isTSX bool) (*TransformResult, error) {
	return TransformWithFilter(ctx, source, lang, isTSX, nil)
}

// TransformWithFilter transforms source code and optionally filters by name pattern(s)
// filterPatterns is an array of glob patterns (e.g., ["handle_*", "!temp_*"])
// Pass nil or empty slice for no filtering
func TransformWithFilter(ctx context.Context, source string, lang Language, isTSX bool, filterPatterns []string) (*TransformResult, error) {
	// Get the appropriate tree-sitter language
	var tsLang *sitter.Language
	if lang == LanguageTypeScript && isTSX {
		tsLang = GetTreeSitterLanguageForTSX()
	} else {
		tsLang = GetTreeSitterLanguage(lang)
	}
	if tsLang == nil {
		return nil, fmt.Errorf("failed to get tree-sitter language for %s", lang)
	}

	// Create parser
	parser := sitter.NewParser()
	parser.SetLanguage(tsLang)

	// Parse source code
	sourceBytes := []byte(source)
	tree, err := parser.ParseCtx(ctx, nil, sourceBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source code: %w", err)
	}
	defer tree.Close()

	if tree.RootNode() == nil {
		return nil, fmt.Errorf("failed to parse source code: no root node")
	}

	// Transform by stripping function bodies (with optional filtering)
	return transformStructure(sourceBytes, tree, lang, filterPatterns)
}

// transformStructure strips function/method bodies while preserving structure
func transformStructure(source []byte, tree *sitter.Tree, lang Language, filterPatterns []string) (*TransformResult, error) {
	nodeTypes := GetNodeTypes(lang)
	bodyTypes := GetBodyNodeTypes(lang)

	// Map to store byte ranges to replace: (start, end) -> replacement
	replacements := make(map[[2]uint32]string)

	// Count total items (functions/methods/classes)
	totalItems := countItems(tree.RootNode(), nodeTypes, 0)

	// If filters are provided, collect matching nodes to keep
	var matchingNodes map[*sitter.Node]bool
	matchedItems := 0
	if len(filterPatterns) > 0 {
		matchingNodes = make(map[*sitter.Node]bool)
		matchedItems = collectMatchingNodes(tree.RootNode(), nodeTypes, source, filterPatterns, matchingNodes, 0)
	} else {
		matchedItems = totalItems // No filter = all items match
	}

	// Recursively collect body nodes to replace
	if err := collectBodyReplacements(tree.RootNode(), nodeTypes, bodyTypes, replacements, matchingNodes, 0); err != nil {
		return nil, err
	}

	// Check node count limit
	if len(replacements) > MaxASTNodes {
		return nil, fmt.Errorf("too many AST nodes: %d (max: %d) - possible malicious input", len(replacements), MaxASTNodes)
	}

	// Build output by replacing bodies
	transformed, err := buildOutput(source, replacements)
	if err != nil {
		return nil, err
	}

	return &TransformResult{
		Transformed:   transformed,
		MatchedItems:  matchedItems,
		TotalItems:    totalItems,
		FilteredItems: totalItems - matchedItems,
	}, nil
}

// countItems counts total functions/methods/classes in the AST
func countItems(node *sitter.Node, nodeTypes NodeTypes, depth int) int {
	// Prevent stack overflow
	if depth >= MaxASTDepth {
		return 0
	}

	count := 0
	nodeType := node.Type()

	// Check if this is a function/method/class
	if nodeType == nodeTypes.Function || nodeType == nodeTypes.Method || nodeType == nodeTypes.Class ||
		nodeType == "arrow_function" || nodeType == "function_expression" {
		count++
	}

	// Recursively count children
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child != nil {
			count += countItems(child, nodeTypes, depth+1)
		}
	}

	return count
}

// collectMatchingNodes finds nodes whose names match the filter patterns
// Returns the count of matched nodes
// Filter patterns starting with ! are exclusion patterns (inverse filter)
func collectMatchingNodes(node *sitter.Node, nodeTypes NodeTypes, source []byte, filterPatterns []string, matching map[*sitter.Node]bool, depth int) int {
	// Prevent stack overflow
	if depth >= MaxASTDepth {
		return 0
	}

	matchCount := 0
	nodeType := node.Type()

	// Check if this is a function/method/class
	if nodeType == nodeTypes.Function || nodeType == nodeTypes.Method || nodeType == nodeTypes.Class ||
		nodeType == "arrow_function" || nodeType == "function_expression" {
		// Extract name
		name := extractNodeName(node, source)
		if name != "" {
			// Check against all filter patterns
			// Start with matched = false if there are inclusion patterns, true if only exclusion patterns
			hasInclusionPatterns := false
			for _, pattern := range filterPatterns {
				if !strings.HasPrefix(pattern, "!") {
					hasInclusionPatterns = true
					break
				}
			}

			matched := !hasInclusionPatterns // If only exclusions, start with true

			// Apply patterns in order - exclusions have priority
			for _, pattern := range filterPatterns {
				if strings.HasPrefix(pattern, "!") {
					// Exclusion pattern
					exclusionPattern := pattern[1:]
					if exclusionPattern != "" {
						isMatch, _ := doublestar.Match(exclusionPattern, name)
						if isMatch {
							matched = false
							break // Exclusion wins immediately
						}
					}
				} else {
					// Inclusion pattern
					isMatch, _ := doublestar.Match(pattern, name)
					if isMatch {
						matched = true
					}
				}
			}

			if matched {
				matching[node] = true
				matchCount++
			}
		}
	}

	// Recursively process children
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child != nil {
			matchCount += collectMatchingNodes(child, nodeTypes, source, filterPatterns, matching, depth+1)
		}
	}

	return matchCount
}

// extractNodeName extracts the name identifier from a function/method/class node
func extractNodeName(node *sitter.Node, source []byte) string {
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		// Look for identifier or name nodes
		childType := child.Type()
		if childType == "identifier" || childType == "name" || childType == "property_identifier" {
			return string(source[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// collectBodyReplacements recursively finds function/method bodies to replace
func collectBodyReplacements(node *sitter.Node, nodeTypes NodeTypes, bodyTypes []string, replacements map[[2]uint32]string, matchingNodes map[*sitter.Node]bool, depth int) error {
	// Prevent stack overflow
	if depth >= MaxASTDepth {
		return fmt.Errorf("maximum AST depth exceeded: %d (possible deeply nested code)", MaxASTDepth)
	}

	nodeType := node.Type()

	// Check if this is a function/method/class with a body
	if nodeType == nodeTypes.Function || nodeType == nodeTypes.Method || nodeType == "arrow_function" || nodeType == "function_expression" {
		// If filter is active, skip nodes that don't match
		if matchingNodes != nil && !matchingNodes[node] {
			// Skip this node - don't include it in output
			// Mark entire node for removal
			start := node.StartByte()
			end := node.EndByte()
			replacements[[2]uint32{start, end}] = ""
			return nil // Don't traverse children
		}

		// Node matches filter (or no filter) - strip body only
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
			if err := collectBodyReplacements(child, nodeTypes, bodyTypes, replacements, matchingNodes, depth+1); err != nil {
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

		// For filtered-out nodes (empty replacement), remove the entire node including trailing newline
		if r.replacement == "" {
			// Find the end of the line after this node
			endPos := r.end
			for endPos < uint32(len(source)) && source[endPos] != '\n' {
				endPos++
			}
			// Skip the newline too if present
			if endPos < uint32(len(source)) && source[endPos] == '\n' {
				endPos++
			}
			// Also trim leading whitespace/newlines before the node if it's on its own line
			startPos := r.start
			if startPos > 0 {
				// Look back to see if there's only whitespace before this node on the line
				lineStart := startPos
				for lineStart > 0 && source[lineStart-1] != '\n' {
					lineStart--
				}
				// Check if everything between lineStart and startPos is whitespace
				allWhitespace := true
				for i := lineStart; i < startPos; i++ {
					if source[i] != ' ' && source[i] != '\t' {
						allWhitespace = false
						break
					}
				}
				if allWhitespace {
					startPos = lineStart
				}
			}
			// Copy everything before this node
			result.Write(source[lastPos:startPos])
			lastPos = endPos
		} else {
			// Normal replacement - copy everything before and add replacement
			result.Write(source[lastPos:r.start])
			result.WriteString(r.replacement)
			lastPos = r.end
		}
	}

	// Copy remaining source
	result.Write(source[lastPos:])

	return result.String(), nil
}
