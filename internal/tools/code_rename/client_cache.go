package code_rename

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Global cleanup management
var (
	cleanupOnce sync.Once
	cleanupStop chan struct{}
)

// cachedLSPClient wraps an LSP client with metadata for caching
type cachedLSPClient struct {
	client        *LSPClient
	language      string
	workspaceRoot string
	createdAt     time.Time
	lastUsed      time.Time
	mu            sync.Mutex
}

// cacheKey generates a unique key for client caching based on language and workspace
func cacheKey(language, workspaceRoot string) string {
	return fmt.Sprintf("lsp_client:%s:%s", language, workspaceRoot)
}

// startCleanupRoutine starts a single background goroutine that periodically sweeps expired clients
// This prevents goroutine accumulation that would occur with per-client cleanup goroutines
func startCleanupRoutine(cache *sync.Map, logger *logrus.Logger) {
	cleanupOnce.Do(func() {
		cleanupStop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					// Sweep all cached clients and close expired ones
					cache.Range(func(key, value any) bool {
						if c, ok := value.(*cachedLSPClient); ok {
							c.mu.Lock()
							// Close clients that haven't been used in the last minute
							if time.Since(c.lastUsed) >= 1*time.Minute {
								logger.WithFields(logrus.Fields{
									"language": c.language,
									"idle":     time.Since(c.lastUsed).Round(time.Second).String(),
								}).Debug("Cleanup routine closing expired LSP client")

								if c.client != nil {
									c.client.Close()
								}
								cache.Delete(key)
							}
							c.mu.Unlock()
						}
						return true // Continue iteration
					})
				case <-cleanupStop:
					logger.Debug("LSP client cleanup routine stopping")
					return
				}
			}
		}()
		logger.Debug("Started LSP client cleanup routine (30s sweep interval)")
	})
}

// getOrCreateLSPClient retrieves a cached LSP client or creates a new one
// Clients are cached for a fixed 1 minute from creation (not extended on reuse) to improve performance for batch operations
func getOrCreateLSPClient(
	ctx context.Context,
	logger *logrus.Logger,
	cache *sync.Map,
	server *LanguageServer,
	filePath string,
) (*LSPClient, error) {
	// Determine workspace root for cache key
	rootPath, err := findWorkspaceRoot(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find workspace root: %w", err)
	}

	key := cacheKey(server.Language, rootPath)

	// Try to get from cache
	if cached, ok := cache.Load(key); ok {
		if cachedClient, ok := cached.(*cachedLSPClient); ok {
			cachedClient.mu.Lock()
			defer cachedClient.mu.Unlock()

			// Check if still valid (< 1 minute old)
			age := time.Since(cachedClient.createdAt)
			if age < 1*time.Minute {
				// Check if client connection is still alive
				if cachedClient.client != nil && cachedClient.client.conn != nil {
					cachedClient.lastUsed = time.Now()
					logger.WithFields(logrus.Fields{
						"language": server.Language,
						"age":      age.Round(time.Second).String(),
						"root":     rootPath,
					}).Debug("Reusing cached LSP client")

					return cachedClient.client, nil
				}
			}

			// Client expired or connection dead - remove from cache
			logger.WithField("language", server.Language).Debug("Cached LSP client expired or disconnected, creating new one")
			cache.Delete(key)
		}
	}

	// Create new client
	logger.WithFields(logrus.Fields{
		"language": server.Language,
		"root":     rootPath,
	}).Debug("Creating new LSP client")

	client, err := NewLSPClient(ctx, logger, server, filePath)
	if err != nil {
		return nil, err
	}

	// Cache it
	cached := &cachedLSPClient{
		client:        client,
		language:      server.Language,
		workspaceRoot: rootPath,
		createdAt:     time.Now(),
		lastUsed:      time.Now(),
	}
	cache.Store(key, cached)

	// Start the background cleanup routine (only once, shared across all clients)
	startCleanupRoutine(cache, logger)

	return client, nil
}
