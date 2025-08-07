package filesystem

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

const (
	// Filesystem security limits
	DefaultMaxFileSize             = int64(2 * 1024 * 1024 * 1024) // 2GB default file size limit
	DefaultSecureFilePermissions   = 0600                          // Read/write for owner only
	FilesystemMaxFileSizeEnvVar    = "FILESYSTEM_MAX_FILE_SIZE"
	FilesystemSecurePermissionsVar = "FILESYSTEM_SECURE_PERMISSIONS"
)

// FileSystemTool implements filesystem operations with directory access control
type FileSystemTool struct {
	allowedDirectories []string
	maxFileSize        int64
	secureFileMode     os.FileMode
	mu                 sync.RWMutex
}

// init registers the filesystem tool
func init() {
	tool := &FileSystemTool{
		allowedDirectories: getAllowedDirectories(),
	}
	tool.loadSecurityConfig()
	registry.Register(tool)
}

// getAllowedDirectories returns allowed directories from environment or defaults
func getAllowedDirectories() []string {
	// Check for custom allowed directories from environment variable
	if customDirs := os.Getenv("FILESYSTEM_TOOL_ALLOWED_DIRS"); customDirs != "" {
		// Split by colon (Unix-style) or semicolon (Windows-style)
		var dirs []string
		if strings.Contains(customDirs, ";") {
			// Windows-style path separator
			dirs = strings.Split(customDirs, ";")
		} else {
			// Unix-style path separator
			dirs = strings.Split(customDirs, ":")
		}

		// Clean and validate each directory
		var validDirs []string
		for _, dir := range dirs {
			dir = strings.TrimSpace(dir)
			if dir != "" {
				// Convert to absolute path
				if absDir, err := filepath.Abs(dir); err == nil {
					validDirs = append(validDirs, absDir)
				}
			}
		}

		if len(validDirs) > 0 {
			return validDirs
		}
	}

	// Fall back to default allowed directories
	return getDefaultAllowedDirectories()
}

// getDefaultAllowedDirectories returns default allowed directories
func getDefaultAllowedDirectories() []string {
	// Default to current working directory and user home directory
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()

	var dirs []string
	if cwd != "" {
		dirs = append(dirs, cwd)
	}
	if home != "" && home != cwd {
		dirs = append(dirs, home)
	}

	// If no directories found, allow current directory
	if len(dirs) == 0 {
		dirs = append(dirs, ".")
	}

	return dirs
}

// loadSecurityConfig loads security configuration from environment variables
func (t *FileSystemTool) loadSecurityConfig() {
	// Load max file size
	t.maxFileSize = DefaultMaxFileSize
	if sizeStr := os.Getenv(FilesystemMaxFileSizeEnvVar); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size > 0 {
			t.maxFileSize = size
		}
	}

	// Load secure file permissions
	t.secureFileMode = DefaultSecureFilePermissions
	if permStr := os.Getenv(FilesystemSecurePermissionsVar); permStr != "" {
		if perm, err := strconv.ParseUint(permStr, 8, 32); err == nil {
			t.secureFileMode = os.FileMode(perm)
		}
	}
}

// validateFileSize validates that the file size is within limits
func (t *FileSystemTool) validateFileSize(size int64) error {
	if size > t.maxFileSize {
		sizeMB := float64(size) / (1024 * 1024)
		maxSizeMB := float64(t.maxFileSize) / (1024 * 1024)
		return fmt.Errorf("file size %.1fMB exceeds maximum allowed size of %.1fMB (use %s environment variable to adjust limit)", sizeMB, maxSizeMB, FilesystemMaxFileSizeEnvVar)
	}
	return nil
}

// Definition returns the tool's definition for MCP registration
func (t *FileSystemTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"filesystem",
		mcp.WithDescription(`Advanced filesystem operations for managing files and directories. Do not use this tool unless explicitly requested by the user.

Functions and their required parameters:

• read_file: path (required), head (optional), tail (optional)
• read_multiple_files: paths (required)
• write_file: path (required), content (required)
• edit_file: path (required), edits (required), dryRun (optional)
• create_directory: path (required)
• list_directory: path (required), sortBy (optional)
• list_directory_with_sizes: path (required), sortBy (optional)
• directory_tree: path (required)
• move_file: source (required), destination (required)
• search_files: path (required), pattern (required), excludePatterns (optional)
• get_file_info: path (required)
• list_allowed_directories: (no parameters)
`),
		mcp.WithString("function",
			mcp.Required(),
			mcp.Description("Function to execute"),
			mcp.Enum("read_file", "read_multiple_files", "write_file", "edit_file",
				"create_directory", "list_directory", "list_directory_with_sizes",
				"directory_tree", "move_file", "search_files", "get_file_info",
				"list_allowed_directories"),
		),
		mcp.WithObject("options",
			mcp.Description("Function-specific options - see function description for parameters"),
			mcp.Properties(map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File or directory path",
				},
				"paths": map[string]interface{}{
					"type":        "array",
					"description": "Array of file paths",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "File content to write",
				},
				"head": map[string]interface{}{
					"type":        "number",
					"description": "Read only first N lines",
				},
				"tail": map[string]interface{}{
					"type":        "number",
					"description": "Read only last N lines",
				},
				"edits": map[string]interface{}{
					"type":        "array",
					"description": "Array of edit operations",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"oldText": map[string]interface{}{
								"type":        "string",
								"description": "Text to search for",
							},
							"newText": map[string]interface{}{
								"type":        "string",
								"description": "Text to replace with",
							},
						},
						"required": []string{"oldText", "newText"},
					},
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "Preview changes without applying",
					"default":     false,
				},
				"source": map[string]interface{}{
					"type":        "string",
					"description": "Source path for move operation",
				},
				"destination": map[string]interface{}{
					"type":        "string",
					"description": "Destination path for move operation",
				},
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Search pattern",
				},
				"excludePatterns": map[string]interface{}{
					"type":        "array",
					"description": "Patterns to exclude from search",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"sortBy": map[string]interface{}{
					"type":        "string",
					"description": "Sort directory listing by name or size",
					"enum":        []string{"name", "size"},
					"default":     "name",
				},
			}),
		),
	)
}

// Execute executes the filesystem tool
func (t *FileSystemTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Check if filesystem tool is enabled (disabled by defaulty)
	if !tools.IsToolEnabled("filesystem") {
		return nil, fmt.Errorf("filesystem tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'filesystem'")
	}

	// Parse function parameter
	function, ok := args["function"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: function")
	}

	// Parse options
	options := make(map[string]interface{})
	if optionsRaw, ok := args["options"]; ok {
		if optionsMap, ok := optionsRaw.(map[string]interface{}); ok {
			options = optionsMap
		}
	}

	// Execute the requested function
	switch function {
	case "read_file":
		return t.readFile(ctx, logger, options)
	case "read_multiple_files":
		return t.readMultipleFiles(ctx, logger, options)
	case "write_file":
		return t.writeFile(ctx, logger, options)
	case "edit_file":
		return t.editFile(ctx, logger, options)
	case "create_directory":
		return t.createDirectory(ctx, logger, options)
	case "list_directory":
		return t.listDirectory(ctx, logger, options)
	case "list_directory_with_sizes":
		return t.listDirectoryWithSizes(ctx, logger, options)
	case "directory_tree":
		return t.directoryTree(ctx, logger, options)
	case "move_file":
		return t.moveFile(ctx, logger, options)
	case "search_files":
		return t.searchFiles(ctx, logger, options)
	case "get_file_info":
		return t.getFileInfo(ctx, logger, options)
	case "list_allowed_directories":
		return t.listAllowedDirectories(ctx, logger, options)
	default:
		return nil, fmt.Errorf("unknown function: %s", function)
	}
}

// validatePath checks if a path is within allowed directories
func (t *FileSystemTool) validatePath(requestedPath string) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Expand home directory
	if strings.HasPrefix(requestedPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		requestedPath = filepath.Join(home, requestedPath[2:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(requestedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Clean the path
	cleanPath := filepath.Clean(absPath)

	// Check if path is within allowed directories
	for _, allowedDir := range t.allowedDirectories {
		allowedAbs, err := filepath.Abs(allowedDir)
		if err != nil {
			continue
		}
		allowedClean := filepath.Clean(allowedAbs)

		// Check if the path is within the allowed directory
		if cleanPath == allowedClean || strings.HasPrefix(cleanPath+string(filepath.Separator), allowedClean+string(filepath.Separator)) {
			// Handle symlinks by checking their real path
			realPath, err := filepath.EvalSymlinks(cleanPath)
			if err != nil {
				// For new files that don't exist yet, check parent directory
				if os.IsNotExist(err) {
					parentDir := filepath.Dir(cleanPath)
					if parentRealPath, parentErr := filepath.EvalSymlinks(parentDir); parentErr == nil {
						// Check if parent's real path is still within allowed directories
						if t.isPathWithinAllowedReal(parentRealPath, allowedClean) {
							return cleanPath, nil
						}
					}
					return cleanPath, nil // Allow creation of new files in valid directories
				}
				return "", fmt.Errorf("failed to resolve symlinks: %w", err)
			}

			// Check if the real path is still within allowed directories (considering symlinks in allowed dirs)
			if t.isPathWithinAllowedReal(realPath, allowedClean) {
				return realPath, nil
			}
			return "", fmt.Errorf("access denied - symlink target outside allowed directories: %s", realPath)
		}
	}

	return "", fmt.Errorf("access denied - path outside allowed directories: %s", cleanPath)
}

// isPathWithinAllowedReal checks if a real path is within the allowed directory, considering symlinks
func (t *FileSystemTool) isPathWithinAllowedReal(realPath, allowedClean string) bool {
	cleanRealPath := filepath.Clean(realPath)

	// Check direct match
	if cleanRealPath == allowedClean || strings.HasPrefix(cleanRealPath+string(filepath.Separator), allowedClean+string(filepath.Separator)) {
		return true
	}

	// Also resolve the allowed directory's symlinks to handle cases like /tmp -> /private/tmp on macOS
	allowedReal, err := filepath.EvalSymlinks(allowedClean)
	if err == nil {
		allowedRealClean := filepath.Clean(allowedReal)
		if cleanRealPath == allowedRealClean || strings.HasPrefix(cleanRealPath+string(filepath.Separator), allowedRealClean+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// readFile reads the contents of a file
func (t *FileSystemTool) readFile(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Check file size if file exists
	if fileInfo, err := os.Stat(validPath); err == nil {
		if err := t.validateFileSize(fileInfo.Size()); err != nil {
			return nil, fmt.Errorf("file size validation failed: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("file access error: %w", err)
	}

	// Check for head/tail options
	var head, tail *int
	if headRaw, ok := options["head"]; ok {
		if headFloat, ok := headRaw.(float64); ok {
			headInt := int(headFloat)
			head = &headInt
		}
	}
	if tailRaw, ok := options["tail"]; ok {
		if tailFloat, ok := tailRaw.(float64); ok {
			tailInt := int(tailFloat)
			tail = &tailInt
		}
	}

	if head != nil && tail != nil {
		return nil, fmt.Errorf("cannot specify both head and tail parameters")
	}

	var content string
	if head != nil {
		content, err = t.readFileHead(validPath, *head)
	} else if tail != nil {
		content, err = t.readFileTail(validPath, *tail)
	} else {
		contentBytes, readErr := os.ReadFile(validPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read file: %w", readErr)
		}
		content = string(contentBytes)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return mcp.NewToolResultText(content), nil
}

// readFileHead reads the first N lines of a file
func (t *FileSystemTool) readFileHead(path string, numLines int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't override the main error
			_ = closeErr // Acknowledge the error to satisfy linter
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	for i := 0; i < numLines && scanner.Scan(); i++ {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// readFileTail reads the last N lines of a file
func (t *FileSystemTool) readFileTail(path string, numLines int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't override the main error
			_ = closeErr // Acknowledge the error to satisfy linter
		}
	}()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	fileSize := stat.Size()

	if fileSize == 0 {
		return "", nil
	}

	// Read from end of file in chunks
	const chunkSize = 1024
	var lines []string
	var buffer []byte
	position := fileSize

	for len(lines) < numLines && position > 0 {
		// Calculate chunk size to read
		readSize := int64(chunkSize)
		if position < readSize {
			readSize = position
		}
		position -= readSize

		// Read chunk
		chunk := make([]byte, readSize)
		_, err := file.ReadAt(chunk, position)
		if err != nil && err != io.EOF {
			return "", err
		}

		// Prepend to buffer
		buffer = append(chunk, buffer...)

		// Split into lines
		text := string(buffer)
		allLines := strings.Split(text, "\n")

		// If we're not at the beginning of the file, the first line might be incomplete
		if position > 0 && len(allLines) > 1 {
			// Keep the first (incomplete) line in buffer for next iteration
			buffer = []byte(allLines[0])
			allLines = allLines[1:]
		} else {
			buffer = nil
		}

		// Add lines to result (in reverse order since we're reading backwards)
		for i := len(allLines) - 1; i >= 0 && len(lines) < numLines; i-- {
			if allLines[i] != "" || i == len(allLines)-1 { // Keep empty lines except trailing ones
				lines = append([]string{allLines[i]}, lines...)
			}
		}
	}

	// Limit to requested number of lines
	if len(lines) > numLines {
		lines = lines[len(lines)-numLines:]
	}

	return strings.Join(lines, "\n"), nil
}

// readMultipleFiles reads multiple files simultaneously
func (t *FileSystemTool) readMultipleFiles(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	pathsRaw, ok := options["paths"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: paths")
	}

	pathsInterface, ok := pathsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("paths must be an array")
	}

	var paths []string
	for _, pathInterface := range pathsInterface {
		if pathStr, ok := pathInterface.(string); ok {
			paths = append(paths, pathStr)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no valid paths provided")
	}

	var results []string
	for _, path := range paths {
		validPath, err := t.validatePath(path)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: Error - %s", path, err.Error()))
			continue
		}

		// Check file size before reading
		if fileInfo, err := os.Stat(validPath); err == nil {
			if err := t.validateFileSize(fileInfo.Size()); err != nil {
				results = append(results, fmt.Sprintf("%s: Error - file size validation failed: %s", path, err.Error()))
				continue
			}
		} else if !os.IsNotExist(err) {
			results = append(results, fmt.Sprintf("%s: Error - file access error: %s", path, err.Error()))
			continue
		}

		content, err := os.ReadFile(validPath)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: Error - %s", path, err.Error()))
			continue
		}

		results = append(results, fmt.Sprintf("%s:\n%s", path, string(content)))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n---\n")), nil
}

// writeFile creates or overwrites a file
func (t *FileSystemTool) writeFile(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	content, ok := options["content"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: content")
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Validate content size before writing
	contentSize := int64(len(content))
	if err := t.validateFileSize(contentSize); err != nil {
		return nil, fmt.Errorf("content size validation failed: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(validPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Use atomic write with temporary file
	tempPath := validPath + ".tmp." + t.generateRandomString(8)

	if err := os.WriteFile(tempPath, []byte(content), t.secureFileMode); err != nil {
		return nil, fmt.Errorf("failed to write temporary file: %w", err)
	}

	if err := os.Rename(tempPath, validPath); err != nil {
		_ = os.Remove(tempPath) // Clean up temp file, ignore error
		return nil, fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully wrote to %s", path)), nil
}

// generateRandomString generates a random string for temporary files
func (t *FileSystemTool) generateRandomString(length int) string {
	bytes := make([]byte, length/2)
	_, _ = rand.Read(bytes) // Ignore error as rand.Read from crypto/rand never fails
	return hex.EncodeToString(bytes)
}

// editFile performs line-based edits on a file
func (t *FileSystemTool) editFile(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	editsRaw, ok := options["edits"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: edits")
	}

	dryRun := false
	if dryRunRaw, ok := options["dryRun"]; ok {
		if dryRunBool, ok := dryRunRaw.(bool); ok {
			dryRun = dryRunBool
		}
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Check existing file size if file exists
	if fileInfo, err := os.Stat(validPath); err == nil {
		if err := t.validateFileSize(fileInfo.Size()); err != nil {
			return nil, fmt.Errorf("existing file size validation failed: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("file access error: %w", err)
	}

	// Parse edits
	var edits []EditOperation
	if editsArray, ok := editsRaw.([]interface{}); ok {
		for _, editRaw := range editsArray {
			if editMap, ok := editRaw.(map[string]interface{}); ok {
				oldText, oldOk := editMap["oldText"].(string)
				newText, newOk := editMap["newText"].(string)
				if oldOk && newOk {
					edits = append(edits, EditOperation{
						OldText: oldText,
						NewText: newText,
					})
				}
			}
		}
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("no valid edits provided")
	}

	// Read file content
	content, err := os.ReadFile(validPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	originalContent := string(content)
	modifiedContent := originalContent

	// Apply edits
	for _, edit := range edits {
		if !strings.Contains(modifiedContent, edit.OldText) {
			return nil, fmt.Errorf("could not find text to replace: %s", edit.OldText)
		}
		modifiedContent = strings.Replace(modifiedContent, edit.OldText, edit.NewText, 1)
	}

	// Validate modified content size
	modifiedSize := int64(len(modifiedContent))
	if err := t.validateFileSize(modifiedSize); err != nil {
		return nil, fmt.Errorf("modified content size validation failed: %w", err)
	}

	// Create diff
	diff := t.createDiff(originalContent, modifiedContent, path)

	if !dryRun {
		// Write the modified content atomically
		tempPath := validPath + ".tmp." + t.generateRandomString(8)

		if err := os.WriteFile(tempPath, []byte(modifiedContent), t.secureFileMode); err != nil {
			return nil, fmt.Errorf("failed to write temporary file: %w", err)
		}

		if err := os.Rename(tempPath, validPath); err != nil {
			_ = os.Remove(tempPath) // Clean up temp file, ignore error
			return nil, fmt.Errorf("failed to rename temporary file: %w", err)
		}
	}

	return mcp.NewToolResultText(diff), nil
}

// createDiff creates a simple diff between original and modified content
func (t *FileSystemTool) createDiff(original, modified, filename string) string {
	if original == modified {
		return "No changes made."
	}

	originalLines := strings.Split(original, "\n")
	modifiedLines := strings.Split(modified, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s (original)\n", filename))
	diff.WriteString(fmt.Sprintf("+++ %s (modified)\n", filename))

	// Simple line-by-line diff
	maxLines := len(originalLines)
	if len(modifiedLines) > maxLines {
		maxLines = len(modifiedLines)
	}

	for i := 0; i < maxLines; i++ {
		var origLine, modLine string
		if i < len(originalLines) {
			origLine = originalLines[i]
		}
		if i < len(modifiedLines) {
			modLine = modifiedLines[i]
		}

		if origLine != modLine {
			if origLine != "" {
				diff.WriteString(fmt.Sprintf("-%s\n", origLine))
			}
			if modLine != "" {
				diff.WriteString(fmt.Sprintf("+%s\n", modLine))
			}
		}
	}

	return diff.String()
}

// createDirectory creates a directory
func (t *FileSystemTool) createDirectory(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(validPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully created directory %s", path)), nil
}

// listDirectory lists directory contents
func (t *FileSystemTool) listDirectory(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(validPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var result strings.Builder
	for _, entry := range entries {
		prefix := "[FILE]"
		if entry.IsDir() {
			prefix = "[DIR]"
		}
		result.WriteString(fmt.Sprintf("%s %s\n", prefix, entry.Name()))
	}

	return mcp.NewToolResultText(strings.TrimSuffix(result.String(), "\n")), nil
}

// listDirectoryWithSizes lists directory contents with sizes
func (t *FileSystemTool) listDirectoryWithSizes(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	sortBy := "name"
	if sortByRaw, ok := options["sortBy"]; ok {
		if sortByStr, ok := sortByRaw.(string); ok {
			sortBy = sortByStr
		}
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(validPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Get detailed information for each entry
	type entryInfo struct {
		name  string
		isDir bool
		size  int64
	}

	var detailedEntries []entryInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		detailedEntries = append(detailedEntries, entryInfo{
			name:  entry.Name(),
			isDir: entry.IsDir(),
			size:  info.Size(),
		})
	}

	// Sort entries
	if sortBy == "size" {
		sort.Slice(detailedEntries, func(i, j int) bool {
			return detailedEntries[i].size > detailedEntries[j].size
		})
	} else {
		sort.Slice(detailedEntries, func(i, j int) bool {
			return detailedEntries[i].name < detailedEntries[j].name
		})
	}

	// Format output
	var result strings.Builder
	var totalFiles, totalDirs int
	var totalSize int64

	for _, entry := range detailedEntries {
		prefix := "[FILE]"
		sizeStr := ""
		if entry.isDir {
			prefix = "[DIR]"
			totalDirs++
		} else {
			totalFiles++
			totalSize += entry.size
			sizeStr = fmt.Sprintf("%10s", t.formatSize(entry.size))
		}
		result.WriteString(fmt.Sprintf("%s %-30s %s\n", prefix, entry.name, sizeStr))
	}

	// Add summary
	result.WriteString(fmt.Sprintf("\nTotal: %d files, %d directories\n", totalFiles, totalDirs))
	result.WriteString(fmt.Sprintf("Combined size: %s\n", t.formatSize(totalSize)))

	return mcp.NewToolResultText(strings.TrimSuffix(result.String(), "\n")), nil
}

// formatSize formats file size in human-readable format
func (t *FileSystemTool) formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// directoryTree creates a recursive tree view of directories
func (t *FileSystemTool) directoryTree(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	tree, err := t.buildDirectoryTree(validPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build directory tree: %w", err)
	}

	// Convert to JSON-like string representation
	result := t.formatDirectoryTree(tree, 0)
	return mcp.NewToolResultText(result), nil
}

// buildDirectoryTree recursively builds a directory tree
func (t *FileSystemTool) buildDirectoryTree(path string) ([]DirectoryEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []DirectoryEntry
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())

		// Validate each path
		if _, err := t.validatePath(entryPath); err != nil {
			continue // Skip invalid paths
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		dirEntry := DirectoryEntry{
			Name:     entry.Name(),
			Type:     "file",
			Size:     info.Size(),
			Modified: info.ModTime(),
		}

		if entry.IsDir() {
			dirEntry.Type = "directory"
			dirEntry.Size = 0
			children, err := t.buildDirectoryTree(entryPath)
			if err == nil {
				dirEntry.Children = children
			} else {
				dirEntry.Children = []DirectoryEntry{} // Empty array for directories we can't read
			}
		}

		result = append(result, dirEntry)
	}

	return result, nil
}

// formatDirectoryTree formats the directory tree for display
func (t *FileSystemTool) formatDirectoryTree(entries []DirectoryEntry, indent int) string {
	var result strings.Builder
	indentStr := strings.Repeat("  ", indent)

	for i, entry := range entries {
		if i > 0 {
			result.WriteString(",\n")
		}
		result.WriteString(fmt.Sprintf("%s{\n", indentStr))
		result.WriteString(fmt.Sprintf("%s  \"name\": \"%s\",\n", indentStr, entry.Name))
		result.WriteString(fmt.Sprintf("%s  \"type\": \"%s\"", indentStr, entry.Type))

		if entry.Type == "file" {
			result.WriteString(fmt.Sprintf(",\n%s  \"size\": %d", indentStr, entry.Size))
		}

		if entry.Type == "directory" {
			result.WriteString(fmt.Sprintf(",\n%s  \"children\": [", indentStr))
			if len(entry.Children) > 0 {
				result.WriteString("\n")
				result.WriteString(t.formatDirectoryTree(entry.Children, indent+2))
				result.WriteString(fmt.Sprintf("\n%s  ]", indentStr))
			} else {
				result.WriteString("]")
			}
		}

		result.WriteString(fmt.Sprintf("\n%s}", indentStr))
	}

	return result.String()
}

// moveFile moves or renames files and directories
func (t *FileSystemTool) moveFile(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	source, ok := options["source"].(string)
	if !ok || source == "" {
		return nil, fmt.Errorf("missing required parameter: source")
	}

	destination, ok := options["destination"].(string)
	if !ok || destination == "" {
		return nil, fmt.Errorf("missing required parameter: destination")
	}

	validSource, err := t.validatePath(source)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}

	validDestination, err := t.validatePath(destination)
	if err != nil {
		return nil, fmt.Errorf("invalid destination path: %w", err)
	}

	// Check if destination already exists
	if _, err := os.Stat(validDestination); err == nil {
		return nil, fmt.Errorf("destination already exists: %s", destination)
	}

	if err := os.Rename(validSource, validDestination); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully moved %s to %s", source, destination)), nil
}

// searchFiles recursively searches for files matching a pattern
func (t *FileSystemTool) searchFiles(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	pattern, ok := options["pattern"].(string)
	if !ok || pattern == "" {
		return nil, fmt.Errorf("missing required parameter: pattern")
	}

	var excludePatterns []string
	if excludePatternsRaw, ok := options["excludePatterns"]; ok {
		if excludePatternsArray, ok := excludePatternsRaw.([]interface{}); ok {
			for _, patternRaw := range excludePatternsArray {
				if patternStr, ok := patternRaw.(string); ok {
					excludePatterns = append(excludePatterns, patternStr)
				}
			}
		}
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	results, err := t.performSearch(validPath, pattern, excludePatterns)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No matches found"), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// performSearch performs the actual file search
func (t *FileSystemTool) performSearch(rootPath, pattern string, excludePatterns []string) ([]string, error) {
	var results []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}

		// Validate path is still within allowed directories
		if _, validateErr := t.validatePath(path); validateErr != nil {
			return nil // Skip invalid paths
		}

		// Check exclude patterns
		relativePath, _ := filepath.Rel(rootPath, path)
		for _, excludePattern := range excludePatterns {
			if matched, _ := filepath.Match(excludePattern, filepath.Base(path)); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Also check against relative path for directory patterns
			if matched, _ := filepath.Match(excludePattern, relativePath); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check if name matches pattern (case-insensitive)
		name := strings.ToLower(info.Name())
		searchPattern := strings.ToLower(pattern)
		if strings.Contains(name, searchPattern) {
			results = append(results, path)
		}

		return nil
	})

	return results, err
}

// getFileInfo retrieves detailed file information
func (t *FileSystemTool) getFileInfo(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	path, ok := options["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	validPath, err := t.validatePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(validPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileInfo := FileInfo{
		Size:        info.Size(),
		Modified:    info.ModTime(),
		IsDirectory: info.IsDir(),
		IsFile:      !info.IsDir(),
		Permissions: fmt.Sprintf("%o", info.Mode().Perm()),
	}

	// Try to get creation and access times (platform-specific)
	// For cross-platform compatibility, we'll use modification time as fallback
	fileInfo.Created = info.ModTime()
	fileInfo.Accessed = info.ModTime()

	// For cross-platform compatibility, we use modification time for all timestamps
	// Different platforms have different field names in syscall.Stat_t
	// To avoid compilation issues, we'll keep it simple and use ModTime
	// This ensures the tool works consistently across all platforms
	fileInfo.Created = info.ModTime()
	fileInfo.Accessed = info.ModTime()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Path: %s\n", path))
	result.WriteString(fmt.Sprintf("Size: %s (%d bytes)\n", t.formatSize(fileInfo.Size), fileInfo.Size))
	result.WriteString(fmt.Sprintf("Type: %s\n", map[bool]string{true: "Directory", false: "File"}[fileInfo.IsDirectory]))
	result.WriteString(fmt.Sprintf("Permissions: %s\n", fileInfo.Permissions))
	result.WriteString(fmt.Sprintf("Modified: %s\n", fileInfo.Modified.Format(time.RFC3339)))
	result.WriteString(fmt.Sprintf("Created: %s\n", fileInfo.Created.Format(time.RFC3339)))
	result.WriteString(fmt.Sprintf("Accessed: %s", fileInfo.Accessed.Format(time.RFC3339)))

	return mcp.NewToolResultText(result.String()), nil
}

// listAllowedDirectories returns the list of allowed directories
func (t *FileSystemTool) listAllowedDirectories(ctx context.Context, logger *logrus.Logger, options map[string]interface{}) (*mcp.CallToolResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result strings.Builder
	result.WriteString("Allowed directories:\n")
	for _, dir := range t.allowedDirectories {
		result.WriteString(fmt.Sprintf("  %s\n", dir))
	}

	return mcp.NewToolResultText(strings.TrimSuffix(result.String(), "\n")), nil
}

// SetAllowedDirectories sets the allowed directories (for testing purposes)
func (t *FileSystemTool) SetAllowedDirectories(dirs []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.allowedDirectories = dirs
}

// LoadSecurityConfig loads security configuration (for testing purposes)
func (t *FileSystemTool) LoadSecurityConfig() {
	t.loadSecurityConfig()
}

// ProvideExtendedInfo provides detailed usage information for the filesystem tool
func (t *FileSystemTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Read a file with line limits",
				Arguments: map[string]interface{}{
					"function": "read_file",
					"options": map[string]interface{}{
						"path": "/Users/username/projects/myapp/config.json",
						"head": 20,
					},
				},
				ExpectedResult: "Returns first 20 lines of the config file. Use 'tail' instead of 'head' to get last N lines",
			},
			{
				Description: "Edit a file with multiple replacements",
				Arguments: map[string]interface{}{
					"function": "edit_file",
					"options": map[string]interface{}{
						"path": "/Users/username/projects/myapp/src/config.js",
						"edits": []map[string]interface{}{
							{"oldText": "const API_URL = 'http://localhost:3000'", "newText": "const API_URL = 'https://api.production.com'"},
							{"oldText": "debug: true", "newText": "debug: false"},
						},
						"dryRun": true,
					},
				},
				ExpectedResult: "Shows diff preview without applying changes. Remove dryRun or set to false to apply changes",
			},
			{
				Description: "Search for files with exclusions",
				Arguments: map[string]interface{}{
					"function": "search_files",
					"options": map[string]interface{}{
						"path":            "/Users/username/projects/webapp",
						"pattern":         "component",
						"excludePatterns": []string{"node_modules", "*.test.js", "dist"},
					},
				},
				ExpectedResult: "Finds files containing 'component' while excluding test files, dependencies, and build directories",
			},
			{
				Description: "List directory with size information sorted by size",
				Arguments: map[string]interface{}{
					"function": "list_directory_with_sizes",
					"options": map[string]interface{}{
						"path":   "/Users/username/projects/myapp/src",
						"sortBy": "size",
					},
				},
				ExpectedResult: "Lists directory contents with file sizes, sorted from largest to smallest files",
			},
			{
				Description: "Create directory structure and write file",
				Arguments: map[string]interface{}{
					"function": "write_file",
					"options": map[string]interface{}{
						"path":    "/Users/username/projects/newapp/src/config/database.js",
						"content": "module.exports = {\n  host: 'localhost',\n  port: 5432,\n  database: 'myapp'\n};",
					},
				},
				ExpectedResult: "Creates the directory structure if it doesn't exist, then writes the file with secure permissions",
			},
		},
		CommonPatterns: []string{
			"Use 'list_allowed_directories' first to see which directories you can access",
			"Use 'dryRun: true' in edit_file operations to preview changes before applying",
			"Use head/tail parameters in read_file for large files to avoid reading entire contents",
			"Use 'get_file_info' to check file permissions and timestamps before operations",
			"Combine 'search_files' with exclude patterns to filter out irrelevant results",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Access denied - path outside allowed directories",
				Solution: "The filesystem tool has security restrictions. Use 'list_allowed_directories' to see valid paths, or ask the user to set FILESYSTEM_TOOL_ALLOWED_DIRS environment variable to configure allowed directories.",
			},
			{
				Problem:  "File size exceeds maximum allowed size error",
				Solution: "Large files are rejected for security. Use FILESYSTEM_MAX_FILE_SIZE environment variable to increase limits, or use head/tail parameters for reading large files partially.",
			},
			{
				Problem:  "Could not find text to replace in edit_file",
				Solution: "The oldText in edit operations must match exactly, including whitespace. Use read_file first to see the exact content, or use dryRun to test edits.",
			},
			{
				Problem:  "Permission denied errors",
				Solution: "Ensure the process has read/write permissions to the target files and directories. Check file permissions with get_file_info function.",
			},
		},
		ParameterDetails: map[string]string{
			"function": "Required parameter specifying which filesystem operation to perform. Each function has different required parameters in the options object.",
			"options":  "Function-specific parameters object. Structure varies by function - see examples for each function's required parameters.",
		},
		WhenToUse:    "Use when explicitly requested by the user to use the 'filesystem tool' (or 'filesystem mcp tool') file operations like reading, writing, editing, or managing files and directories. Do not use this tool unless specifically asked by the user.",
		WhenNotToUse: "Don't use proactively or automatically. Do not use when you have other methods of interacting with files that are more efficient. Only use when the user explicitly requests file system operations. Not suitable for users who haven't enabled the tool or configured allowed directories.",
	}
}
