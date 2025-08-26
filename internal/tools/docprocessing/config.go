package docprocessing

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/config"
	"github.com/sammcj/mcp-devtools/internal/security"
)

const (
	// Document processing security limits
	DefaultMaxMemoryLimit             = int64(5 * 1024 * 1024 * 1024) // 5GB default memory limit
	DefaultMaxFileSizeMB              = 100                           // Default file size in MB
	DocProcessingMaxMemoryLimitEnvVar = "DOCLING_MAX_MEMORY_LIMIT"
	DocProcessingMaxFileSizeEnvVar    = "DOCLING_MAX_FILE_SIZE"
)

// Supported file types for document processing
var SupportedFileTypes = map[string]bool{
	// Document formats
	".pdf":  true,
	".docx": true,
	".doc":  true, // Legacy Word format
	".xlsx": true,
	".xls":  true, // Legacy Excel format
	".pptx": true,
	".ppt":  true, // Legacy PowerPoint format
	".txt":  true, // Plain text
	".md":   true, // Markdown
	".rtf":  true, // Rich Text Format
	// Web formats
	".html": true,
	".htm":  true,
	".csv":  true,
	// Image formats
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
}

// Config holds the configuration for document processing
type Config struct {
	// Python Configuration
	PythonPath string // Path to Python executable with Docling installed

	// Cache Configuration
	CacheDir     string // Directory for caching processed documents
	CacheEnabled bool   // Enable/disable caching

	// Hardware Configuration
	HardwareAcceleration HardwareAcceleration // Hardware acceleration mode

	// Processing Configuration
	Timeout        int   // Processing timeout in seconds
	MaxFileSize    int   // Maximum file size in MB
	MaxMemoryLimit int64 // Maximum memory limit in bytes

	// OCR Configuration
	OCRLanguages []string // Default OCR languages

	// Vision Model Configuration
	VisionModel string // Vision model to use

	// Certificate Configuration
	ExtraCACerts string // Path to additional CA certificates file or directory
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultCacheDir := filepath.Join(homeDir, ".mcp-devtools", "docling-cache")

	return &Config{
		PythonPath:           detectPythonPath(),
		CacheDir:             defaultCacheDir,
		CacheEnabled:         true,
		HardwareAcceleration: HardwareAccelerationAuto,
		Timeout:              300,                   // 5 minutes
		MaxFileSize:          DefaultMaxFileSizeMB,  // 100 MB
		MaxMemoryLimit:       DefaultMaxMemoryLimit, // 5GB
		OCRLanguages:         []string{"en"},
		VisionModel:          "SmolDocling",
	}
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	config := DefaultConfig()

	// Python Configuration
	if pythonPath := os.Getenv("DOCLING_PYTHON_PATH"); pythonPath != "" {
		config.PythonPath = pythonPath
	}

	// Cache Configuration
	if cacheDir := os.Getenv("DOCLING_CACHE_DIR"); cacheDir != "" {
		config.CacheDir = cacheDir
	}

	if cacheEnabled := os.Getenv("DOCLING_CACHE_ENABLED"); cacheEnabled != "" {
		if enabled, err := strconv.ParseBool(cacheEnabled); err == nil {
			config.CacheEnabled = enabled
		}
	}

	// Hardware Configuration
	if hwAccel := os.Getenv("DOCLING_HARDWARE_ACCELERATION"); hwAccel != "" {
		switch strings.ToLower(hwAccel) {
		case "auto":
			config.HardwareAcceleration = HardwareAccelerationAuto
		case "mps":
			config.HardwareAcceleration = HardwareAccelerationMPS
		case "cuda":
			config.HardwareAcceleration = HardwareAccelerationCUDA
		case "cpu":
			config.HardwareAcceleration = HardwareAccelerationCPU
		}
	}

	// Processing Configuration
	if timeout := os.Getenv("DOCLING_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			config.Timeout = t
		}
	}

	if maxFileSize := os.Getenv("DOCLING_MAX_FILE_SIZE"); maxFileSize != "" {
		if size, err := strconv.Atoi(maxFileSize); err == nil && size > 0 {
			config.MaxFileSize = size
		}
	}

	if maxMemoryLimit := os.Getenv("DOCLING_MAX_MEMORY_LIMIT"); maxMemoryLimit != "" {
		if limit, err := strconv.ParseInt(maxMemoryLimit, 10, 64); err == nil && limit > 0 {
			config.MaxMemoryLimit = limit
		}
	}

	// OCR Configuration
	if ocrLangs := os.Getenv("DOCLING_OCR_LANGUAGES"); ocrLangs != "" {
		languages := strings.Split(ocrLangs, ",")
		for i, lang := range languages {
			languages[i] = strings.TrimSpace(lang)
		}
		config.OCRLanguages = languages
	}

	// Vision Model Configuration
	if visionModel := os.Getenv("DOCLING_VISION_MODEL"); visionModel != "" {
		config.VisionModel = visionModel
	}

	// Certificate Configuration
	if extraCACerts := os.Getenv("DOCLING_EXTRA_CA_CERTS"); extraCACerts != "" {
		config.ExtraCACerts = extraCACerts
	}

	return config
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate Python path
	if c.PythonPath == "" {
		return fmt.Errorf("docling package not found! `Run pip install -U docling` in the Python environment your MCP client is using. Once installed you can optionally run docling-tools models download to automatically download the advanced vision models")
	}

	// Validate cache directory
	if c.CacheEnabled && c.CacheDir == "" {
		return fmt.Errorf("cache directory is required when caching is enabled")
	}

	// Validate timeout
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	// Validate max file size
	if c.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be greater than 0")
	}

	// Validate max memory limit
	if c.MaxMemoryLimit <= 0 {
		return fmt.Errorf("max memory limit must be greater than 0")
	}

	// Validate OCR languages
	if len(c.OCRLanguages) == 0 {
		return fmt.Errorf("at least one OCR language must be specified")
	}

	// Validate certificates if configured
	if err := c.ValidateCertificates(); err != nil {
		return fmt.Errorf("certificate validation failed: %w", err)
	}

	return nil
}

// EnsureCacheDir creates the cache directory if it doesn't exist
func (c *Config) EnsureCacheDir() error {
	if !c.CacheEnabled {
		return nil
	}

	// Security: Check file access for cache directory
	if err := security.CheckFileAccess(c.CacheDir); err != nil {
		return fmt.Errorf("cache directory access denied: %w", err)
	}

	if err := os.MkdirAll(c.CacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", c.CacheDir, err)
	}

	return nil
}

// CleanupTemporaryFiles performs cleanup of temporary files and directories
func (c *Config) CleanupTemporaryFiles() error {
	var errors []string

	// Clean up embedded scripts temporary directory
	if err := CleanupEmbeddedScripts(); err != nil {
		errors = append(errors, fmt.Sprintf("embedded scripts cleanup failed: %v", err))
	}

	// Clean up old cache files (older than configured max age)
	if c.CacheEnabled && c.CacheDir != "" {
		maxAge := 6 * 7 * 24 * time.Hour // 6 weeks default
		if maxAgeEnv := os.Getenv("DOCLING_CACHE_MAX_AGE_HOURS"); maxAgeEnv != "" {
			if hours, err := strconv.Atoi(maxAgeEnv); err == nil && hours > 0 {
				maxAge = time.Duration(hours) * time.Hour
			}
		}

		if err := c.cleanupCacheFiles(maxAge); err != nil {
			errors = append(errors, fmt.Sprintf("cache cleanup failed: %v", err))
		}
	}

	// Clean up system temporary directory for mcp-devtools files
	if err := c.cleanupSystemTempFiles(); err != nil {
		errors = append(errors, fmt.Sprintf("system temp cleanup failed: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// cleanupCacheFiles removes old cache files
func (c *Config) cleanupCacheFiles(maxAge time.Duration) error {
	if !c.CacheEnabled || c.CacheDir == "" {
		return nil
	}

	cutoffTime := time.Now().Add(-maxAge)
	return filepath.Walk(c.CacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			_ = os.Remove(path) // Silently remove old files
		}

		return nil
	})
}

// cleanupSystemTempFiles removes old temporary files created by mcp-devtools
func (c *Config) cleanupSystemTempFiles() error {
	tempDir := os.TempDir()
	cutoffTime := time.Now().Add(-24 * time.Hour) // Remove files older than 24 hours

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return nil // Silently fail if we can't read temp directory
	}

	for _, entry := range entries {
		// Only clean up mcp-devtools related temp files
		if !strings.HasPrefix(entry.Name(), "mcp-devtools-") {
			continue
		}

		filePath := filepath.Join(tempDir, entry.Name())
		if info, err := os.Stat(filePath); err == nil {
			if info.ModTime().Before(cutoffTime) {
				if entry.IsDir() {
					_ = os.RemoveAll(filePath)
				} else {
					_ = os.Remove(filePath)
				}
			}
		}
	}

	return nil
}

// GetMaxMemoryLimit returns the configured maximum memory limit in bytes
func (c *Config) GetMaxMemoryLimit() int64 {
	return c.MaxMemoryLimit
}

// ValidateFileSize validates that the file size is within limits
func (c *Config) ValidateFileSize(fileSizeBytes int64) error {
	maxSizeBytes := int64(c.MaxFileSize) * 1024 * 1024 // Convert MB to bytes
	if fileSizeBytes > maxSizeBytes {
		sizeMB := float64(fileSizeBytes) / (1024 * 1024)
		maxSizeMB := float64(maxSizeBytes) / (1024 * 1024)
		return fmt.Errorf("document file size %.1fMB exceeds maximum allowed size of %.1fMB (use %s environment variable to adjust limit)", sizeMB, maxSizeMB, DocProcessingMaxFileSizeEnvVar)
	}
	return nil
}

// ValidateMemoryLimit validates that memory usage is within limits
func (c *Config) ValidateMemoryLimit() error {
	// This is a placeholder for future memory monitoring implementation
	// For now, we rely on the file size limits and Python process limits
	// to prevent excessive memory usage during document processing
	if c.MaxMemoryLimit <= 0 {
		return fmt.Errorf("memory limit must be greater than 0")
	}
	return nil
}

// ValidateFileType validates that the file type is supported for processing
func (c *Config) ValidateFileType(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return fmt.Errorf("file has no extension, unable to determine file type")
	}

	if !SupportedFileTypes[ext] {
		supportedTypes := make([]string, 0, len(SupportedFileTypes))
		for fileType := range SupportedFileTypes {
			supportedTypes = append(supportedTypes, fileType)
		}
		return fmt.Errorf("unsupported file type '%s'. Supported types: %s", ext, strings.Join(supportedTypes, ", "))
	}

	return nil
}

// GetSystemInfo returns system information for diagnostics
func (c *Config) GetSystemInfo() *SystemInfo {
	info := &SystemInfo{
		Platform:         runtime.GOOS,
		Architecture:     runtime.GOARCH,
		PythonPath:       c.PythonPath,
		DoclingAvailable: c.isDoclingAvailable(),
		CacheDirectory:   c.CacheDir,
		CacheEnabled:     c.CacheEnabled,
		MaxFileSize:      c.MaxFileSize,
		MaxMemoryLimit:   c.MaxMemoryLimit,
		DefaultTimeout:   c.Timeout,
	}

	// Detect available hardware acceleration
	info.HardwareAcceleration = c.detectAvailableAcceleration()

	// Get Python and Docling versions if available
	if pythonVersion := c.getPythonVersion(); pythonVersion != "" {
		info.PythonVersion = pythonVersion
	}

	if doclingVersion := c.getDoclingVersion(); doclingVersion != "" {
		info.DoclingVersion = doclingVersion
	}

	return info
}

// detectPythonPath attempts to find a suitable Python executable with docling
// Priority order: 1. Environment variable, 2. State file, 3. Discovery, 4. Auto-install + update state
func detectPythonPath() string {
	// 1. Check environment variable first (highest priority)
	if envPath := os.Getenv("DOCLING_PYTHON_PATH"); envPath != "" {
		if isDoclingAvailableInPython(envPath) {
			return envPath
		}
	}

	// 2. Check state file (if not stale)
	state := config.GetGlobalState()
	if !state.IsStale() {
		cachedPath, doclingAvailable := state.GetPythonPath()
		if cachedPath != "" && doclingAvailable && isDoclingAvailableInPython(cachedPath) {
			return cachedPath
		}
	}

	// 3. Discovery - find available Python with docling
	pythonPaths := getCommonPythonPaths()
	for _, pythonPath := range pythonPaths {
		if isDoclingAvailableInPython(pythonPath) {
			// Update state file with discovered path for faster startup next time
			_ = state.SetPythonPath(pythonPath, true)
			return pythonPath
		}
	}

	// No Python with docling found - update state to avoid repeated searches
	_ = state.SetPythonPath("", false)
	return ""
}

// getCommonPythonPaths returns a list of common Python installation paths
func getCommonPythonPaths() []string {
	var paths []string

	// Get user home directory
	homeDir, _ := os.UserHomeDir()

	// Common Python executable names
	pythonNames := []string{"python3", "python", "python3.13", "python3.12", "python3.11", "python3.10"}

	// 1. Check PATH first (most common case)
	for _, name := range pythonNames {
		if path, err := findExecutable(name); err == nil {
			paths = append(paths, path)
		}
	}

	// 2. Common virtual environment locations
	commonVenvPaths := []string{
		filepath.Join(homeDir, ".venv", "bin"),
		filepath.Join(homeDir, "venv", "bin"),
		filepath.Join(homeDir, ".pyenv", "shims"),
	}

	for _, venvPath := range commonVenvPaths {
		for _, name := range pythonNames {
			fullPath := filepath.Join(venvPath, name)
			if _, err := os.Stat(fullPath); err == nil {
				paths = append(paths, fullPath)
			}
		}
	}

	// 3. Homebrew Python locations (macOS)
	if runtime.GOOS == "darwin" {
		brewPaths := []string{
			"/opt/homebrew/bin", // Apple Silicon
			"/opt/homebrew/opt/python@3.13/bin",
			"/opt/homebrew/opt/python@3.12/bin",
			"/opt/homebrew/opt/python@3.11/bin",
			"/opt/homebrew/opt/python@3.10/bin",
			"/usr/local/opt/python@3.13/bin",
			"/usr/local/opt/python@3.12/bin",
			"/usr/local/opt/python@3.11/bin",
			"/usr/local/opt/python@3.10/bin",
			"/usr/local/bin", // Intel Mac
		}

		for _, brewPath := range brewPaths {
			for _, name := range pythonNames {
				fullPath := filepath.Join(brewPath, name)
				if _, err := os.Stat(fullPath); err == nil {
					paths = append(paths, fullPath)
				}
			}
		}
	}

	// 4. System Python locations
	systemPaths := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/bin",
	}

	for _, sysPath := range systemPaths {
		for _, name := range pythonNames {
			fullPath := filepath.Join(sysPath, name)
			if _, err := os.Stat(fullPath); err == nil {
				paths = append(paths, fullPath)
			}
		}
	}

	// 5. UV Python locations
	uvPaths := []string{
		filepath.Join(homeDir, ".local", "share", "uv", "python"),
		filepath.Join(homeDir, ".uv", "python"),
	}

	for _, uvPath := range uvPaths {
		// UV typically has versioned directories
		if entries, err := os.ReadDir(uvPath); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					binPath := filepath.Join(uvPath, entry.Name(), "bin")
					for _, name := range pythonNames {
						fullPath := filepath.Join(binPath, name)
						if _, err := os.Stat(fullPath); err == nil {
							paths = append(paths, fullPath)
						}
					}
				}
			}
		}
	}

	// 6. Conda/Miniconda locations
	condaPaths := []string{
		filepath.Join(homeDir, "miniconda3", "bin"),
		filepath.Join(homeDir, "anaconda3", "bin"),
		filepath.Join(homeDir, "miniforge3", "bin"),
		"/opt/miniconda3/bin",
		"/opt/anaconda3/bin",
		"/usr/local/miniconda3/bin",
		"/usr/local/anaconda3/bin",
	}

	for _, condaPath := range condaPaths {
		for _, name := range pythonNames {
			fullPath := filepath.Join(condaPath, name)
			if _, err := os.Stat(fullPath); err == nil {
				paths = append(paths, fullPath)
			}
		}
	}

	// Remove duplicates while preserving order
	seen := make(map[string]bool)
	var uniquePaths []string
	for _, path := range paths {
		if !seen[path] {
			seen[path] = true
			uniquePaths = append(uniquePaths, path)
		}
	}

	return uniquePaths
}

// isDoclingAvailableInPython checks if docling is available in the given Python installation
func isDoclingAvailableInPython(pythonPath string) bool {
	if pythonPath == "" {
		return false
	}

	// Check if the Python executable exists and is executable
	if info, err := os.Stat(pythonPath); err != nil || info.IsDir() {
		return false
	}

	// Try to import docling with a short timeout
	cmd := fmt.Sprintf(`%s -c "import docling; print('available')"`, pythonPath)
	if err := runCommand(cmd, 5); err != nil {
		return false
	}

	return true
}

// findExecutable searches for an executable in PATH
func findExecutable(name string) (string, error) {
	// Check if it's already an absolute path
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", fmt.Errorf("executable not found: %s", name)
	}

	// Search in PATH
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}

		fullPath := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			// Try with .exe extension on Windows
			if !strings.HasSuffix(fullPath, ".exe") {
				fullPath += ".exe"
			}
		}

		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("executable not found in PATH: %s", name)
}

// isDoclingAvailable checks if Docling is available in the Python environment
func (c *Config) isDoclingAvailable() bool {
	if c.PythonPath == "" {
		return false
	}

	// Try to import docling
	cmd := fmt.Sprintf(`%s -c "import docling; print('available')"`, c.PythonPath)
	if err := runCommand(cmd, 10); err != nil {
		return false
	}

	return true
}

// getPythonVersion gets the Python version
func (c *Config) getPythonVersion() string {
	if c.PythonPath == "" {
		return ""
	}

	cmd := fmt.Sprintf(`%s --version`, c.PythonPath)
	output, err := runCommandWithOutput(cmd, 5)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}

// getDoclingVersion gets the Docling version
func (c *Config) getDoclingVersion() string {
	if c.PythonPath == "" {
		return ""
	}

	cmd := fmt.Sprintf(`%s -c "import docling; print(docling.__version__)"`, c.PythonPath)
	output, err := runCommandWithOutput(cmd, 2)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}

// detectAvailableAcceleration detects available hardware acceleration options
func (c *Config) detectAvailableAcceleration() []HardwareAcceleration {
	available := []HardwareAcceleration{HardwareAccelerationCPU} // CPU is always available

	// Check for MPS (macOS Metal Performance Shaders)
	if runtime.GOOS == "darwin" {
		if c.isMPSAvailable() {
			available = append(available, HardwareAccelerationMPS)
		}
	}

	// Check for CUDA (NVIDIA GPUs)
	if c.isCUDAAvailable() {
		available = append(available, HardwareAccelerationCUDA)
	}

	return available
}

// isMPSAvailable checks if MPS is available (macOS only)
func (c *Config) isMPSAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	if c.PythonPath == "" {
		return false
	}

	// Check if PyTorch with MPS support is available
	cmd := fmt.Sprintf(`%s -c "import torch; print(torch.backends.mps.is_available())"`, c.PythonPath)
	output, err := runCommandWithOutput(cmd, 10)
	if err != nil {
		return false
	}

	return strings.TrimSpace(output) == "True"
}

// isCUDAAvailable checks if CUDA is available
func (c *Config) isCUDAAvailable() bool {
	if c.PythonPath == "" {
		return false
	}

	// Check if PyTorch with CUDA support is available
	cmd := fmt.Sprintf(`%s -c "import torch; print(torch.cuda.is_available())"`, c.PythonPath)
	output, err := runCommandWithOutput(cmd, 10)
	if err != nil {
		return false
	}

	return strings.TrimSpace(output) == "True"
}

// ResolveHardwareAcceleration resolves the hardware acceleration setting
func (c *Config) ResolveHardwareAcceleration() HardwareAcceleration {
	if c.HardwareAcceleration != HardwareAccelerationAuto {
		return c.HardwareAcceleration
	}

	// Auto-detect best available option
	available := c.detectAvailableAcceleration()

	// Prefer MPS on macOS, then CUDA, then CPU
	for _, accel := range []HardwareAcceleration{HardwareAccelerationMPS, HardwareAccelerationCUDA, HardwareAccelerationCPU} {
		if slices.Contains(available, accel) {
			return accel
		}
	}

	return HardwareAccelerationCPU
}

// GetScriptPath returns the path to the Python wrapper script
func (c *Config) GetScriptPath() string {
	// First check if there's an environment variable override
	if scriptPath := os.Getenv("DOCLING_SCRIPT_PATH"); scriptPath != "" {
		return scriptPath
	}

	// Try to find the script relative to the binary location
	if execPath, err := os.Executable(); err == nil {
		// Get the directory containing the binary
		binDir := filepath.Dir(execPath)

		// Look for the script in common locations relative to the binary
		possiblePaths := []string{
			filepath.Join(binDir, "internal", "tools", "docprocessing", "python", "docling_processor.py"),               // Same level as bin/
			filepath.Join(binDir, "..", "internal", "tools", "docprocessing", "python", "docling_processor.py"),         // One level up
			filepath.Join(filepath.Dir(binDir), "internal", "tools", "docprocessing", "python", "docling_processor.py"), // Project root
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	// Fall back to current working directory
	if cwd, err := os.Getwd(); err == nil {
		scriptPath := filepath.Join(cwd, "internal", "tools", "docprocessing", "python", "docling_processor.py")
		if _, err := os.Stat(scriptPath); err == nil {
			return scriptPath
		}
	}

	// Try embedded scripts as fallback
	if IsEmbeddedScriptsAvailable() {
		if embeddedPath, err := GetEmbeddedScriptPath(); err == nil {
			return embeddedPath
		}
	}

	// Last resort - return relative path (new location)
	return "internal/tools/docprocessing/python/docling_processor.py"
}

// runCommand runs a command with a timeout and returns an error if it fails
func runCommand(cmdStr string, timeoutSeconds int) error {
	_, err := runCommandWithOutput(cmdStr, timeoutSeconds)
	return err
}

// GetCertificateEnvironment returns environment variables for certificate configuration
func (c *Config) GetCertificateEnvironment() []string {
	var env []string

	if c.ExtraCACerts != "" {
		// Validate that the certificate path exists
		if _, err := os.Stat(c.ExtraCACerts); err == nil {
			// Set environment variables for both Python and system certificate handling
			env = append(env, fmt.Sprintf("SSL_CERT_FILE=%s", c.ExtraCACerts))
			env = append(env, fmt.Sprintf("REQUESTS_CA_BUNDLE=%s", c.ExtraCACerts))
			env = append(env, fmt.Sprintf("CURL_CA_BUNDLE=%s", c.ExtraCACerts))

			// For pip and Python package installations
			env = append(env, fmt.Sprintf("PIP_CERT=%s", c.ExtraCACerts))
			env = append(env, "PIP_TRUSTED_HOST=pypi.org,pypi.python.org,files.pythonhosted.org")

			// For conda if used
			env = append(env, fmt.Sprintf("CONDA_SSL_VERIFY=%s", c.ExtraCACerts))
		}
	}

	return env
}

// ValidateCertificates validates the certificate configuration
func (c *Config) ValidateCertificates() error {
	if c.ExtraCACerts == "" {
		return nil // No certificates configured, which is fine
	}

	// Security: Check file access for certificate path
	if err := security.CheckFileAccess(c.ExtraCACerts); err != nil {
		return fmt.Errorf("certificate path access denied: %w", err)
	}

	// Check if the certificate path exists
	info, err := os.Stat(c.ExtraCACerts)
	if err != nil {
		return fmt.Errorf("certificate path does not exist: %s", c.ExtraCACerts)
	}

	// Check if it's a file or directory
	if info.IsDir() {
		// If it's a directory, check if it contains certificate files
		entries, err := os.ReadDir(c.ExtraCACerts)
		if err != nil {
			return fmt.Errorf("cannot read certificate directory: %s", c.ExtraCACerts)
		}

		// Look for common certificate file extensions
		hasCerts := false
		for _, entry := range entries {
			if !entry.IsDir() {
				name := strings.ToLower(entry.Name())
				if strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".crt") ||
					strings.HasSuffix(name, ".cer") || strings.HasSuffix(name, ".ca-bundle") {
					hasCerts = true
					break
				}
			}
		}

		if !hasCerts {
			return fmt.Errorf("certificate directory contains no certificate files: %s", c.ExtraCACerts)
		}
	} else {
		// If it's a file, check if it's readable (security access was already checked above)
		file, err := os.Open(c.ExtraCACerts)
		if err != nil {
			return fmt.Errorf("cannot read certificate file: %s", c.ExtraCACerts)
		}
		_ = file.Close()
	}

	return nil
}

// runCommandWithOutput runs a command with a timeout and returns the output
func runCommandWithOutput(cmdStr string, timeoutSeconds int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Execute command through shell to handle complex commands properly
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timeout after %d seconds", timeoutSeconds)
		}
		return "", err
	}

	return string(output), nil
}
