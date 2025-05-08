package shadcnui

import (
	"time"
)

const (
	// listComponentsCacheKey is the cache key for the list of all shadcn/ui components.
	listComponentsCacheKey = "shadcnui:list_components"
	// listComponentsCacheTTL is the TTL for the component list cache.
	listComponentsCacheTTL = 24 * time.Hour
)

// CacheEntry defines the structure for cached data.
// Moved here to be shared across shadcnui tools.
type CacheEntry struct {
	Data      interface{}
	Timestamp time.Time
}

// newToolResultJSON helper is removed from here.
// Shadcnui tools will use packageversions.NewToolResultJSON instead.
