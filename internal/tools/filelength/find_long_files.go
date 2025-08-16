package filelength

import (
	"bufio"
	"bytes"
	"context"
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
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

const (
	defaultMaxFileSizeKB = 2048
	defaultLineThreshold = 700
	bufferSize           = 32 * 1024 // 32KB buffer for optimal line counting performance
)

// FindLongFilesTool implements finding files with excessive line counts
type FindLongFilesTool struct{}

// init registers the find-long-files tool
func init() {
	registry.Register(&FindLongFilesTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *FindLongFilesTool) Definition() mcp.Tool {
	// Check if return prompt is disabled
	returnPrompt, envExists := os.LookupEnv("LONG_FILES_RETURN_PROMPT")
	isPromptDisabled := envExists && strings.TrimSpace(returnPrompt) == ""

	// Build description based on whether prompt is enabled
	baseDescription := `Efficiently find files that have a high line count, highlighting that they may need refactoring.

- Ignores binaries
- Respects .gitignore
- Returns a formatted checklist of files exceeding the threshold`

	var description string
	if isPromptDisabled {
		description = baseDescription + "."
	} else {
		description = baseDescription + " and suggestions for the next step in handling them."
	}

	return mcp.NewTool(
		"find_long_files",
		mcp.WithDescription(description),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute directory path to search for long files (e.g. '/Users/username/git/project')"),
		),
		mcp.WithNumber("line_threshold",
			mcp.Description("Minimum number of lines to consider 'long' (default: 700)"),
			mcp.DefaultNumber(700),
		),
		mcp.WithArray("additional_excludes",
			mcp.Description("Additional glob patterns to exclude"),
		),
	)
}

// Execute executes the find-long-files tool
func (t *FindLongFilesTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	startTime := time.Now()
	logger.Info("Executing find-long-files tool")

	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"path":                     request.Path,
		"line_threshold":           request.LineThreshold,
		"additional_excludes":      request.AdditionalExcludes,
		"sort_by_directory_totals": request.SortByDirectoryTotals,
	}).Debug("Find long files parameters")

	// Find long files
	longFiles, totalScanned, skippedLargeFiles, err := t.findLongFiles(ctx, logger, request)
	if err != nil {
		return nil, fmt.Errorf("failed to find long files: %w", err)
	}

	// Generate checklist
	checklist := t.generateChecklist(longFiles, request, startTime)

	// Get default message from environment or use default
	defaultMessage, envExists := os.LookupEnv("LONG_FILES_RETURN_PROMPT")
	if !envExists {
		// Environment variable not set, use default
		defaultMessage = `**Suggested Next Steps (Unless the user has instructed you otherwise)**:
1. Take this checklist and save it to a temporary location.
2. Perform a _quick_ review of the identified files and under each checklist item adds a concise (1-2 sentences) summary of what should be done to reduce the file length, considering what sort of pattern or logic should be used to decide how the code or content should be split (or if not - why), ensuring your strategy values concise, clean, efficient code and operations.
3. Then stop and ask the user to review.`
	} else {
		// Environment variable is set - if it's empty/whitespace, user wants no message
		defaultMessage = strings.TrimSpace(defaultMessage)
	}

	response := &FindLongFilesResponse{
		Checklist:         checklist,
		LastChecked:       startTime,
		CalculationTime:   time.Since(startTime).String(),
		TotalFilesScanned: totalScanned,
		TotalFilesFound:   len(longFiles),
		SkippedLargeFiles: skippedLargeFiles,
		Message:           defaultMessage,
	}

	logger.WithFields(logrus.Fields{
		"total_files_scanned": totalScanned,
		"total_files_found":   len(longFiles),
		"calculation_time":    response.CalculationTime,
	}).Info("Find long files completed successfully")

	// Build output with optional skipped files section and message
	output := response.Checklist

	// Add skipped large files section if any
	if len(response.SkippedLargeFiles) > 0 {
		// Get the max size for display purposes
		maxFileSizeKB := t.getMaxFileSizeKB()

		// Format size display
		var sizeDisplay string
		if maxFileSizeKB >= 1024 {
			sizeDisplay = fmt.Sprintf("%.1fMB", float64(maxFileSizeKB)/1024)
		} else {
			sizeDisplay = fmt.Sprintf("%dKB", maxFileSizeKB)
		}

		output += fmt.Sprintf("\n## Skipped Files (>%s)\n\nThe following files were skipped due to being larger than %s:\n\n", sizeDisplay, sizeDisplay)
		for _, file := range response.SkippedLargeFiles {
			output += fmt.Sprintf("- `%s`\n", file)
		}
	}

	// Add message if not empty
	if defaultMessage != "" {
		output += "\n\n" + response.Message
	}

	return t.newToolResultText(output)
}

// parseRequest parses and validates the tool arguments
func (t *FindLongFilesTool) parseRequest(args map[string]interface{}) (*FindLongFilesRequest, error) {
	request := &FindLongFilesRequest{
		LineThreshold:         defaultLineThreshold,
		AdditionalExcludes:    []string{},
		SortByDirectoryTotals: false,
	}

	// Parse path (required)
	pathRaw, ok := args["path"].(string)
	if !ok || pathRaw == "" {
		return nil, fmt.Errorf("missing required parameter: path")
	}

	// Validate that path is absolute
	if !filepath.IsAbs(pathRaw) {
		return nil, fmt.Errorf("path must be absolute (e.g., '/Users/username/project'), got: %s", pathRaw)
	}

	request.Path = pathRaw

	// Parse line threshold - check environment variable first
	if envThreshold := os.Getenv("LONG_FILES_DEFAULT_LENGTH"); envThreshold != "" {
		if threshold, err := strconv.Atoi(envThreshold); err == nil && threshold > 0 {
			request.LineThreshold = threshold
		}
	}
	if thresholdRaw, ok := args["line_threshold"].(float64); ok {
		threshold := int(thresholdRaw)
		if threshold < 1 {
			return nil, fmt.Errorf("line_threshold must be at least 1")
		}
		request.LineThreshold = threshold
	}

	// Parse additional excludes - check environment variable first
	if envExcludes := os.Getenv("LONG_FILES_ADDITIONAL_EXCLUDES"); envExcludes != "" {
		request.AdditionalExcludes = strings.Split(envExcludes, ",")
		for i, exclude := range request.AdditionalExcludes {
			request.AdditionalExcludes[i] = strings.TrimSpace(exclude)
		}
	}
	if excludesRaw, ok := args["additional_excludes"].([]interface{}); ok {
		excludes := make([]string, len(excludesRaw))
		for i, exclude := range excludesRaw {
			if excludeStr, ok := exclude.(string); ok {
				excludes[i] = excludeStr
			}
		}
		request.AdditionalExcludes = excludes
	}

	// Parse sort option from environment variable
	if envSort := os.Getenv("LONG_FILES_SORT_BY_DIRECTORY_TOTALS"); envSort != "" {
		if envSort == "true" || envSort == "1" {
			request.SortByDirectoryTotals = true
		}
	}

	// Validate path exists
	if _, err := os.Stat(request.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", request.Path)
	}

	// Security check for file access
	if err := security.CheckFileAccess(request.Path); err != nil {
		return nil, err
	}

	return request, nil
}

// findLongFiles finds all files exceeding the line threshold
func (t *FindLongFilesTool) findLongFiles(ctx context.Context, logger *logrus.Logger, request *FindLongFilesRequest) ([]FileInfo, int, []string, error) {
	var longFiles []FileInfo
	var skippedLargeFiles []string
	totalScanned := 0

	// Get maximum file size from environment variable or use default
	maxFileSizeKB := t.getMaxFileSizeKB()
	maxFileSize := int64(maxFileSizeKB * 1024) // Convert KB to bytes

	// Load gitignore patterns
	gitignorePatterns, err := t.loadGitignorePatterns(request.Path)
	if err != nil {
		logger.WithError(err).Warn("Failed to load .gitignore patterns, continuing without them")
	}

	// Walk the directory tree
	err = filepath.Walk(request.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Handle permission errors gracefully - skip inaccessible files/directories
			if os.IsPermission(err) {
				logger.WithError(err).WithField("path", path).Debug("Skipping path due to permission error")
				if info != nil && info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Check if we have read permission on the directory
			if !t.hasReadPermission(path) {
				logger.WithField("dir", path).Debug("Skipping directory due to lack of read permission")
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be excluded
		if t.shouldExcludeFile(path, gitignorePatterns, request.AdditionalExcludes) {
			return nil
		}

		// Check file size - skip files larger than 2MB
		if info.Size() > maxFileSize {
			relPath, _ := filepath.Rel(request.Path, path)
			if !strings.HasPrefix(relPath, "./") && !strings.HasPrefix(relPath, "../") {
				relPath = "./" + relPath
			}
			skippedLargeFiles = append(skippedLargeFiles, relPath)
			logger.WithFields(logrus.Fields{
				"file":        relPath,
				"size":        info.Size(),
				"max_size_kb": maxFileSizeKB,
			}).Debug("Skipping file due to size limit")
			return nil
		}

		// Check if we have read permission on the file
		if !t.hasReadPermission(path) {
			logger.WithField("file", path).Debug("Skipping file due to lack of read permission")
			return nil
		}

		// Skip binary files
		if t.isBinaryFile(path) {
			return nil
		}

		totalScanned++

		// Count lines in file
		lineCount, err := t.countLines(path)
		if err != nil {
			logger.WithError(err).WithField("file", path).Warn("Failed to count lines in file")
			return nil
		}

		// Check if file exceeds threshold
		if lineCount >= request.LineThreshold {
			relPath, _ := filepath.Rel(request.Path, path)
			if !strings.HasPrefix(relPath, "./") && !strings.HasPrefix(relPath, "../") {
				relPath = "./" + relPath
			}

			longFiles = append(longFiles, FileInfo{
				Path:      relPath,
				LineCount: lineCount,
				Directory: filepath.Dir(relPath),
				SizeBytes: info.Size(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, totalScanned, skippedLargeFiles, err
	}

	// Sort files
	if request.SortByDirectoryTotals {
		longFiles = t.sortByDirectoryTotals(longFiles)
	} else {
		// Sort by line count descending
		sort.Slice(longFiles, func(i, j int) bool {
			return longFiles[i].LineCount > longFiles[j].LineCount
		})
	}

	return longFiles, totalScanned, skippedLargeFiles, nil
}

// loadGitignorePatterns loads gitignore patterns from .gitignore file
func (t *FindLongFilesTool) loadGitignorePatterns(basePath string) ([]string, error) {
	gitignorePath := filepath.Join(basePath, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns, scanner.Err()
}

// getDefaultExcludePatterns returns comprehensive default exclusion patterns
func (t *FindLongFilesTool) getDefaultExcludePatterns() []string {
	return []string{
		// Binary document formats
		"**/*.docx", "**/*.doc", "**/*.rtf", "**/*.odt",
		"**/*.pptx", "**/*.ppt", "**/*.odp",
		"**/*.xlsx", "**/*.xls", "**/*.ods",
		"**/*.pdf",

		// Image formats
		"**/*.png", "**/*.jpg", "**/*.jpeg", "**/*.gif", "**/*.bmp",
		"**/*.tiff", "**/*.tif", "**/*.webp", "**/*.ico", "**/*.svg",

		// Audio/Video formats
		"**/*.mp3", "**/*.mp4", "**/*.avi", "**/*.mov", "**/*.wmv",
		"**/*.flv", "**/*.webm", "**/*.ogg", "**/*.wav", "**/*.flac",

		// Archive formats
		"**/*.zip", "**/*.tar", "**/*.gz", "**/*.bz2", "**/*.xz",
		"**/*.7z", "**/*.rar", "**/*.dmg", "**/*.iso",

		// Binary/executable formats
		"**/*.bin", "**/*.exe", "**/*.dll", "**/*.so", "**/*.dylib",
		"**/*.class", "**/*.jar", "**/*.war", "**/*.ear",
		"**/*.deb", "**/*.rpm", "**/*.pkg", "**/*.msi",

		// Certificate and key files
		"**/*.pem", "**/*.crt", "**/*.cer", "**/*.p12", "**/*.pfx",
		"**/*.key", "**/*.pub", "**/*.gpg",

		// Temporary and log files
		"**/*.tmp", "**/*.temp", "**/*.bak", "**/*.swp", "**/*.swo",
		"**/*.log", "**/*.log.*", "**/*.out", "**/*.out.*",
		"**/*.old", "**/*.orig", "**/*.rej",

		// Common directories to exclude
		"**/node_modules/**", "**/.git/**", "**/.svn/**", "**/.hg/**",
		"**/.bzr/**", "**/vendor/**", "**/.venv/**", "**/venv/**",
		"**/__pycache__/**", "**/.pytest_cache/**", "**/coverage/**",
		"**/build/**", "**/dist/**", "**/target/**", "**/bin/**",
		"**/.npm/**", "**/.yarn/**", "**/.cache/**", "**/cache/**",
		"**/.DS_Store", "**/Thumbs.db",

		// Common license and documentation files (exact matches)
		"LICENSE", "LICENCE", "LICENSE.*", "LICENCE.*",
		"CONTRIBUTING", "CONTRIBUTING.*", "CHANGELOG", "CHANGELOG.*",
		"AUTHORS", "AUTHORS.*", "CREDITS", "CREDITS.*",
		"COPYING", "COPYING.*", "NOTICE", "NOTICE.*",

		// Font files
		"**/*.ttf", "**/*.otf", "**/*.woff", "**/*.woff2", "**/*.eot",

		// Database files
		"**/*.db", "**/*.sqlite", "**/*.sqlite3", "**/*.mdb",

		// IDE and editor files
		"**/.vscode/**", "**/.idea/**", "**/*.sublime-*",
		"**/.settings/**", "**/.project", "**/.classpath",
	}
}

// shouldExcludeFile checks if a file should be excluded based on patterns
func (t *FindLongFilesTool) shouldExcludeFile(path string, gitignorePatterns, additionalExcludes []string) bool {
	// Get relative path for pattern matching
	fileName := filepath.Base(path)

	// First check default exclusions (most efficient, checks binary files first)
	defaultExcludes := t.getDefaultExcludePatterns()
	for _, pattern := range defaultExcludes {
		if t.matchesPattern(path, fileName, pattern) {
			return true
		}
	}

	// Check gitignore patterns
	for _, pattern := range gitignorePatterns {
		if t.matchesPattern(path, fileName, pattern) {
			return true
		}
	}

	// Check additional excludes from user/environment
	for _, pattern := range additionalExcludes {
		if t.matchesPattern(path, fileName, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a file path matches a given pattern
func (t *FindLongFilesTool) matchesPattern(path, fileName, pattern string) bool {
	// Handle exact filename matches (like LICENSE)
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "/") {
		return fileName == pattern || strings.HasPrefix(fileName, pattern+".")
	}

	// Handle directory patterns like "**/.git/**"
	if strings.HasSuffix(pattern, "/**") {
		dirPattern := strings.TrimSuffix(pattern, "/**")
		// Remove leading **/ if present
		dirPattern = strings.TrimPrefix(dirPattern, "**/")
		// Check if path contains this directory
		return strings.Contains(path, "/"+dirPattern+"/") ||
			strings.HasPrefix(path, dirPattern+"/") ||
			strings.HasPrefix(path, "./"+dirPattern+"/")
	}

	// Handle patterns starting with **/ (like "**/*.bin")
	if strings.HasPrefix(pattern, "**/") {
		simplePattern := strings.TrimPrefix(pattern, "**/")
		// Try to match against filename
		if matched, _ := filepath.Match(simplePattern, fileName); matched {
			return true
		}
		// Try to match against the entire path
		if matched, _ := filepath.Match(simplePattern, path); matched {
			return true
		}
	}

	// Handle regular glob patterns
	if matched, _ := filepath.Match(pattern, fileName); matched {
		return true
	}
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	// Handle simple substring matches for patterns without globs
	if !strings.Contains(pattern, "*") {
		return strings.Contains(path, pattern)
	}

	return false
}

// isBinaryFile checks if a file is binary by reading a small sample
func (t *FindLongFilesTool) isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return true // Assume binary if we can't read it
	}
	defer func() {
		_ = file.Close()
	}()

	// Read first 512 bytes to check for binary content
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return true
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}

	return false
}

// countLines efficiently counts lines in a file using bytes.Count for optimal performance
func (t *FindLongFilesTool) countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = file.Close()
	}()

	// Use optimised buffer-based counting for better performance
	// Based on JimBLineCounter from go-line-counter benchmark results
	buf := make([]byte, bufferSize)
	lineCount := 0
	lineSep := []byte("\n")

	for {
		n, err := file.Read(buf)
		if n > 0 {
			lineCount += bytes.Count(buf[:n], lineSep)
		}
		if err != nil {
			if err == io.EOF {
				return lineCount, nil
			}
			return lineCount, err
		}
	}
}

// sortByDirectoryTotals sorts files by directory totals
func (t *FindLongFilesTool) sortByDirectoryTotals(files []FileInfo) []FileInfo {
	// Group files by directory
	dirMap := make(map[string]*DirectoryInfo)
	for _, file := range files {
		if dirInfo, exists := dirMap[file.Directory]; exists {
			dirInfo.Files = append(dirInfo.Files, file)
			dirInfo.TotalLines += file.LineCount
			dirInfo.FileCount++
		} else {
			dirMap[file.Directory] = &DirectoryInfo{
				Path:       file.Directory,
				Files:      []FileInfo{file},
				TotalLines: file.LineCount,
				FileCount:  1,
			}
		}
	}

	// Convert to slice and sort by total lines, then by file count
	var dirs []*DirectoryInfo
	for _, dirInfo := range dirMap {
		dirs = append(dirs, dirInfo)
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].TotalLines == dirs[j].TotalLines {
			return dirs[i].FileCount > dirs[j].FileCount
		}
		return dirs[i].TotalLines > dirs[j].TotalLines
	})

	// Flatten back to file list
	var result []FileInfo
	for _, dirInfo := range dirs {
		// Sort files within directory by line count
		sort.Slice(dirInfo.Files, func(i, j int) bool {
			return dirInfo.Files[i].LineCount > dirInfo.Files[j].LineCount
		})
		result = append(result, dirInfo.Files...)
	}

	return result
}

// generateChecklist generates the formatted checklist output
func (t *FindLongFilesTool) generateChecklist(files []FileInfo, request *FindLongFilesRequest, startTime time.Time) string {
	var builder strings.Builder

	// Format calculation time in seconds with one decimal place
	calculationTime := time.Since(startTime).Seconds()
	timeFormatted := fmt.Sprintf("%.1fs", calculationTime)

	builder.WriteString(fmt.Sprintf("# Checklist of files over %d lines\n\n", request.LineThreshold))
	builder.WriteString(fmt.Sprintf("Last checked: %s\n", startTime.Format("2006-01-02 15:04:05")))
	builder.WriteString(fmt.Sprintf("Calculated in: %s\n\n", timeFormatted))

	if len(files) == 0 {
		builder.WriteString("No files found exceeding the line threshold.\n")
		return builder.String()
	}

	for _, file := range files {
		sizeFormatted := t.formatFileSize(file.SizeBytes)
		builder.WriteString(fmt.Sprintf("- [ ] `%s`: %d Lines, %s\n", file.Path, file.LineCount, sizeFormatted))
	}

	return builder.String()
}

// hasReadPermission checks if we have read permission on a file or directory
func (t *FindLongFilesTool) hasReadPermission(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() {
		_ = file.Close()
	}()
	return true
}

// formatFileSize formats file size in human-readable format
func (t *FindLongFilesTool) formatFileSize(sizeBytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case sizeBytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(sizeBytes)/GB)
	case sizeBytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(sizeBytes)/MB)
	case sizeBytes >= KB:
		return fmt.Sprintf("%.0fKB", float64(sizeBytes)/KB)
	default:
		return fmt.Sprintf("%dB", sizeBytes)
	}
}

// getMaxFileSizeKB returns the configured maximum file size in KB
func (t *FindLongFilesTool) getMaxFileSizeKB() int {
	maxFileSizeKB := defaultMaxFileSizeKB
	if envMaxSize := os.Getenv("LONG_FILES_MAX_SIZE_KB"); envMaxSize != "" {
		if parsedSize, err := strconv.Atoi(envMaxSize); err == nil && parsedSize > 0 {
			maxFileSizeKB = parsedSize
		}
	}
	return maxFileSizeKB
}

// newToolResultText creates a new tool result with text content
func (t *FindLongFilesTool) newToolResultText(content string) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText(content), nil
}

// ProvideExtendedInfo provides detailed usage information for the find_long_files tool
func (t *FindLongFilesTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Find files over 700 lines in a project",
				Arguments: map[string]interface{}{
					"path": "/Users/username/projects/my-app",
				},
				ExpectedResult: "Returns a checklist of files exceeding 700 lines with their line counts and file sizes, sorted by line count descending",
			},
			{
				Description: "Find files over 500 lines with custom threshold",
				Arguments: map[string]interface{}{
					"path":           "/Users/username/projects/large-codebase",
					"line_threshold": 500,
				},
				ExpectedResult: "Returns files exceeding 500 lines, useful for more aggressive refactoring targets or smaller codebases",
			},
			{
				Description: "Find long files excluding additional patterns",
				Arguments: map[string]interface{}{
					"path":                "/Users/username/projects/web-app",
					"line_threshold":      600,
					"additional_excludes": []string{"**/*.generated.ts", "**/*.spec.ts", "**/migrations/**"},
				},
				ExpectedResult: "Finds files over 600 lines while excluding generated files, tests, and database migrations from the scan",
			},
			{
				Description: "Find files over 1000 lines for major refactoring",
				Arguments: map[string]interface{}{
					"path":           "/Users/username/projects/legacy-system",
					"line_threshold": 1000,
				},
				ExpectedResult: "Returns only files with extremely high line counts (1000+) that are prime candidates for major refactoring",
			},
		},
		CommonPatterns: []string{
			"Start with default threshold (700 lines) to get a baseline of refactoring candidates",
			"Exclude test files, generated data, and migrations with additional_excludes",
			"Use the checklist output to track refactoring progress over time",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Path does not exist error",
				Solution: "Ensure the path parameter is an absolute path (starts with /) and the directory actually exists. Check for typos in the path.",
			},
			{
				Problem:  "Permission denied errors during scan",
				Solution: "The tool will skip files and directories it cannot read due to permissions. This is normal behaviour - inaccessible files are automatically excluded from results.",
			},
			{
				Problem:  "Too many files being flagged as long",
				Solution: "Increase the line_threshold parameter to focus on the most problematic files, or use additional_excludes to filter out acceptable long files like configuration or data files.",
			},
			{
				Problem:  "Binary files or generated files appearing in results",
				Solution: "The tool automatically excludes most binary files and respects .gitignore. For additional exclusions, use the additional_excludes parameter with glob patterns.",
			},
			{
				Problem:  "Scan taking too long on large codebases",
				Solution: "The tool is optimised for performance but very large codebases may take time. Consider scanning specific subdirectories first, or excluding large vendor/dependency folders.",
			},
		},
		ParameterDetails: map[string]string{
			"path":                "Absolute path to the directory to scan (required). Must start with / on Unix systems or drive letter on Windows. The tool will recursively scan all subdirectories.",
			"line_threshold":      "Minimum line count to consider a file 'long' (default: 700). Common values: 300-500 for strict standards, 700-1000 for moderate standards, 1000+ for focusing on extreme cases.",
			"additional_excludes": "Array of glob patterns to exclude beyond default exclusions. Examples: ['**/*.test.js', '**/generated/**', '**/*.d.ts']. Useful for excluding test files, generated code, or specific file types.",
		},
		WhenToUse:    "Use during code reviews, refactoring planning, technical debt assessment, or as part of continuous code quality monitoring. Ideal for identifying files that may be difficult to maintain due to their size.",
		WhenNotToUse: "Don't use on directories you don't have read permission for, or when you need analysis of specific file contents rather than just line counts. Not suitable for binary file analysis or dependency management.",
	}
}
