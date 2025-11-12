package codeskim

import (
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
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
		// TSX uses TypeScript parser with JSX support
		return LanguageTypeScript, nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s (supported: .py, .go, .js, .jsx, .ts, .tsx)", ext)
	}
}

// ValidateLanguage checks if a language string is valid
func ValidateLanguage(lang string) (Language, error) {
	switch Language(lang) {
	case LanguagePython, LanguageGo, LanguageJavaScript, LanguageTypeScript:
		return Language(lang), nil
	default:
		return "", fmt.Errorf("unsupported language: %s (supported: python, go, javascript, typescript)", lang)
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
		// TypeScript parser (not TSX)
		return typescript.GetLanguage()
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
	default:
		return []string{}
	}
}
