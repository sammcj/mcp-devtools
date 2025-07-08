package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateFile represents the cached state for mcp-devtools
type StateFile struct {
	// Python configuration state
	PythonPath       string `json:"python_path,omitempty"`
	DoclingAvailable bool   `json:"docling_available"`
	LastChecked      int64  `json:"last_checked,omitempty"` // Unix timestamp

	// Other cached state can be added here in the future
	mu sync.RWMutex `json:"-"`
}

var (
	globalState *StateFile
	stateOnce   sync.Once
)

// GetGlobalState returns the singleton global state
func GetGlobalState() *StateFile {
	stateOnce.Do(func() {
		globalState = loadGlobalState()
	})
	return globalState
}

// loadGlobalState loads the global state from disk
func loadGlobalState() *StateFile {
	state := &StateFile{}

	statePath := getStatePath()
	if data, err := os.ReadFile(statePath); err == nil {
		// Ignore JSON parsing errors and use defaults
		_ = json.Unmarshal(data, state)
	}

	return state
}

// Save saves the global state to disk
func (s *StateFile) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	statePath := getStatePath()

	// Ensure state directory exists
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// SetPythonPath sets the Python path and saves the state
func (s *StateFile) SetPythonPath(path string, doclingAvailable bool) error {
	s.mu.Lock()
	s.PythonPath = path
	s.DoclingAvailable = doclingAvailable
	s.LastChecked = getCurrentTimestamp()
	s.mu.Unlock()

	return s.Save()
}

// GetPythonPath returns the cached Python path
func (s *StateFile) GetPythonPath() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PythonPath, s.DoclingAvailable
}

// IsStale checks if the cached state is stale (older than 24 hours)
func (s *StateFile) IsStale() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.LastChecked == 0 {
		return true
	}

	// Consider stale if older than 24 hours
	return getCurrentTimestamp()-s.LastChecked > 24*60*60
}

// getStatePath returns the path to the global state file
func getStatePath() string {
	// Check for custom state path from environment
	if customPath := os.Getenv("MCP_DEVTOOLS_STATE_PATH"); customPath != "" {
		return customPath
	}

	// Default to ~/.mcp-devtools/state.json
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".mcp-devtools", "state.json")
}

// getCurrentTimestamp returns the current Unix timestamp
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
