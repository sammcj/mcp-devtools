package security

import (
	"crypto/sha1"
	"fmt"
	"sync/atomic"
	"time"
)

// StartCleanup starts the periodic cache cleanup routine
func (c *Cache) StartCleanup() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			c.cleanup()
		}
	}()
}

// Get retrieves a cached security result
func (c *Cache) Get(key string) (*SecurityResult, bool) {
	if val, exists := c.data.Load(key); exists {
		entry := val.(*CacheEntry)

		// Simple TTL check
		if time.Since(entry.Created) > c.maxAge {
			c.data.Delete(key)
			atomic.AddInt64(&c.size, -1)
			return nil, false
		}

		return entry.Result, true
	}
	return nil, false
}

// Set stores a security result in the cache
func (c *Cache) Set(key string, result *SecurityResult) {
	// Simple size limit - if over limit, don't cache (fail open)
	if atomic.LoadInt64(&c.size) >= int64(c.maxSize) {
		return // Skip caching when full - simpler than complex eviction
	}

	entry := &CacheEntry{
		Result:  result,
		Created: time.Now(),
	}

	// Only increment if this is a new key
	if _, loaded := c.data.LoadOrStore(key, entry); !loaded {
		atomic.AddInt64(&c.size, 1)
	}
}

// cleanup removes expired entries from the cache
func (c *Cache) cleanup() {
	now := time.Now()
	c.data.Range(func(key, value any) bool {
		entry := value.(*CacheEntry)
		if now.Sub(entry.Created) > c.maxAge {
			c.data.Delete(key)
			atomic.AddInt64(&c.size, -1)
		}
		return true
	})
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.data.Range(func(key, value any) bool {
		c.data.Delete(key)
		return true
	})
	atomic.StoreInt64(&c.size, 0)
}

// Size returns the current number of cached entries
func (c *Cache) Size() int {
	return int(atomic.LoadInt64(&c.size))
}

// GenerateCacheKey generates a cache key from content and source
func GenerateCacheKey(content string, sourceURL string) string {
	hasher := sha1.New()
	hasher.Write([]byte(content))
	hasher.Write([]byte(sourceURL))
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16] // Short hash
}

// GetWithGeneration retrieves or generates a cached result
func (c *Cache) GetWithGeneration(content string, source SourceContext, generator func() (*SecurityResult, error)) (*SecurityResult, error) {
	// Generate cache key
	key := GenerateCacheKey(content, source.URL)

	// Try to get from cache first
	if result, found := c.Get(key); found {
		return result, nil
	}

	// Generate new result
	result, err := generator()
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.Set(key, result)

	return result, nil
}
