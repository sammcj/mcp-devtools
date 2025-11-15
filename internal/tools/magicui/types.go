package magicui

import "time"

// ComponentInfo holds details for a Magic UI component
type ComponentInfo struct {
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies,omitempty"`
	Files        []File   `json:"files,omitempty"`
}

// File represents a component file from the registry
type File struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Target string `json:"target"`
}

// RegistryItem represents a single item in the Magic UI registry
type RegistryItem struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Files        []File   `json:"files,omitempty"`
}

// Registry represents the Magic UI registry structure
type Registry struct {
	Name     string         `json:"name"`
	Homepage string         `json:"homepage"`
	Items    []RegistryItem `json:"items"`
}

// CacheEntry represents a cached value with timestamp
type CacheEntry struct {
	Data      any
	Timestamp time.Time
}

// Constants for cache keys and TTLs
const (
	registryCacheKey = "magicui:registry"
	registryCacheTTL = 24 * time.Hour

	componentDetailsCachePrefix = "magicui:component:"
	componentDetailsCacheTTL    = 24 * time.Hour

	MagicUIRegistryURL = "https://raw.githubusercontent.com/magicuidesign/magicui/main/registry.json"
	MagicUIDocsURL     = "https://magicui.design"
	MagicUIGitHubURL   = "https://github.com/magicuidesign/magicui"
)
