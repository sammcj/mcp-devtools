package shadcnui

import (
	"time"
)

const (
	// listComponentsCacheKey is the cache key for the list of all shadcn ui components.
	listComponentsCacheKey = "shadcnui:list_components"
	// listComponentsCacheTTL is the TTL for the component list cache.
	listComponentsCacheTTL = 24 * time.Hour

	// getComponentDetailsCachePrefix is the cache key prefix for component details.
	getComponentDetailsCachePrefix = "shadcnui:get_details:"
	// getComponentDetailsCacheTTL is the TTL for component details cache.
	getComponentDetailsCacheTTL = 24 * time.Hour

	// getComponentExamplesCachePrefix is the cache key prefix for component examples.
	getComponentExamplesCachePrefix = "shadcnui:get_examples:"
	// getComponentExamplesCacheTTL is the TTL for component examples cache.
	getComponentExamplesCacheTTL = 24 * time.Hour
)

// CacheEntry defines the structure for cached data.
// Moved here to be shared across shadcnui tools.
type CacheEntry struct {
	Data      any
	Timestamp time.Time
}

// newToolResultJSON helper is removed from here.
// Shadcnui tools will use packageversions.NewToolResultJSON instead.
