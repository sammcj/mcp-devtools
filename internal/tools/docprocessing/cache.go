package docprocessing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheManager handles caching of document processing results
type CacheManager struct {
	config *Config
}

// NewCacheManager creates a new cache manager
func NewCacheManager(config *Config) *CacheManager {
	return &CacheManager{
		config: config,
	}
}

// GenerateCacheKey generates a cache key for the given request
func (cm *CacheManager) GenerateCacheKey(req *DocumentProcessingRequest) string {
	// Create a hash based on the request parameters that affect the output
	keyData := struct {
		Source               string               `json:"source"`
		ProcessingMode       ProcessingMode       `json:"processing_mode"`
		EnableOCR            bool                 `json:"enable_ocr"`
		OCRLanguages         []string             `json:"ocr_languages"`
		PreserveImages       bool                 `json:"preserve_images"`
		OutputFormat         OutputFormat         `json:"output_format"`
		TableFormerMode      TableFormerMode      `json:"table_former_mode"`
		CellMatching         *bool                `json:"cell_matching"`
		VisionMode           VisionProcessingMode `json:"vision_mode"`
		DiagramDescription   bool                 `json:"diagram_description"`
		ChartDataExtraction  bool                 `json:"chart_data_extraction"`
		EnableRemoteServices bool                 `json:"enable_remote_services"`
	}{
		Source:               req.Source,
		ProcessingMode:       req.ProcessingMode,
		EnableOCR:            req.EnableOCR,
		OCRLanguages:         req.OCRLanguages,
		PreserveImages:       req.PreserveImages,
		OutputFormat:         req.OutputFormat,
		TableFormerMode:      req.TableFormerMode,
		CellMatching:         req.CellMatching,
		VisionMode:           req.VisionMode,
		DiagramDescription:   req.DiagramDescription,
		ChartDataExtraction:  req.ChartDataExtraction,
		EnableRemoteServices: req.EnableRemoteServices,
	}

	// Convert to JSON and hash
	jsonData, _ := json.Marshal(keyData)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

// GetCacheFilePath returns the file path for a cache key
func (cm *CacheManager) GetCacheFilePath(cacheKey string) string {
	return filepath.Join(cm.config.CacheDir, cacheKey+".json")
}

// Get retrieves a cached result if it exists and is valid
func (cm *CacheManager) Get(cacheKey string) (*DocumentProcessingResponse, bool) {
	if !cm.config.CacheEnabled {
		return nil, false
	}

	filePath := cm.GetCacheFilePath(cacheKey)

	// Check if cache file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, false
	}

	// Read cache file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	// Parse cached response
	var cachedResponse CachedResponse
	if err := json.Unmarshal(data, &cachedResponse); err != nil {
		return nil, false
	}

	// Check if cache is still valid (not expired)
	if cm.isCacheExpired(&cachedResponse) {
		// Remove expired cache file
		_ = os.Remove(filePath)
		return nil, false
	}

	// Validate that referenced files still exist
	if !cm.validateReferencedFiles(&cachedResponse.Response) {
		// Referenced files are missing, invalidate cache
		_ = os.Remove(filePath)
		return nil, false
	}

	// Mark as cache hit
	cachedResponse.Response.CacheHit = true
	return &cachedResponse.Response, true
}

// Set stores a result in the cache
func (cm *CacheManager) Set(cacheKey string, response *DocumentProcessingResponse) error {
	if !cm.config.CacheEnabled {
		return nil
	}

	// Ensure cache directory exists
	if err := cm.config.EnsureCacheDir(); err != nil {
		return fmt.Errorf("failed to ensure cache directory: %w", err)
	}

	// Create cached response with metadata
	cachedResponse := CachedResponse{
		Response:  *response,
		CacheKey:  cacheKey,
		Timestamp: time.Now(),
		TTL:       cm.getDefaultTTL(),
	}

	// Mark as not a cache hit for the stored version
	cachedResponse.Response.CacheHit = false

	// Serialize to JSON
	data, err := json.MarshalIndent(cachedResponse, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	// Write to cache file
	filePath := cm.GetCacheFilePath(cacheKey)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Delete removes a cached result
func (cm *CacheManager) Delete(cacheKey string) error {
	if !cm.config.CacheEnabled {
		return nil
	}

	filePath := cm.GetCacheFilePath(cacheKey)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}

	return nil
}

// ClearFileCache removes all cache entries for a specific source file
func (cm *CacheManager) ClearFileCache(source string) error {
	if !cm.config.CacheEnabled {
		return nil
	}

	// Find all cache files that match this source
	pattern := filepath.Join(cm.config.CacheDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find cache files: %w", err)
	}

	var removedCount int
	for _, match := range matches {
		// Read cache file to check if it matches the source
		if data, err := os.ReadFile(match); err == nil {
			var cachedResponse CachedResponse
			if err := json.Unmarshal(data, &cachedResponse); err == nil {
				// Check if this cache entry is for the same source file
				if cachedResponse.Response.Source == source {
					if err := os.Remove(match); err == nil {
						removedCount++
					}
				}
			}
		}
	}

	return nil
}

// Clear removes all cached results
func (cm *CacheManager) Clear() error {
	if !cm.config.CacheEnabled {
		return nil
	}

	// Remove all .json files in the cache directory
	pattern := filepath.Join(cm.config.CacheDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find cache files: %w", err)
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			return fmt.Errorf("failed to remove cache file %s: %w", match, err)
		}
	}

	return nil
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() (*CacheStats, error) {
	stats := &CacheStats{
		Enabled:   cm.config.CacheEnabled,
		Directory: cm.config.CacheDir,
	}

	if !cm.config.CacheEnabled {
		return stats, nil
	}

	// Count cache files
	pattern := filepath.Join(cm.config.CacheDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return stats, fmt.Errorf("failed to find cache files: %w", err)
	}

	stats.TotalFiles = len(matches)

	// Calculate total size and check for expired files
	var totalSize int64
	var expiredCount int

	for _, match := range matches {
		// Get file size
		if info, err := os.Stat(match); err == nil {
			totalSize += info.Size()

			// Check if file is expired
			if data, err := os.ReadFile(match); err == nil {
				var cachedResponse CachedResponse
				if err := json.Unmarshal(data, &cachedResponse); err == nil {
					if cm.isCacheExpired(&cachedResponse) {
						expiredCount++
					}
				}
			}
		}
	}

	stats.TotalSize = totalSize
	stats.ExpiredFiles = expiredCount

	return stats, nil
}

// CleanExpired removes expired cache entries
func (cm *CacheManager) CleanExpired() error {
	if !cm.config.CacheEnabled {
		return nil
	}

	pattern := filepath.Join(cm.config.CacheDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find cache files: %w", err)
	}

	var removedCount int
	for _, match := range matches {
		// Read and check if expired
		if data, err := os.ReadFile(match); err == nil {
			var cachedResponse CachedResponse
			if err := json.Unmarshal(data, &cachedResponse); err == nil {
				if cm.isCacheExpired(&cachedResponse) {
					if err := os.Remove(match); err == nil {
						removedCount++
					}
				}
			}
		}
	}

	return nil
}

// CleanOldFiles removes cache files older than the specified duration
// This is useful for cleaning up files that may not have proper TTL metadata
func (cm *CacheManager) CleanOldFiles(maxAge time.Duration) error {
	if !cm.config.CacheEnabled {
		return nil
	}

	pattern := filepath.Join(cm.config.CacheDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find cache files: %w", err)
	}

	cutoffTime := time.Now().Add(-maxAge)
	var removedCount int

	for _, match := range matches {
		// Check file modification time
		if info, err := os.Stat(match); err == nil {
			if info.ModTime().Before(cutoffTime) {
				if err := os.Remove(match); err == nil {
					removedCount++
				}
			}
		}
	}

	return nil
}

// PerformMaintenance performs routine cache maintenance including:
// - Removing expired entries
// - Removing old files (older than maxAge)
func (cm *CacheManager) PerformMaintenance(maxAge time.Duration) error {
	if !cm.config.CacheEnabled {
		return nil
	}

	// Clean expired entries first
	if err := cm.CleanExpired(); err != nil {
		return fmt.Errorf("failed to clean expired entries: %w", err)
	}

	// Clean old files
	if err := cm.CleanOldFiles(maxAge); err != nil {
		return fmt.Errorf("failed to clean old files: %w", err)
	}

	return nil
}

// isCacheExpired checks if a cached response has expired
func (cm *CacheManager) isCacheExpired(cached *CachedResponse) bool {
	if cached.TTL <= 0 {
		return false // No expiration
	}

	expirationTime := cached.Timestamp.Add(cached.TTL)
	return time.Now().After(expirationTime)
}

// getDefaultTTL returns the default time-to-live for cache entries
func (cm *CacheManager) getDefaultTTL() time.Duration {
	// Default to 24 hours
	return 24 * time.Hour
}

// validateReferencedFiles checks if all files referenced in the cached response still exist
func (cm *CacheManager) validateReferencedFiles(response *DocumentProcessingResponse) bool {
	// Check if any images are referenced and validate they exist
	for _, image := range response.Images {
		if image.FilePath != "" {
			if _, err := os.Stat(image.FilePath); os.IsNotExist(err) {
				return false // Image file is missing
			}
		}
	}

	// Note: We don't validate markdown files here because:
	// 1. Inline responses don't generate markdown files
	// 2. The cache validation should focus on generated assets (images) that are referenced
	// 3. The markdown content is stored in the cache itself, not as separate files
	// 4. Users might legitimately delete markdown files while keeping images
	//
	// The main concern is ensuring that image files referenced in the response still exist,
	// as these are external dependencies that could be deleted independently.

	return true // All referenced files exist
}

// CachedResponse represents a cached document processing response
type CachedResponse struct {
	Response  DocumentProcessingResponse `json:"response"`
	CacheKey  string                     `json:"cache_key"`
	Timestamp time.Time                  `json:"timestamp"`
	TTL       time.Duration              `json:"ttl"` // Time to live
}

// CacheStats provides statistics about the cache
type CacheStats struct {
	Enabled      bool   `json:"enabled"`
	Directory    string `json:"directory"`
	TotalFiles   int    `json:"total_files"`
	TotalSize    int64  `json:"total_size"`    // Size in bytes
	ExpiredFiles int    `json:"expired_files"` // Number of expired files
}
