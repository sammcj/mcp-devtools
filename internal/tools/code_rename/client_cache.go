package code_rename

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
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

	// Start cleanup goroutine for this client (1 minute lifetime, no extensions)
	go func() {
		time.Sleep(1 * time.Minute)

		// Check if still in cache and expired
		if stillCached, ok := cache.Load(key); ok {
			if c, ok := stillCached.(*cachedLSPClient); ok {
				c.mu.Lock()
				defer c.mu.Unlock()

				// Only close if not used recently
				if time.Since(c.lastUsed) >= 1*time.Minute {
					logger.WithFields(logrus.Fields{
						"language": server.Language,
						"age":      time.Since(c.createdAt).Round(time.Second).String(),
					}).Debug("Closing expired LSP client")

					if c.client != nil {
						c.client.Close()
					}
					cache.Delete(key)
				}
				// Note: No lifetime extension to prevent goroutine accumulation
				// If client is still needed, a new one will be created on next use
			}
		}
	}()

	return client, nil
}
