package replaceinallfiles

import (
	"bufio"
	"context"
	"encoding/json"
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
	"github.com/sirupsen/logrus"
)

const (
	defaultMaxWorkers    = 4
	defaultMaxFileSizeKB = 2048
	bufferSize           = 32 * 1024 // 32KB buffer for file reading
)

// ReplaceInAllFilesTool implements find and replace across multiple files
type ReplaceInAllFilesTool struct{}

// ReplacementPair represents a source-target replacement pair
type ReplacementPair struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// ReplaceInAllFilesRequest represents the tool request parameters
type ReplaceInAllFilesRequest struct {
	Path               string            `json:"path"`
	ReplacementPairs   []ReplacementPair `json:"replacement_pairs"`
	DryRun             bool              `json:"dry_run"`
	AdditionalExcludes []string          `json:"additional_excludes"`
}

// FileReplacement represents replacement results for a single file
type FileReplacement struct {
	Path             string         `json:"path"`
	ReplacementCount map[string]int `json:"replacement_count"` // source -> count
	Modified         bool           `json:"modified"`
	Error            string         `json:"error,omitempty"`
}

// ReplaceInAllFilesResponse represents the tool response
type ReplaceInAllFilesResponse struct {
	FilesProcessed []FileReplacement `json:"files_processed"`
	TotalFiles     int               `json:"total_files"`
	ModifiedFiles  int               `json:"modified_files"`
	TotalScanned   int               `json:"total_scanned"`
	SkippedFiles   []string          `json:"skipped_files,omitempty"`
	ExecutionTime  string            `json:"execution_time"`
	DryRun         bool              `json:"dry_run"`
	Summary        string            `json:"summary"`
}

// WorkerJob represents a file processing job
type WorkerJob struct {
	FilePath string
	Content  []byte
}

// WorkerResult represents the result of processing a file
type WorkerResult struct {
	FileReplacement FileReplacement
	Error           error
}

// init registers the replace-in-all-files tool
func init() {
	registry.Register(&ReplaceInAllFilesTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *ReplaceInAllFilesTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"replace_in_all_files",
		mcp.WithDescription(`Efficiently and accurately find and replace one or more matching pieces of text across all files in a given path recursively.

This tool can be used by AI coding agents to perform safe, exact string replacements across large projects. It respects .gitignore patterns, skips binary files, and only operates on files with write permissions.

The tool will only replace EXACT matches of the source strings with the target strings (single line only). It handles special characters, quotes, and symbols safely without interpretation.`),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute path to directory or file to operate on (e.g. '/Users/username/project' or '/path/to/file.txt')"),
		),
		mcp.WithArray("replacement_pairs",
			mcp.Required(),
			mcp.Description("Array of replacement pairs, each containing 'source' and 'target' strings. Example: [{\"source\": \"oldFunction\", \"target\": \"newFunction\"}, {\"source\": \"console.log\", \"target\": \"logger.info\"}]"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("If true, only preview changes without modifying files (default: false)"),
			mcp.DefaultBool(false),
		),
		mcp.WithArray("additional_excludes",
			mcp.Description("Optional: Additional glob patterns to exclude (In addition to the default exclusions of .gitignore globs and binary files)"),
		),
	)
}

// Execute executes the replace-in-all-files tool
func (t *ReplaceInAllFilesTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	startTime := time.Now()
	logger.Info("Executing replace-in-all-files tool")

	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"path":                request.Path,
		"replacement_pairs":   len(request.ReplacementPairs),
		"dry_run":             request.DryRun,
		"additional_excludes": request.AdditionalExcludes,
	}).Debug("Replace in all files parameters")

	// Find files to process
	filesToProcess, totalScanned, skippedFiles, err := t.findFilesToProcess(ctx, logger, request)
	if err != nil {
		return nil, fmt.Errorf("failed to find files to process: %w", err)
	}

	// Process files using worker pool
	results, err := t.processFiles(ctx, logger, filesToProcess, request)
	if err != nil {
		return nil, fmt.Errorf("failed to process files: %w", err)
	}

	// Generate response
	response := t.generateResponse(results, totalScanned, skippedFiles, request, startTime)

	logger.WithFields(logrus.Fields{
		"total_files":    response.TotalFiles,
		"modified_files": response.ModifiedFiles,
		"execution_time": response.ExecutionTime,
		"dry_run":        response.DryRun,
	}).Info("Replace in all files completed successfully")

	return t.newToolResultJSON(response)
}

// parseRequest parses and validates the tool arguments
func (t *ReplaceInAllFilesTool) parseRequest(args map[string]interface{}) (*ReplaceInAllFilesRequest, error) {
	request := &ReplaceInAllFilesRequest{
		DryRun:             false,
		AdditionalExcludes: []string{},
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

	// Validate path exists
	if _, err := os.Stat(pathRaw); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", pathRaw)
	}

	request.Path = pathRaw

	// Parse replacement_pairs (required)
	pairsRaw, ok := args["replacement_pairs"].([]interface{})
	if !ok || len(pairsRaw) == 0 {
		return nil, fmt.Errorf("missing required parameter: replacement_pairs")
	}

	for i, pairRaw := range pairsRaw {
		pairMap, ok := pairRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("replacement_pairs[%d] must be an object", i)
		}

		source, ok := pairMap["source"].(string)
		if !ok || source == "" {
			return nil, fmt.Errorf("replacement_pairs[%d].source is required and must be a non-empty string", i)
		}

		target, ok := pairMap["target"].(string)
		if !ok {
			return nil, fmt.Errorf("replacement_pairs[%d].target is required and must be a string", i)
		}

		// Check for duplicate source strings
		for _, existingPair := range request.ReplacementPairs {
			if existingPair.Source == source {
				return nil, fmt.Errorf("duplicate source string in replacement_pairs: %s", source)
			}
		}

		request.ReplacementPairs = append(request.ReplacementPairs, ReplacementPair{
			Source: source,
			Target: target,
		})
	}

	// Parse dry_run (optional)
	if dryRunRaw, ok := args["dry_run"].(bool); ok {
		request.DryRun = dryRunRaw
	}

	// Parse additional_excludes (optional)
	if excludesRaw, ok := args["additional_excludes"].([]interface{}); ok {
		excludes := make([]string, len(excludesRaw))
		for i, exclude := range excludesRaw {
			if excludeStr, ok := exclude.(string); ok {
				excludes[i] = excludeStr
			}
		}
		request.AdditionalExcludes = excludes
	}

	return request, nil
}

// findFilesToProcess finds all files that should be processed
func (t *ReplaceInAllFilesTool) findFilesToProcess(ctx context.Context, logger *logrus.Logger, request *ReplaceInAllFilesRequest) ([]string, int, []string, error) {
	var filesToProcess []string
	var skippedFiles []string
	totalScanned := 0

	// Get maximum file size from environment variable or use default
	maxFileSizeKB := t.getMaxFileSizeKB()
	maxFileSize := int64(maxFileSizeKB * 1024) // Convert KB to bytes

	// Load gitignore patterns
	gitignorePatterns, err := t.loadGitignorePatterns(request.Path)
	if err != nil {
		logger.WithError(err).Warn("Failed to load .gitignore patterns, continuing without them")
	}

	// Determine if we're processing a single file or directory
	stat, err := os.Stat(request.Path)
	if err != nil {
		return nil, totalScanned, skippedFiles, err
	}

	if !stat.IsDir() {
		// Single file processing
		if t.shouldProcessFile(request.Path, stat, gitignorePatterns, request.AdditionalExcludes, maxFileSize, logger) {
			filesToProcess = append(filesToProcess, request.Path)
		} else {
			skippedFiles = append(skippedFiles, request.Path)
		}
		totalScanned = 1
		return filesToProcess, totalScanned, skippedFiles, nil
	}

	// Directory processing
	err = filepath.Walk(request.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Handle permission errors gracefully
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

		totalScanned++

		if t.shouldProcessFile(path, info, gitignorePatterns, request.AdditionalExcludes, maxFileSize, logger) {
			filesToProcess = append(filesToProcess, path)
		} else {
			relPath, _ := filepath.Rel(request.Path, path)
			if !strings.HasPrefix(relPath, "./") && !strings.HasPrefix(relPath, "../") {
				relPath = "./" + relPath
			}
			skippedFiles = append(skippedFiles, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, totalScanned, skippedFiles, err
	}

	return filesToProcess, totalScanned, skippedFiles, nil
}

// shouldProcessFile determines if a file should be processed
func (t *ReplaceInAllFilesTool) shouldProcessFile(path string, info os.FileInfo, gitignorePatterns, additionalExcludes []string, maxFileSize int64, logger *logrus.Logger) bool {
	// Check file size
	if info.Size() > maxFileSize {
		return false
	}

	// Check if file should be excluded
	if t.shouldExcludeFile(path, gitignorePatterns, additionalExcludes) {
		return false
	}

	// Check if we have write permission
	if !t.hasWritePermission(path) {
		logger.WithField("file", path).Debug("Skipping file due to lack of write permission")
		return false
	}

	// Skip binary files
	if t.isBinaryFile(path) {
		return false
	}

	return true
}

// processFiles processes files using a worker pool pattern
func (t *ReplaceInAllFilesTool) processFiles(ctx context.Context, logger *logrus.Logger, filesToProcess []string, request *ReplaceInAllFilesRequest) ([]FileReplacement, error) {
	if len(filesToProcess) == 0 {
		return []FileReplacement{}, nil
	}

	// Get number of workers from environment or use default
	maxWorkers := t.getMaxWorkers()

	// Create channels for job distribution and result collection
	jobs := make(chan WorkerJob, len(filesToProcess))
	results := make(chan WorkerResult, len(filesToProcess))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go t.worker(ctx, logger, jobs, results, request, &wg)
	}

	// Send jobs
	go func() {
		defer close(jobs)
		for _, filePath := range filesToProcess {
			// Read file content
			content, err := os.ReadFile(filePath)
			if err != nil {
				logger.WithError(err).WithField("file", filePath).Warn("Failed to read file")
				results <- WorkerResult{
					FileReplacement: FileReplacement{
						Path:  filePath,
						Error: err.Error(),
					},
					Error: err,
				}
				continue
			}

			jobs <- WorkerJob{
				FilePath: filePath,
				Content:  content,
			}
		}
	}()

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []FileReplacement
	for result := range results {
		allResults = append(allResults, result.FileReplacement)
	}

	return allResults, nil
}

// worker processes files from the jobs channel
func (t *ReplaceInAllFilesTool) worker(ctx context.Context, logger *logrus.Logger, jobs <-chan WorkerJob, results chan<- WorkerResult, request *ReplaceInAllFilesRequest, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
			result := t.processFile(job, request, logger)
			results <- result
		}
	}
}

// processFile processes a single file
func (t *ReplaceInAllFilesTool) processFile(job WorkerJob, request *ReplaceInAllFilesRequest, logger *logrus.Logger) WorkerResult {
	fileReplacement := FileReplacement{
		Path:             job.FilePath,
		ReplacementCount: make(map[string]int),
		Modified:         false,
	}

	// Create a strings.Replacer for efficient multiple replacements
	var oldStrings, newStrings []string
	for _, pair := range request.ReplacementPairs {
		oldStrings = append(oldStrings, pair.Source)
		newStrings = append(newStrings, pair.Target)
	}
	replacer := strings.NewReplacer(func() []string {
		var args []string
		for i := range oldStrings {
			args = append(args, oldStrings[i], newStrings[i])
		}
		return args
	}()...)

	// Convert content to string for processing
	originalContent := string(job.Content)

	// Count occurrences of each source string before replacement
	for _, pair := range request.ReplacementPairs {
		count := strings.Count(originalContent, pair.Source)
		if count > 0 {
			fileReplacement.ReplacementCount[pair.Source] = count
			fileReplacement.Modified = true
		}
	}

	// If no replacements needed, return early
	if !fileReplacement.Modified {
		return WorkerResult{FileReplacement: fileReplacement}
	}

	// Perform replacement if not dry run
	if !request.DryRun {
		newContent := replacer.Replace(originalContent)

		// Write the modified content back to file
		err := os.WriteFile(job.FilePath, []byte(newContent), 0644)
		if err != nil {
			fileReplacement.Error = err.Error()
			logger.WithError(err).WithField("file", job.FilePath).Error("Failed to write file")
			return WorkerResult{
				FileReplacement: fileReplacement,
				Error:           err,
			}
		}
	}

	return WorkerResult{FileReplacement: fileReplacement}
}

// generateResponse creates the final response
func (t *ReplaceInAllFilesTool) generateResponse(results []FileReplacement, totalScanned int, skippedFiles []string, request *ReplaceInAllFilesRequest, startTime time.Time) *ReplaceInAllFilesResponse {
	modifiedFiles := 0
	totalReplacements := 0

	for _, result := range results {
		if result.Modified {
			modifiedFiles++
		}
		for _, count := range result.ReplacementCount {
			totalReplacements += count
		}
	}

	// Sort results by path for consistent output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	// Generate summary
	var summary string
	if request.DryRun {
		summary = fmt.Sprintf("DRY RUN: Would modify %d files with %d total replacements", modifiedFiles, totalReplacements)
	} else {
		if modifiedFiles == 0 {
			summary = "No files were modified - no matching text found"
		} else {
			summary = fmt.Sprintf("Successfully modified %d files with %d total replacements", modifiedFiles, totalReplacements)
		}
	}

	return &ReplaceInAllFilesResponse{
		FilesProcessed: results,
		TotalFiles:     len(results),
		ModifiedFiles:  modifiedFiles,
		TotalScanned:   totalScanned,
		SkippedFiles:   skippedFiles,
		ExecutionTime:  time.Since(startTime).String(),
		DryRun:         request.DryRun,
		Summary:        summary,
	}
}

// Utility functions reused from find_long_files tool

// loadGitignorePatterns loads gitignore patterns from .gitignore file
func (t *ReplaceInAllFilesTool) loadGitignorePatterns(basePath string) ([]string, error) {
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
func (t *ReplaceInAllFilesTool) getDefaultExcludePatterns() []string {
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
func (t *ReplaceInAllFilesTool) shouldExcludeFile(path string, gitignorePatterns, additionalExcludes []string) bool {
	fileName := filepath.Base(path)

	// Check default exclusions
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

	// Check additional excludes
	for _, pattern := range additionalExcludes {
		if t.matchesPattern(path, fileName, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a file path matches a given pattern
func (t *ReplaceInAllFilesTool) matchesPattern(path, fileName, pattern string) bool {
	// Handle exact filename matches
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "/") {
		return fileName == pattern || strings.HasPrefix(fileName, pattern+".")
	}

	// Handle directory patterns
	if strings.HasSuffix(pattern, "/**") {
		dirPattern := strings.TrimSuffix(pattern, "/**")
		dirPattern = strings.TrimPrefix(dirPattern, "**/")
		return strings.Contains(path, "/"+dirPattern+"/") ||
			strings.HasPrefix(path, dirPattern+"/") ||
			strings.HasPrefix(path, "./"+dirPattern+"/")
	}

	// Handle patterns starting with **/
	if strings.HasPrefix(pattern, "**/") {
		simplePattern := strings.TrimPrefix(pattern, "**/")
		if matched, _ := filepath.Match(simplePattern, fileName); matched {
			return true
		}
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

	// Handle simple substring matches
	if !strings.Contains(pattern, "*") {
		return strings.Contains(path, pattern)
	}

	return false
}

// isBinaryFile checks if a file is binary by reading a small sample
func (t *ReplaceInAllFilesTool) isBinaryFile(path string) bool {
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

// hasReadPermission checks if we have read permission on a file or directory
func (t *ReplaceInAllFilesTool) hasReadPermission(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() {
		_ = file.Close()
	}()
	return true
}

// hasWritePermission checks if we have write permission on a file
func (t *ReplaceInAllFilesTool) hasWritePermission(path string) bool {
	// Try to open the file for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0666)
	if err != nil {
		return false
	}
	defer func() {
		_ = file.Close()
	}()
	return true
}

// getMaxFileSizeKB returns the configured maximum file size in KB
func (t *ReplaceInAllFilesTool) getMaxFileSizeKB() int {
	maxFileSizeKB := defaultMaxFileSizeKB
	if envMaxSize := os.Getenv("REPLACE_FILES_MAX_SIZE_KB"); envMaxSize != "" {
		if parsedSize, err := strconv.Atoi(envMaxSize); err == nil && parsedSize > 0 {
			maxFileSizeKB = parsedSize
		}
	}
	return maxFileSizeKB
}

// getMaxWorkers returns the configured maximum number of workers
func (t *ReplaceInAllFilesTool) getMaxWorkers() int {
	maxWorkers := defaultMaxWorkers
	if envMaxWorkers := os.Getenv("REPLACE_FILES_MAX_WORKERS"); envMaxWorkers != "" {
		if parsedWorkers, err := strconv.Atoi(envMaxWorkers); err == nil && parsedWorkers > 0 {
			maxWorkers = parsedWorkers
		}
	}
	return maxWorkers
}

// newToolResultJSON creates a new tool result with JSON content
func (t *ReplaceInAllFilesTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}
