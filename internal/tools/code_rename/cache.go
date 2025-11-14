package code_rename

import (
	"sync"
	"time"
)

var (
	serverCache     *ServerCache
	serverCacheOnce sync.Once
)

// ServerCache caches LSP server availability
type ServerCache struct {
	mu               sync.RWMutex
	availableServers map[string]cacheEntry
	cacheDuration    time.Duration
}

// cacheEntry stores availability and timestamp for a server
type cacheEntry struct {
	available bool
	timestamp time.Time
}

// NewServerCache creates a new server cache with 5-minute expiry
func NewServerCache() *ServerCache {
	return &ServerCache{
		availableServers: make(map[string]cacheEntry),
		cacheDuration:    5 * time.Minute,
	}
}

// GetServerCache returns the global server cache instance
func GetServerCache() *ServerCache {
	serverCacheOnce.Do(func() {
		serverCache = NewServerCache()
	})
	return serverCache
}

// IsAvailable checks if a language server is available (cached)
func (sc *ServerCache) IsAvailable(command string) (bool, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, exists := sc.availableServers[command]
	if !exists {
		return false, false
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > sc.cacheDuration {
		return false, false
	}

	return entry.available, true
}

// SetAvailable sets the availability of a language server
func (sc *ServerCache) SetAvailable(command string, available bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.availableServers[command] = cacheEntry{
		available: available,
		timestamp: time.Now(),
	}
}
