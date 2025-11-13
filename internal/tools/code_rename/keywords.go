package code_rename

import "slices"

// languageKeywords maps language names to their reserved keywords
var languageKeywords = map[string][]string{
	"go": {
		"break", "case", "chan", "const", "continue",
		"default", "defer", "else", "fallthrough", "for",
		"func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return",
		"select", "struct", "switch", "type", "var",
	},
	"python": {
		"False", "None", "True", "and", "as",
		"assert", "async", "await", "break", "class",
		"continue", "def", "del", "elif", "else",
		"except", "finally", "for", "from", "global",
		"if", "import", "in", "is", "lambda",
		"nonlocal", "not", "or", "pass", "raise",
		"return", "try", "while", "with", "yield",
	},
	"javascript": {
		"await", "break", "case", "catch", "class",
		"const", "continue", "debugger", "default", "delete",
		"do", "else", "enum", "export", "extends",
		"false", "finally", "for", "function", "if",
		"import", "in", "instanceof", "let", "new",
		"null", "return", "static", "super", "switch",
		"this", "throw", "true", "try", "typeof",
		"var", "void", "while", "with", "yield",
	},
	"typescript": {
		"await", "break", "case", "catch", "class",
		"const", "continue", "debugger", "default", "delete",
		"do", "else", "enum", "export", "extends",
		"false", "finally", "for", "function", "if",
		"import", "in", "instanceof", "interface", "let",
		"new", "null", "return", "static", "super",
		"switch", "this", "throw", "true", "try",
		"type", "typeof", "var", "void", "while",
		"with", "yield",
	},
	"rust": {
		"as", "async", "await", "break", "const",
		"continue", "crate", "dyn", "else", "enum",
		"extern", "false", "fn", "for", "if",
		"impl", "in", "let", "loop", "match",
		"mod", "move", "mut", "pub", "ref",
		"return", "self", "Self", "static", "struct",
		"super", "trait", "true", "type", "unsafe",
		"use", "where", "while",
	},
	"bash": {
		"if", "then", "else", "elif", "fi",
		"case", "esac", "for", "select", "while",
		"until", "do", "done", "in", "function",
		"time", "coproc", "!", "[[", "]]",
	},
	"sh": {
		"if", "then", "else", "elif", "fi",
		"case", "esac", "for", "while", "until",
		"do", "done", "in",
	},
	"css": {
		// CSS doesn't have keywords in the traditional sense, but these are reserved properties
		"important", "inherit", "initial", "unset", "revert",
	},
	"scss": {
		// SCSS includes CSS keywords plus directives
		"important", "inherit", "initial", "unset", "revert",
		"include", "mixin", "extend", "if", "else",
		"for", "each", "while", "return", "function",
	},
	"less": {
		// LESS keywords
		"important", "inherit", "initial", "unset", "revert",
		"when", "and", "not", "or",
	},
	"json": {
		// JSON has strict values, not keywords in the traditional sense
		"true", "false", "null",
	},
	"yaml": {
		// YAML reserved indicators
		"true", "false", "null", "yes", "no",
		"on", "off",
	},
	"c": {
		"auto", "break", "case", "char", "const",
		"continue", "default", "do", "double", "else",
		"enum", "extern", "float", "for", "goto",
		"if", "inline", "int", "long", "register",
		"restrict", "return", "short", "signed", "sizeof",
		"static", "struct", "switch", "typedef", "union",
		"unsigned", "void", "volatile", "while",
	},
	"cpp": {
		// C++ keywords include all C keywords plus additional ones
		"alignas", "alignof", "and", "and_eq", "asm",
		"auto", "bitand", "bitor", "bool", "break",
		"case", "catch", "char", "char8_t", "char16_t",
		"char32_t", "class", "compl", "concept", "const",
		"consteval", "constexpr", "constinit", "const_cast", "continue",
		"co_await", "co_return", "co_yield", "decltype", "default",
		"delete", "do", "double", "dynamic_cast", "else",
		"enum", "explicit", "export", "extern", "false",
		"float", "for", "friend", "goto", "if",
		"inline", "int", "long", "mutable", "namespace",
		"new", "noexcept", "not", "not_eq", "nullptr",
		"operator", "or", "or_eq", "private", "protected",
		"public", "register", "reinterpret_cast", "requires", "return",
		"short", "signed", "sizeof", "static", "static_assert",
		"static_cast", "struct", "switch", "template", "this",
		"thread_local", "throw", "true", "try", "typedef",
		"typeid", "typename", "union", "unsigned", "using",
		"virtual", "void", "volatile", "wchar_t", "while",
		"xor", "xor_eq",
	},
	"java": {
		"abstract", "assert", "boolean", "break", "byte",
		"case", "catch", "char", "class", "const",
		"continue", "default", "do", "double", "else",
		"enum", "extends", "final", "finally", "float",
		"for", "goto", "if", "implements", "import",
		"instanceof", "int", "interface", "long", "native",
		"new", "package", "private", "protected", "public",
		"return", "short", "static", "strictfp", "super",
		"switch", "synchronized", "this", "throw", "throws",
		"transient", "try", "void", "volatile", "while",
	},
	"swift": {
		"associatedtype", "class", "deinit", "enum", "extension",
		"fileprivate", "func", "import", "init", "inout",
		"internal", "let", "open", "operator", "private",
		"precedencegroup", "protocol", "public", "rethrows", "static",
		"struct", "subscript", "typealias", "var", "break",
		"case", "catch", "continue", "default", "defer",
		"do", "else", "fallthrough", "for", "guard",
		"if", "in", "repeat", "return", "throw",
		"switch", "where", "while", "as", "false",
		"is", "nil", "self", "Self", "super",
		"throws", "true", "try", "async", "await",
	},
}

// isLanguageKeyword checks if a name is a reserved keyword in the given language
func isLanguageKeyword(language, name string) bool {
	keywords, exists := languageKeywords[language]
	if !exists {
		return false
	}

	return slices.Contains(keywords, name)
}
