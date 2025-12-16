//go:build cgo && (darwin || (linux && amd64))

package filetracker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// TrackerFileName is the name of the file tracking data file
	TrackerFileName = "file_tracker.json"
)

// FileInfo stores indexing metadata for a file
type FileInfo struct {
	MtimeAtIndex time.Time `json:"mtime_at_index"`
	IndexedAt    time.Time `json:"indexed_at"`
}

// Tracker tracks file modification times to detect stale indexed files
type Tracker struct {
	files       map[string]FileInfo
	lastIndexed time.Time // When index was last updated
	filePath    string
	logger      *logrus.Logger
	mu          sync.RWMutex
}

// trackerData is the JSON persistence format
type trackerData struct {
	Files       map[string]FileInfo `json:"files"`
	LastIndexed time.Time           `json:"last_indexed"`
}

// NewTracker creates a new file tracker
func NewTracker(storePath string, logger *logrus.Logger) (*Tracker, error) {
	trackerPath := filepath.Join(storePath, TrackerFileName)

	t := &Tracker{
		files:    make(map[string]FileInfo),
		filePath: trackerPath,
		logger:   logger,
	}

	// Load existing tracker data
	if err := t.load(); err != nil {
		logger.WithError(err).Debug("No existing file tracker data, starting fresh")
	}

	return t, nil
}

// MarkIndexed records that a file was indexed with its current mtime
func (t *Tracker) MarkIndexed(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.files[path] = FileInfo{
		MtimeAtIndex: info.ModTime(),
		IndexedAt:    time.Now(),
	}

	return nil
}

// MarkIndexedBatch records multiple files as indexed
func (t *Tracker) MarkIndexedBatch(paths []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.logger.WithError(err).WithField("path", path).Debug("Failed to stat file for tracking")
			continue
		}

		t.files[path] = FileInfo{
			MtimeAtIndex: info.ModTime(),
			IndexedAt:    now,
		}
	}

	// Update global last indexed timestamp
	t.lastIndexed = now

	return nil
}

// GetLastIndexed returns when the index was last updated
func (t *Tracker) GetLastIndexed() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastIndexed
}

// GetStaleFiles returns files that have been modified since indexing
// and whose modifications are older than the staleness threshold.
// This prevents reindexing files that are actively being edited.
func (t *Tracker) GetStaleFiles(stalenessThreshold time.Duration) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	var staleFiles []string

	for path, info := range t.files {
		currentInfo, err := os.Stat(path)
		if err != nil {
			// File was deleted or inaccessible - mark as stale for cleanup
			if os.IsNotExist(err) {
				staleFiles = append(staleFiles, path)
			}
			continue
		}

		currentMtime := currentInfo.ModTime()

		// Check if file was modified since indexing
		if !currentMtime.Equal(info.MtimeAtIndex) {
			// Check if the modification is old enough (not actively being edited)
			timeSinceModification := now.Sub(currentMtime)
			if timeSinceModification >= stalenessThreshold {
				staleFiles = append(staleFiles, path)
			}
		}
	}

	return staleFiles
}

// IsIndexed checks if a file is tracked
func (t *Tracker) IsIndexed(path string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.files[path]
	return exists
}

// RemoveFile removes a file from tracking
func (t *Tracker) RemoveFile(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.files, path)
}

// RemoveFiles removes multiple files from tracking
func (t *Tracker) RemoveFiles(paths []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, path := range paths {
		delete(t.files, path)
	}
}

// Clear removes all tracked files
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.files = make(map[string]FileInfo)
}

// Count returns the number of tracked files
func (t *Tracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.files)
}

// TrackedPaths returns all tracked file paths
func (t *Tracker) TrackedPaths() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	paths := make([]string, 0, len(t.files))
	for path := range t.files {
		paths = append(paths, path)
	}
	return paths
}

// Save persists the tracker data to disk
func (t *Tracker) Save() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	td := trackerData{
		Files:       t.files,
		LastIndexed: t.lastIndexed,
	}

	data, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(t.filePath, data, 0600)
}

// load reads tracker data from disk
func (t *Tracker) load() error {
	data, err := os.ReadFile(t.filePath)
	if err != nil {
		return err
	}

	// Try new format first
	var td trackerData
	if err := json.Unmarshal(data, &td); err == nil && td.Files != nil {
		t.files = td.Files
		t.lastIndexed = td.LastIndexed
		return nil
	}

	// Fall back to old format (just the files map) for backwards compatibility
	return json.Unmarshal(data, &t.files)
}
