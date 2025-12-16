//go:build cgo && (darwin || (linux && amd64))

package codeskim

import (
	"slices"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// ExtractFileGraph extracts relationships from a parsed AST
func ExtractFileGraph(tree *sitter.Tree, source []byte, lang Language) *FileGraph {
	if tree == nil || tree.RootNode() == nil {
		return nil
	}

	graph := &FileGraph{
		Imports:   make([]string, 0),
		Functions: make([]FunctionInfo, 0),
		Classes:   make([]ClassInfo, 0),
	}

	root := tree.RootNode()

	// Extract imports
	graph.Imports = collectImports(root, source, lang, 0)

	// Extract functions with their calls
	graph.Functions = collectFunctions(root, source, lang, 0)

	// Extract classes with inheritance
	graph.Classes = collectClasses(root, source, lang, 0)

	// Calculate connectivity for functions
	calculateConnectivity(graph)

	return graph
}

// collectImports finds all import statements in the AST
func collectImports(node *sitter.Node, source []byte, lang Language, depth int) []string {
	if depth >= MaxASTDepth {
		return nil
	}

	imports := make([]string, 0)
	importTypes := getImportNodeTypes(lang)

	nodeType := node.Type()

	// Check if this is an import node
	if slices.Contains(importTypes, nodeType) {
		importStr := extractImportName(node, source, lang)
		if importStr != "" {
			imports = append(imports, importStr)
		}
		// Don't recurse into import nodes
		return imports
	}

	// Recurse into children
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child != nil {
			childImports := collectImports(child, source, lang, depth+1)
			imports = append(imports, childImports...)
		}
	}

	return imports
}

// extractImportName extracts the imported module/package name from an import node
func extractImportName(node *sitter.Node, source []byte, lang Language) string {
	switch lang {
	case LanguagePython:
		return extractPythonImport(node, source)
	case LanguageGo:
		return extractGoImport(node, source)
	case LanguageJavaScript, LanguageTypeScript:
		return extractJSImport(node, source)
	case LanguageRust:
		return extractRustImport(node, source)
	case LanguageJava:
		return extractJavaImport(node, source)
	default:
		return ""
	}
}

func extractPythonImport(node *sitter.Node, source []byte) string {
	// Python: import_statement or import_from_statement
	// Look for dotted_name or module child
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "dotted_name" || childType == "module" {
			return string(source[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

func extractGoImport(node *sitter.Node, source []byte) string {
	// Go: import_declaration contains import_spec(s)
	// Look for interpreted_string_literal
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "import_spec" {
			// Recurse into import_spec
			return extractGoImport(child, source)
		}
		if childType == "interpreted_string_literal" {
			// Remove quotes
			raw := string(source[child.StartByte():child.EndByte()])
			return strings.Trim(raw, "\"")
		}
		if childType == "import_spec_list" {
			// Handle multiple imports - we'll just get the first for now
			// The caller will need to handle multiple
			return extractGoImport(child, source)
		}
	}
	return ""
}

func extractJSImport(node *sitter.Node, source []byte) string {
	// JS/TS: import_statement contains string for the module
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "string" {
			// Remove quotes
			raw := string(source[child.StartByte():child.EndByte()])
			return strings.Trim(raw, "\"'`")
		}
	}
	return ""
}

func extractRustImport(node *sitter.Node, source []byte) string {
	// Rust: use_declaration contains a path
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "scoped_identifier" || childType == "identifier" || childType == "use_list" {
			return string(source[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

func extractJavaImport(node *sitter.Node, source []byte) string {
	// Java: import_declaration contains scoped_identifier
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "scoped_identifier" {
			return string(source[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// collectFunctions finds all functions and their calls
func collectFunctions(node *sitter.Node, source []byte, lang Language, depth int) []FunctionInfo {
	if depth >= MaxASTDepth {
		return nil
	}

	functions := make([]FunctionInfo, 0)
	nodeTypes := GetNodeTypes(lang)
	nodeType := node.Type()

	// Check if this is a function/method
	if nodeType == nodeTypes.Function || nodeType == nodeTypes.Method ||
		nodeType == "arrow_function" || nodeType == "function_expression" {
		name := extractNodeNameFromNode(node, source)
		if name != "" {
			calls := collectCallsInNode(node, source, lang, 0)
			signature := extractFunctionSignature(node, source, lang)
			line := int(node.StartPoint().Row) + 1 // 1-based line number
			functions = append(functions, FunctionInfo{
				Name:      name,
				Signature: signature,
				Line:      line,
				Calls:     calls,
			})
		}
	}

	// Recurse into children
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child != nil {
			childFunctions := collectFunctions(child, source, lang, depth+1)
			functions = append(functions, childFunctions...)
		}
	}

	return functions
}

// extractFunctionSignature extracts the full function signature (before the body)
func extractFunctionSignature(node *sitter.Node, source []byte, lang Language) string {
	// Find the body node - we want everything before it
	bodyTypes := getBodyNodeTypes(lang)
	var bodyStart uint32

	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if slices.Contains(bodyTypes, childType) {
			bodyStart = child.StartByte()
		}
		if bodyStart > 0 {
			break
		}
	}

	// Extract signature text
	var sigEnd uint32
	if bodyStart > 0 {
		sigEnd = bodyStart
	} else {
		// No body found, use whole node
		sigEnd = node.EndByte()
	}

	sigStart := node.StartByte()
	if sigEnd <= sigStart || sigEnd > uint32(len(source)) {
		return ""
	}

	// Extract and clean up the signature
	sig := strings.TrimSpace(string(source[sigStart:sigEnd]))

	// Remove trailing whitespace and opening brace
	sig = strings.TrimSuffix(sig, "{")
	sig = strings.TrimSuffix(sig, ":")
	sig = strings.TrimSpace(sig)

	// Collapse internal whitespace
	sig = strings.Join(strings.Fields(sig), " ")

	return sig
}

// getBodyNodeTypes returns the node types for function bodies
func getBodyNodeTypes(lang Language) []string {
	switch lang {
	case LanguagePython:
		return []string{"block"}
	case LanguageGo:
		return []string{"block"}
	case LanguageJavaScript, LanguageTypeScript:
		return []string{"statement_block", "expression"}
	case LanguageRust:
		return []string{"block"}
	case LanguageJava:
		return []string{"block"}
	case LanguageC, LanguageCPP:
		return []string{"compound_statement"}
	case LanguageSwift:
		return []string{"code_block"}
	default:
		return []string{"block", "statement_block", "compound_statement"}
	}
}

// extractNodeNameFromNode extracts the name from a function/method/class node
func extractNodeNameFromNode(node *sitter.Node, source []byte) string {
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "identifier" || childType == "name" || childType == "property_identifier" {
			return string(source[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// collectCallsInNode finds all function calls within a node
func collectCallsInNode(node *sitter.Node, source []byte, lang Language, depth int) []string {
	if depth >= MaxASTDepth {
		return nil
	}

	calls := make([]string, 0)
	callTypes := getCallNodeTypes(lang)
	nodeType := node.Type()

	// Check if this is a call expression
	for _, callType := range callTypes {
		if nodeType == callType {
			callName := extractCallName(node, source, lang)
			if callName != "" {
				calls = append(calls, callName)
			}
		}
	}

	// Recurse into children
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child != nil {
			childCalls := collectCallsInNode(child, source, lang, depth+1)
			calls = append(calls, childCalls...)
		}
	}

	return uniqueStrings(calls)
}

// extractCallName extracts the function name from a call expression
func extractCallName(node *sitter.Node, source []byte, lang Language) string {
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()

		// Direct function call
		if childType == "identifier" || childType == "name" {
			return string(source[child.StartByte():child.EndByte()])
		}

		// Method call (e.g., obj.method())
		if childType == "member_expression" || childType == "attribute" ||
			childType == "selector_expression" || childType == "field_expression" {
			// Get the method name (last identifier in the chain)
			return extractLastIdentifier(child, source)
		}

		// For Python 'call' nodes, the function is the first child
		if lang == LanguagePython && i == 0 {
			if childType == "identifier" {
				return string(source[child.StartByte():child.EndByte()])
			}
			if childType == "attribute" {
				return extractLastIdentifier(child, source)
			}
		}
	}
	return ""
}

// extractLastIdentifier gets the last identifier in a member expression chain
func extractLastIdentifier(node *sitter.Node, source []byte) string {
	var lastIdent string
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "identifier" || childType == "property_identifier" || childType == "field_identifier" {
			lastIdent = string(source[child.StartByte():child.EndByte()])
		}
	}
	return lastIdent
}

// collectClasses finds all classes with their inheritance
func collectClasses(node *sitter.Node, source []byte, lang Language, depth int) []ClassInfo {
	if depth >= MaxASTDepth {
		return nil
	}

	classes := make([]ClassInfo, 0)
	nodeTypes := GetNodeTypes(lang)
	nodeType := node.Type()

	// Check if this is a class
	if nodeType == nodeTypes.Class && nodeTypes.Class != "" {
		name := extractNodeNameFromNode(node, source)
		if name != "" {
			extends, implements := extractInheritance(node, source, lang)
			methods := extractClassMethods(node, source, lang)
			classes = append(classes, ClassInfo{
				Name:       name,
				Extends:    extends,
				Implements: implements,
				Methods:    methods,
			})
		}
	}

	// Recurse into children
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child != nil {
			childClasses := collectClasses(child, source, lang, depth+1)
			classes = append(classes, childClasses...)
		}
	}

	return classes
}

// extractInheritance extracts extends and implements from a class node
func extractInheritance(node *sitter.Node, source []byte, lang Language) (string, []string) {
	var extends string
	implements := make([]string, 0)

	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()

		switch lang {
		case LanguagePython:
			// Python: class Foo(Bar, Baz): - argument_list contains bases
			if childType == "argument_list" {
				bases := extractPythonBases(child, source)
				if len(bases) > 0 {
					extends = bases[0]
					if len(bases) > 1 {
						implements = bases[1:]
					}
				}
			}
		case LanguageJavaScript, LanguageTypeScript:
			// JS/TS: class_heritage contains extends_clause and implements_clause
			if childType == "class_heritage" {
				extends, implements = extractJSInheritance(child, source)
			}
		case LanguageJava:
			// Java: superclass and interfaces
			if childType == "superclass" {
				extends = extractFirstIdentifier(child, source)
			}
			if childType == "interfaces" {
				implements = extractInterfaces(child, source)
			}
		case LanguageRust:
			// Rust: impl_item uses "for" clause for trait implementations
			// Currently not extracting trait implementations from Rust
			_ = childType
		}
	}

	return extends, implements
}

func extractPythonBases(node *sitter.Node, source []byte) []string {
	bases := make([]string, 0)
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "identifier" || childType == "attribute" {
			bases = append(bases, string(source[child.StartByte():child.EndByte()]))
		}
	}
	return bases
}

func extractJSInheritance(node *sitter.Node, source []byte) (string, []string) {
	var extends string
	implements := make([]string, 0)

	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "extends_clause" {
			extends = extractFirstIdentifier(child, source)
		}
		if childType == "implements_clause" {
			implements = extractTypeList(child, source)
		}
	}

	return extends, implements
}

func extractFirstIdentifier(node *sitter.Node, source []byte) string {
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "identifier" || childType == "type_identifier" {
			return string(source[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

func extractTypeList(node *sitter.Node, source []byte) []string {
	types := make([]string, 0)
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "identifier" || childType == "type_identifier" {
			types = append(types, string(source[child.StartByte():child.EndByte()]))
		}
	}
	return types
}

func extractInterfaces(node *sitter.Node, source []byte) []string {
	interfaces := make([]string, 0)
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "type_list" {
			return extractTypeList(child, source)
		}
		if childType == "type_identifier" {
			interfaces = append(interfaces, string(source[child.StartByte():child.EndByte()]))
		}
	}
	return interfaces
}

// extractClassMethods extracts method names from a class body
func extractClassMethods(node *sitter.Node, source []byte, lang Language) []string {
	methods := make([]string, 0)
	nodeTypes := GetNodeTypes(lang)

	var traverse func(n *sitter.Node, depth int)
	traverse = func(n *sitter.Node, depth int) {
		if depth >= MaxASTDepth || n == nil {
			return
		}

		nodeType := n.Type()
		if nodeType == nodeTypes.Method || nodeType == nodeTypes.Function {
			name := extractNodeNameFromNode(n, source)
			if name != "" {
				methods = append(methods, name)
			}
			return // Don't recurse into nested functions
		}

		childCount := int(n.ChildCount())
		for i := range childCount {
			child := n.Child(i)
			if child != nil {
				traverse(child, depth+1)
			}
		}
	}

	// Start from class body
	childCount := int(node.ChildCount())
	for i := range childCount {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		// Look for the body/block of the class
		if childType == "block" || childType == "class_body" || childType == "declaration_list" {
			traverse(child, 0)
		}
	}

	return methods
}

// calculateConnectivity sets the connectivity rating for each function
func calculateConnectivity(graph *FileGraph) {
	for i := range graph.Functions {
		// Connectivity = number of calls made + times called by others
		callsMade := len(graph.Functions[i].Calls)
		calledByCount := 0

		// Count how many other functions call this one
		for j := range graph.Functions {
			if i != j {
				for _, call := range graph.Functions[j].Calls {
					if call == graph.Functions[i].Name {
						calledByCount++
					}
				}
			}
		}

		graph.Functions[i].Connectivity = callsMade + calledByCount
	}
}

// getImportNodeTypes returns the node types for import statements
func getImportNodeTypes(lang Language) []string {
	switch lang {
	case LanguagePython:
		return []string{"import_statement", "import_from_statement"}
	case LanguageGo:
		return []string{"import_declaration"}
	case LanguageJavaScript, LanguageTypeScript:
		return []string{"import_statement"}
	case LanguageRust:
		return []string{"use_declaration"}
	case LanguageJava:
		return []string{"import_declaration"}
	default:
		return []string{}
	}
}

// getCallNodeTypes returns the node types for function calls
func getCallNodeTypes(lang Language) []string {
	switch lang {
	case LanguagePython:
		return []string{"call"}
	case LanguageGo:
		return []string{"call_expression"}
	case LanguageJavaScript, LanguageTypeScript:
		return []string{"call_expression"}
	case LanguageRust:
		return []string{"call_expression", "macro_invocation"}
	case LanguageJava:
		return []string{"method_invocation"}
	default:
		return []string{}
	}
}

// uniqueStrings removes duplicates from a string slice
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
