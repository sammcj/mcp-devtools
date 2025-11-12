package codeskim

import (
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"
)

// DetectLanguage detects the programming language from a file path
func DetectLanguage(filePath string) (Language, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".py":
		return LanguagePython, nil
	case ".go":
		return LanguageGo, nil
	case ".js", ".jsx":
		return LanguageJavaScript, nil
	case ".ts":
		return LanguageTypeScript, nil
	case ".tsx":
		return LanguageTypeScript, nil
	case ".rs":
		return LanguageRust, nil
	case ".c", ".h":
		return LanguageC, nil
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".hh":
		return LanguageCPP, nil
	case ".sh", ".bash":
		return LanguageBash, nil
	case ".html", ".htm":
		return LanguageHTML, nil
	case ".css":
		return LanguageCSS, nil
	case ".swift":
		return LanguageSwift, nil
	case ".java":
		return LanguageJava, nil
	case ".yml", ".yaml":
		return LanguageYAML, nil
	case ".hcl", ".tf":
		return LanguageHCL, nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// ValidateLanguage checks if a language string is valid
func ValidateLanguage(lang string) (Language, error) {
	switch Language(lang) {
	case LanguagePython, LanguageGo, LanguageJavaScript, LanguageTypeScript,
		LanguageRust, LanguageC, LanguageCPP, LanguageBash, LanguageHTML,
		LanguageCSS, LanguageSwift, LanguageJava, LanguageYAML, LanguageHCL:
		return Language(lang), nil
	default:
		return "", fmt.Errorf("unsupported language: %s", lang)
	}
}

// GetTreeSitterLanguage returns the tree-sitter language for a given language
func GetTreeSitterLanguage(lang Language) *sitter.Language {
	switch lang {
	case LanguagePython:
		return python.GetLanguage()
	case LanguageGo:
		return golang.GetLanguage()
	case LanguageJavaScript:
		return javascript.GetLanguage()
	case LanguageTypeScript:
		return typescript.GetLanguage()
	case LanguageRust:
		return rust.GetLanguage()
	case LanguageC:
		return c.GetLanguage()
	case LanguageCPP:
		return cpp.GetLanguage()
	case LanguageBash:
		return bash.GetLanguage()
	case LanguageHTML:
		return html.GetLanguage()
	case LanguageCSS:
		return css.GetLanguage()
	case LanguageSwift:
		return swift.GetLanguage()
	case LanguageJava:
		return java.GetLanguage()
	case LanguageYAML:
		return yaml.GetLanguage()
	case LanguageHCL:
		return hcl.GetLanguage()
	default:
		return nil
	}
}

// GetTreeSitterLanguageForTSX returns the TSX-specific tree-sitter language
func GetTreeSitterLanguageForTSX() *sitter.Language {
	return tsx.GetLanguage()
}

// GetNodeTypes returns the language-specific node types for AST traversal
func GetNodeTypes(lang Language) NodeTypes {
	switch lang {
	case LanguagePython:
		return NodeTypes{
			Function: "function_definition",
			Method:   "function_definition",
			Class:    "class_definition",
		}
	case LanguageGo:
		return NodeTypes{
			Function: "function_declaration",
			Method:   "method_declaration",
			Class:    "type_declaration",
		}
	case LanguageJavaScript, LanguageTypeScript:
		return NodeTypes{
			Function: "function_declaration",
			Method:   "method_definition",
			Class:    "class_declaration",
		}
	case LanguageRust:
		return NodeTypes{
			Function: "function_item",
			Method:   "function_item",
			Class:    "impl_item",
		}
	case LanguageC:
		return NodeTypes{
			Function: "function_definition",
			Method:   "function_definition",
			Class:    "struct_specifier",
		}
	case LanguageCPP:
		return NodeTypes{
			Function: "function_definition",
			Method:   "function_definition",
			Class:    "class_specifier",
		}
	case LanguageBash:
		return NodeTypes{
			Function: "function_definition",
			Method:   "function_definition",
			Class:    "",
		}
	case LanguageHTML:
		return NodeTypes{
			Function: "script_element",
			Method:   "",
			Class:    "",
		}
	case LanguageCSS:
		return NodeTypes{
			Function: "rule_set",
			Method:   "",
			Class:    "",
		}
	case LanguageSwift:
		return NodeTypes{
			Function: "function_declaration",
			Method:   "function_declaration",
			Class:    "class_declaration",
		}
	case LanguageJava:
		return NodeTypes{
			Function: "method_declaration",
			Method:   "method_declaration",
			Class:    "class_declaration",
		}
	case LanguageYAML, LanguageHCL:
		// YAML and HCL don't have functions in traditional sense
		return NodeTypes{}
	default:
		return NodeTypes{}
	}
}

// GetBodyNodeTypes returns the node types that represent function/method bodies
func GetBodyNodeTypes(lang Language) []string {
	switch lang {
	case LanguagePython:
		return []string{"block"}
	case LanguageGo:
		return []string{"block"}
	case LanguageJavaScript, LanguageTypeScript:
		return []string{"statement_block"}
	case LanguageRust:
		return []string{"block"}
	case LanguageC, LanguageCPP:
		return []string{"compound_statement"}
	case LanguageBash:
		return []string{"compound_statement"}
	case LanguageHTML:
		return []string{"raw_text"}
	case LanguageCSS:
		return []string{"block"}
	case LanguageSwift:
		return []string{"function_body"}
	case LanguageJava:
		return []string{"block"}
	case LanguageYAML, LanguageHCL:
		return []string{}
	default:
		return []string{}
	}
}
