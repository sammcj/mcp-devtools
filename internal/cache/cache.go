package cache

import (
	"sync"
	"time"
)

// Cache provides a simple in-memory cache with expiration
type Cache struct {
	data  map[string]interface{}
	times map[string]time.Time
	ttl   time.Duration
	mu    sync.RWMutex
}

// NewCache creates a new cache with the specified TTL
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		data:  make(map[string]interface{}),
		times: make(map[string]time.Time),
		ttl:   ttl,
	}
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, exists := c.data[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(c.times[key]) > c.ttl {
		return nil, false
	}

	return val, true
}

// Set stores a value in the cache
func (c *Cache) Set(key string, val interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = val
	c.times[key] = time.Now()
}
