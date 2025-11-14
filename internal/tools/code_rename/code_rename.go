package code_rename

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
	"go.lsp.dev/protocol"
)

// CodeRenameTool implements symbol renaming via LSP
type CodeRenameTool struct{}

// init registers the tool with the registry
func init() {
	registry.Register(&CodeRenameTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *CodeRenameTool) Definition() mcp.Tool {
	// Detect available languages at registration time
	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Silent detection
	availableLangs := GetAvailableLanguages(ctx, logger)

	// Build description with only available languages
	description := "Efficiently and safely rename symbols (variables, functions, types, methods) across codebase using LSP. Handles references, imports, comments. More efficient than individually editing files when renaming across multiple files."
	if len(availableLangs) > 0 {
		description += " Supports: " + strings.Join(availableLangs, ", ")
	} else {
		description += " No LSP servers detected - install language servers to enable renaming."
	}

	return mcp.NewTool(
		"code_rename",
		mcp.WithDescription(description),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Absolute path to file containing the symbol"),
		),
		mcp.WithString("old_name",
			mcp.Required(),
			mcp.Description("Current name of the symbol to rename"),
		),
		mcp.WithString("new_name",
			mcp.Required(),
			mcp.Description("New name for the symbol"),
		),
		mcp.WithBoolean("preview",
			mcp.Description("Return preview without applying changes"),
			mcp.DefaultBool(true),
		),
		mcp.WithNumber("line",
			mcp.Description("Optional 1-based line number for symbol disambiguation"),
		),
		mcp.WithNumber("column",
			mcp.Description("Optional 1-based column number for symbol disambiguation"),
		),
	)
}

// renameParams holds validated parameters for rename operation
type renameParams struct {
	filePath string
	absPath  string
	oldName  string
	newName  string
	preview  bool
	language string
	line     int // optional, 0 means not provided
	column   int // optional, 0 means not provided
}

// validateAndPrepareParams validates and prepares parameters from tool arguments
func validateAndPrepareParams(args map[string]any) (*renameParams, error) {
	// Parse required parameters
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return nil, fmt.Errorf("missing required parameter: file_path")
	}

	oldName, ok := args["old_name"].(string)
	if !ok || oldName == "" {
		return nil, fmt.Errorf("missing required parameter: old_name")
	}

	newName, ok := args["new_name"].(string)
	if !ok || newName == "" {
		return nil, fmt.Errorf("missing required parameter: new_name")
	}

	// Note: We validate identifier name later after detecting language
	// to enable language-specific keyword checking

	// Parse optional parameters
	preview := true
	if previewRaw, ok := args["preview"].(bool); ok {
		preview = previewRaw
	}

	// Parse optional position parameters
	line := 0
	if lineRaw, ok := args["line"].(float64); ok {
		line = int(lineRaw)
		if line < 1 {
			return nil, fmt.Errorf("line must be >= 1")
		}
	}

	column := 0
	if columnRaw, ok := args["column"].(float64); ok {
		column = int(columnRaw)
		if column < 1 {
			return nil, fmt.Errorf("column must be >= 1")
		}
	}

	// Both line and column must be provided together, or neither
	if (line > 0 && column == 0) || (line == 0 && column > 0) {
		return nil, fmt.Errorf("line and column must be provided together for position-based lookup")
	}

	// Make path absolute
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Security: Check file access permission
	if err := security.CheckFileAccess(absPath); err != nil {
		return nil, err
	}

	// Validate file exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("file not found: %s", absPath)
	}

	// Detect language early to fail fast on unsupported file types
	language := DetectLanguage(absPath)
	if language == "" {
		return nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(absPath))
	}

	// Validate new name is a valid identifier with language-specific checks
	if err := validateIdentifierName(newName, language); err != nil {
		return nil, fmt.Errorf("invalid new_name: %w", err)
	}

	return &renameParams{
		filePath: filePath,
		absPath:  absPath,
		oldName:  oldName,
		newName:  newName,
		preview:  preview,
		language: language,
		line:     line,
		column:   column,
	}, nil
}

// symbolPosition holds the position of a symbol in a file
type symbolPosition struct {
	line   int
	column int
}

// validateSymbolAtPosition checks that the given position contains the specified symbol
func validateSymbolAtPosition(filePath, symbolName string, line, column int) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Validate line number
	if line < 1 || line > len(lines) {
		return fmt.Errorf("line %d is out of range (file has %d lines)", line, len(lines))
	}

	lineContent := lines[line-1] // Convert to 0-based

	// Validate column number
	if column < 1 || column > len(lineContent)+1 {
		return fmt.Errorf("column %d is out of range for line %d (line has %d characters)", column, line, len(lineContent))
	}

	// Check if symbol name appears at this position
	symbolPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbolName) + `\b`)
	matches := symbolPattern.FindAllStringIndex(lineContent, -1)

	columnIdx := column - 1 // Convert to 0-based
	for _, match := range matches {
		// Check if this match contains our column position
		if columnIdx >= match[0] && columnIdx < match[1] {
			return nil // Found the symbol at this position
		}
	}

	return fmt.Errorf("symbol '%s' not found at line %d, column %d", symbolName, line, column)
}

// performLSPRename executes the rename operation using LSP client
// If existingClient is provided, it will be reused; otherwise a new client is created
func performLSPRename(
	ctx context.Context,
	logger *logrus.Logger,
	cache *sync.Map,
	params *renameParams,
	pos *symbolPosition,
	existingClient *LSPClient,
) (*protocol.WorkspaceEdit, error) {
	var client *LSPClient
	var err error
	var shouldClose bool

	if existingClient != nil {
		// Reuse provided client
		client = existingClient
		shouldClose = false
		logger.Debug("Reusing existing LSP client")
	} else {
		// Find LSP server for this language
		server, err := FindServerForLanguage(ctx, logger, params.language)
		if err != nil {
			return nil, fmt.Errorf("failed to find LSP server: %w", err)
		}

		if server == nil {
			availableLangs := GetAvailableLanguages(ctx, logger)
			installCmd := getInstallCommand(params.language)
			if len(availableLangs) > 0 {
				return nil, fmt.Errorf("no LSP server available for %s (available languages: %v). Install command: %s", params.language, availableLangs, installCmd)
			}
			return nil, fmt.Errorf("no LSP server available for %s. Install command: %s", params.language, installCmd)
		}

		logger.WithField("server", server.Command).Debug("Found LSP server")

		// Get or create cached LSP client
		client, err = getOrCreateLSPClient(ctx, logger, cache, server, params.absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get LSP client: %w", err)
		}
		shouldClose = false // Don't close cached clients
	}

	if shouldClose {
		defer client.Close()
	}

	// Prepare rename to get current symbol name
	symbol, err := client.PrepareRename(ctx, params.absPath, pos.line, pos.column)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare rename: %w", err)
	}

	logger.WithField("symbol", symbol).Debug("Prepared rename")

	// Perform rename
	workspaceEdit, err := client.Rename(ctx, params.absPath, pos.line, pos.column, params.newName)
	if err != nil {
		return nil, fmt.Errorf("failed to perform rename: %w", err)
	}

	return workspaceEdit, nil
}

// Execute executes the tool's logic
func (t *CodeRenameTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Validate and prepare parameters
	params, err := validateAndPrepareParams(args)
	if err != nil {
		return nil, err
	}

	var pos *symbolPosition
	var client *LSPClient

	// If position is provided, use it directly
	if params.line > 0 && params.column > 0 {
		// Position provided - validate it contains the symbol
		if err := validateSymbolAtPosition(params.absPath, params.oldName, params.line, params.column); err != nil {
			return nil, err
		}
		pos = &symbolPosition{
			line:   params.line,
			column: params.column,
		}
	} else {
		// Need to find symbol position - create LSP client for accurate symbol detection
		server, err := FindServerForLanguage(ctx, logger, params.language)
		if err != nil {
			return nil, fmt.Errorf("failed to find LSP server: %w", err)
		}

		if server == nil {
			availableLangs := GetAvailableLanguages(ctx, logger)
			installCmd := getInstallCommand(params.language)
			if len(availableLangs) > 0 {
				return nil, fmt.Errorf("no LSP server available for %s (available languages: %v). Install command: %s", params.language, availableLangs, installCmd)
			}
			return nil, fmt.Errorf("no LSP server available for %s. Install command: %s", params.language, installCmd)
		}

		// Get or create cached LSP client
		client, err = getOrCreateLSPClient(ctx, logger, cache, server, params.absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create LSP client: %w", err)
		}
		// Don't close client - it's cached

		// Use LSP-validated symbol search
		pos, err = findSymbolPositionWithLSP(ctx, logger, client, params.absPath, params.oldName)
		if err != nil {
			return nil, err
		}
	}

	logger.WithFields(logrus.Fields{
		"file":     params.absPath,
		"old_name": params.oldName,
		"line":     pos.line,
		"column":   pos.column,
		"new_name": params.newName,
		"preview":  params.preview,
	}).Info("Executing code rename")

	logger.WithField("language", params.language).Debug("Detected language")

	// Perform LSP rename operation (passing client if we created one for symbol finding)
	workspaceEdit, err := performLSPRename(ctx, logger, cache, params, pos, client)
	if err != nil {
		return nil, err
	}

	// Convert to our result format
	result, err := convertWorkspaceEdit(workspaceEdit, params.preview)
	if err != nil {
		return nil, fmt.Errorf("failed to convert workspace edit: %w", err)
	}

	// Check if there was an error in the result
	if result.Error != "" {
		return nil, fmt.Errorf("rename failed: %s", result.Error)
	}

	logger.WithFields(logrus.Fields{
		"files_modified": result.FilesModified,
		"replacements":   result.TotalReplacements,
		"applied":        !params.preview,
	}).Info("Rename completed")

	// Apply changes if not preview mode
	if !params.preview {
		applyResult, err := applyWorkspaceEdit(workspaceEdit)
		if err != nil {
			// Merge rollback information into result
			if applyResult != nil {
				result.RolledBack = applyResult.RolledBack
				result.RollbackSuccessful = applyResult.RollbackSuccessful
				result.BackupLocation = applyResult.BackupLocation
				result.FilesReverted = applyResult.FilesReverted
			}
			result.Error = fmt.Sprintf("failed to apply changes: %v", err)
			result.Applied = false
		} else if applyResult != nil {
			// Success - merge apply result
			result.Applied = applyResult.Applied

			// Synchronise modified files with LSP server if we have a client
			if client != nil {
				logger.Debug("Synchronising modified files with LSP server")

				// Get list of modified files from workspace edit
				modifiedFiles := getModifiedFiles(workspaceEdit)

				for _, filePath := range modifiedFiles {
					if err := client.SyncDocument(ctx, filePath); err != nil {
						logger.WithFields(logrus.Fields{
							"file":  filePath,
							"error": err,
						}).Warn("Failed to sync document with LSP server")
						// Don't fail the entire operation, just log the warning
					}
				}

				logger.WithField("files_synced", len(modifiedFiles)).Debug("LSP synchronisation complete")
			}
		}
	}

	// Return result as structured content for better machine readability
	return &mcp.CallToolResult{
		StructuredContent: result,
	}, nil
}

// ProvideExtendedInfo implements the ExtendedHelpProvider interface
func (t *CodeRenameTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Preview rename of a Go function",
				Arguments: map[string]any{
					"file_path": "/Users/dev/project/main.go",
					"old_name":  "processUser",
					"new_name":  "handleUser",
				},
				ExpectedResult: "Returns count of affected files, total changes, and change preview snippets showing before/after for each modification",
			},
			{
				Description: "Apply rename to a Python variable",
				Arguments: map[string]any{
					"file_path": "/Users/dev/app/handlers.py",
					"old_name":  "user_id",
					"new_name":  "userId",
					"preview":   false,
				},
				ExpectedResult: "Applies rename atomically with automatic rollback on failure. Returns 'applied: true' on success or rollback details on failure",
			},
			{
				Description: "Disambiguate symbol by position",
				Arguments: map[string]any{
					"file_path": "/Users/dev/src/api.ts",
					"old_name":  "name",
					"new_name":  "userName",
					"line":      15,
					"column":    10,
				},
				ExpectedResult: "Renames only the 'name' symbol at line 15, column 10, not other 'name' symbols in the file",
			},
		},
		CommonPatterns: []string{
			"Always use preview mode (default) first to verify changes - preview now shows actual change snippets",
			"Tool automatically finds symbol position - just provide the name",
			"For multiple symbols with same name, use line/column parameters to disambiguate",
			"All renames are atomic - if any file fails, all changes are rolled back automatically",
			"Preview mode is token-efficient: shows up to 5 changes per file, 50 total",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Error: 'no LSP server available for <language>'",
				Solution: "Install the required LSP server. Commands are provided in the error message. For Go: 'go install golang.org/x/tools/gopls@latest'. For TypeScript: 'npm install -g typescript-language-server'",
			},
			{
				Problem:  "Error: 'failed to find symbol' or 'symbol not found'",
				Solution: "Ensure the old_name exactly matches the symbol in the file (case-sensitive). Check for typos. If multiple symbols have the same name, use line/column parameters to specify which one",
			},
			{
				Problem:  "Error: 'identifier cannot be a <language> keyword: <name>'",
				Solution: "The new name is a reserved keyword in the detected language. Choose a different name that isn't a language keyword",
			},
			{
				Problem:  "Rename affects more files than expected",
				Solution: "This is normal - LSP finds all references including imports and usages across files. Review the change preview carefully before applying",
			},
			{
				Problem:  "Error: 'file modified since analysis'",
				Solution: "A file was modified by another process during the rename operation. Retry the operation - the tool detected this to prevent incorrect changes",
			},
			{
				Problem:  "Changes rolled back after error",
				Solution: "The tool automatically rolled back all changes when an error occurred. Check the error message and backup_location in the result. All files have been restored to their original state",
			},
		},
		ParameterDetails: map[string]string{
			"file_path": "Absolute path to file containing the symbol. Must exist and be accessible",
			"old_name":  "Current name of the symbol to rename. Must exactly match (case-sensitive). Tool will find its position automatically unless line/column specified",
			"new_name":  "New name for the symbol. Must be a valid identifier (letters, numbers, underscores; cannot start with digit). Cannot be a language keyword",
			"preview":   "When true (default), shows what would change without modifying files including change snippets. When false, applies the rename atomically with automatic rollback on failure",
			"line":      "Optional 1-based line number for symbol disambiguation. Must be used with column parameter. Validates that the symbol exists at this exact position",
			"column":    "Optional 1-based column number for symbol disambiguation. Must be used with line parameter. Allows renaming specific occurrences when multiple symbols share the same name",
		},
		WhenToUse:    "Use when you need to safely rename variables, functions, types, or methods across a codebase. The LSP-based approach ensures all references are found, including cross-file imports and usages. Atomic operations with automatic rollback make this ideal for critical refactoring",
		WhenNotToUse: "Don't use for simple find-replace operations where context doesn't matter. Don't use for renaming across multiple languages (LSP servers are language-specific). For bulk renames or pattern-based changes, standard text tools may be more appropriate",
	}
}

// getModifiedFiles extracts the list of files modified in a workspace edit
func getModifiedFiles(edit *protocol.WorkspaceEdit) []string {
	fileSet := make(map[string]bool)

	// Handle legacy Changes format
	for uriStr := range edit.Changes {
		filePath := uriToPath(string(uriStr))
		fileSet[filePath] = true
	}

	// Handle modern DocumentChanges format
	for _, textDocEdit := range edit.DocumentChanges {
		filePath := uriToPath(string(textDocEdit.TextDocument.URI))
		fileSet[filePath] = true
	}

	// Convert set to slice
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}

	return files
}

// findSymbolPositionWithLSP uses LSP to find and validate symbol positions
// This is more accurate than regex as LSP understands language syntax
func findSymbolPositionWithLSP(
	ctx context.Context,
	logger *logrus.Logger,
	client *LSPClient,
	filePath string,
	symbolName string,
) (*symbolPosition, error) {
	// Read file to find candidate positions
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	symbolPattern, err := regexp.Compile(`\b` + regexp.QuoteMeta(symbolName) + `\b`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex pattern: %w", err)
	}

	// Find all candidate positions
	var candidates []symbolPosition
	for lineIdx, line := range lines {
		matches := symbolPattern.FindAllStringIndex(line, -1)
		for _, match := range matches {
			candidates = append(candidates, symbolPosition{
				line:   lineIdx + 1,  // Convert to 1-based
				column: match[0] + 1, // Convert to 1-based
			})
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("symbol '%s' not found in file", symbolName)
	}

	logger.WithFields(logrus.Fields{
		"symbol":     symbolName,
		"candidates": len(candidates),
	}).Debug("Found symbol candidates, validating with LSP")

	// Open document in LSP
	if err := client.openDocument(ctx, filePath); err != nil {
		return nil, fmt.Errorf("failed to open document: %w", err)
	}

	// Try each candidate with PrepareRename
	for i, candidate := range candidates {
		symbol, err := client.PrepareRename(ctx, filePath, candidate.line, candidate.column)

		if err == nil && symbol != "" {
			// Found valid renameable symbol
			logger.WithFields(logrus.Fields{
				"symbol":    symbol,
				"line":      candidate.line,
				"column":    candidate.column,
				"candidate": i + 1,
				"total":     len(candidates),
			}).Debug("Found valid renameable symbol via LSP")

			return &candidate, nil
		}

		// PrepareRename failed - not a valid symbol (comment, string, etc.)
		logger.WithFields(logrus.Fields{
			"line":      candidate.line,
			"column":    candidate.column,
			"candidate": i + 1,
			"error":     err,
		}).Debug("LSP rejected candidate position")
	}

	return nil, fmt.Errorf("symbol '%s' found %d time(s) in file but none at renameable positions (may be in comments or strings)", symbolName, len(candidates))
}

// validateIdentifierName checks if a name is a valid identifier
// It uses a permissive check that works for most languages
// Also checks language-specific keywords
func validateIdentifierName(name, language string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}

	// Check for whitespace
	for _, ch := range name {
		if unicode.IsSpace(ch) {
			return fmt.Errorf("identifier cannot contain whitespace")
		}
	}

	// Check for common invalid characters (operators, punctuation)
	invalidChars := regexp.MustCompile(`[^\w]`)
	if invalidChars.MatchString(name) {
		return fmt.Errorf("identifier contains invalid characters (only letters, numbers, and underscores allowed)")
	}

	// Check first character isn't a digit
	firstChar := rune(name[0])
	if unicode.IsDigit(firstChar) {
		return fmt.Errorf("identifier cannot start with a digit")
	}

	// Check language-specific keywords
	if isLanguageKeyword(language, name) {
		return fmt.Errorf("identifier cannot be a %s keyword: %s", language, name)
	}

	return nil
}

// getInstallCommand returns the installation command for a language's LSP server
func getInstallCommand(language string) string {
	commands := map[string]string{
		"go":         "go install golang.org/x/tools/gopls@latest",
		"typescript": "npm install -g typescript-language-server",
		"javascript": "npm install -g typescript-language-server",
		"python":     "pip install pyright",
		"rust":       "rustup component add rust-analyzer",
		"bash":       "npm install -g bash-language-server",
		"sh":         "npm install -g bash-language-server",
		"html":       "npm install -g vscode-langservers-extracted",
		"css":        "npm install -g vscode-langservers-extracted",
		"scss":       "npm install -g vscode-langservers-extracted",
		"less":       "npm install -g vscode-langservers-extracted",
		"json":       "npm install -g vscode-langservers-extracted",
		"yaml":       "npm install -g yaml-language-server",
		"c":          "brew install llvm (macOS) or apt install clangd (Linux)",
		"cpp":        "brew install llvm (macOS) or apt install clangd (Linux)",
		"java":       "brew install jdtls (macOS) or download from Eclipse",
		"swift":      "included with Xcode or Swift toolchain",
	}

	if cmd, ok := commands[language]; ok {
		return cmd
	}
	return "see documentation for installation instructions"
}

// applyEditsToFile applies text edits to a single file atomically
// Returns an error on any failure (security, read, apply, or write)
func applyEditsToFile(filePath string, edits []protocol.TextEdit) error {
	// Security: Check file access permission before modification
	if err := security.CheckFileAccess(filePath); err != nil {
		return fmt.Errorf("access denied for %s: %w", filePath, err)
	}

	// Get original file info to preserve permissions
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	// Read current file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Apply edits (in reverse order to maintain offsets)
	newContent := applyTextEdits(string(content), edits)

	// Write back to file preserving original permissions
	if err := os.WriteFile(filePath, []byte(newContent), fileInfo.Mode()); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// applyWorkspaceEdit applies the workspace edit to the file system with transaction support
func applyWorkspaceEdit(edit *protocol.WorkspaceEdit) (*RenameResult, error) {
	if edit == nil {
		return nil, fmt.Errorf("no changes to apply")
	}

	// Handle both legacy Changes and modern DocumentChanges formats
	if len(edit.Changes) == 0 && len(edit.DocumentChanges) == 0 {
		return nil, fmt.Errorf("no changes to apply")
	}

	// Create transaction
	tx, err := NewRenameTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Pre-flight validation
	if err := tx.PreflightCheck(edit); err != nil {
		_ = tx.Cleanup() // Clean up backup directory
		return nil, fmt.Errorf("pre-flight validation failed: %w", err)
	}

	result := &RenameResult{}

	// Apply changes with rollback support
	applyErr := func() error {
		// Apply legacy Changes format
		for uriStr, textEdits := range edit.Changes {
			filePath := uriToPath(string(uriStr))

			// Check file modification time before applying
			if originalChecksum, exists := tx.checksums[filePath]; exists {
				if err := checkFileModificationTime(filePath, originalChecksum); err != nil {
					return err
				}
			}

			if err := tx.ApplyWithTracking(filePath, textEdits); err != nil {
				return fmt.Errorf("failed to apply changes to %s: %w", filePath, err)
			}
		}

		// Apply modern DocumentChanges format
		for _, textDocEdit := range edit.DocumentChanges {
			filePath := uriToPath(string(textDocEdit.TextDocument.URI))

			// Check file modification time before applying
			if originalChecksum, exists := tx.checksums[filePath]; exists {
				if err := checkFileModificationTime(filePath, originalChecksum); err != nil {
					return err
				}
			}

			if err := tx.ApplyWithTracking(filePath, textDocEdit.Edits); err != nil {
				return fmt.Errorf("failed to apply changes to %s: %w", filePath, err)
			}
		}

		return nil
	}()

	// Handle errors with rollback
	if applyErr != nil {
		// Attempt rollback
		reverted, rollbackErr := tx.Rollback()

		result.RolledBack = true
		result.FilesReverted = reverted
		result.BackupLocation = tx.KeepBackups() // Keep backups for debugging

		if rollbackErr != nil {
			result.RollbackSuccessful = false
			return result, errors.Join(fmt.Errorf("apply failed: %w", applyErr), fmt.Errorf("rollback had errors: %w", rollbackErr))
		}

		result.RollbackSuccessful = true
		return result, fmt.Errorf("changes rolled back due to error: %w", applyErr)
	}

	// Success - cleanup backups
	_ = tx.Cleanup() // Ignore cleanup errors - rename was successful

	result.Applied = true
	return result, nil
}

// applyTextEdits applies text edits to content
// Edits are applied in reverse order to maintain correct offsets
func applyTextEdits(content string, edits []protocol.TextEdit) string {
	// Detect line ending style from original content
	lineEnding := detectLineEnding(content)

	// Split into lines preserving original line ending style
	lines := strings.Split(content, lineEnding)

	// Handle edge case where content might use mixed line endings
	// In that case, normalise to LF for processing and restore after
	if len(lines) == 1 && strings.Contains(content, "\n") {
		lines = strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
		lineEnding = "\n"
	}

	// Apply edits in reverse order to maintain offsets
	for i := len(edits) - 1; i >= 0; i-- {
		lines = applyTextEdit(lines, edits[i])
	}

	return strings.Join(lines, lineEnding)
}

// detectLineEnding detects the line ending style used in content
func detectLineEnding(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	if strings.Contains(content, "\r") {
		return "\r"
	}
	return "\n"
}

// applyTextEdit applies a single text edit to lines
func applyTextEdit(lines []string, edit protocol.TextEdit) []string {
	startLine := int(edit.Range.Start.Line)
	startChar := int(edit.Range.Start.Character)
	endLine := int(edit.Range.End.Line)
	endChar := int(edit.Range.End.Character)

	if startLine < 0 || startLine >= len(lines) {
		return lines
	}

	if startLine == endLine {
		// Single line edit
		line := lines[startLine]
		if endChar > len(line) {
			endChar = len(line)
		}
		newLine := line[:startChar] + edit.NewText + line[endChar:]
		lines[startLine] = newLine
	} else {
		// Multi-line edit (rare)
		prefix := lines[startLine][:startChar]
		suffix := ""
		if endLine < len(lines) {
			suffix = lines[endLine][endChar:]
		}

		// Replace multiple lines with one
		newLine := prefix + edit.NewText + suffix
		result := make([]string, 0, len(lines)-(endLine-startLine))
		result = append(result, lines[:startLine]...)
		result = append(result, newLine)
		if endLine+1 < len(lines) {
			result = append(result, lines[endLine+1:]...)
		}
		lines = result
	}

	return lines
}
