package security

import "time"

// SecurityRules represents the complete YAML rule configuration
type SecurityRules struct {
	Version        string          `yaml:"version"`
	Metadata       RuleMetadata    `yaml:"metadata"`
	Settings       Settings        `yaml:"settings"`
	TrustedDomains []string        `yaml:"trusted_domains"`
	AccessControl  AccessControl   `yaml:"access_control"`
	Rules          map[string]Rule `yaml:"rules"`
	AdvancedRules  map[string]Rule `yaml:"advanced_rules,omitempty"`
}

// RuleMetadata contains rule file metadata
type RuleMetadata struct {
	Description string `yaml:"description"`
	Created     string `yaml:"created"`
	Note        string `yaml:"note"`
}

// Settings contains global rule settings
type Settings struct {
	Enabled               bool    `yaml:"enabled"`
	DefaultAction         string  `yaml:"default_action"`
	AutoReload            bool    `yaml:"auto_reload"`
	CaseSensitive         bool    `yaml:"case_sensitive"`
	EnableNotifications   bool    `yaml:"enable_notifications"`
	MaxContentSize        int     `yaml:"max_content_size"`        // Maximum content size to scan (KB)
	MaxEntropySize        int     `yaml:"max_entropy_size"`        // Maximum content size for entropy analysis (KB)
	SizeExceededBehaviour string  `yaml:"size_exceeded_behaviour"` // Behaviour when size limits exceeded: "allow", "warn", "block"
	LogPath               string  `yaml:"log_path"`                // Custom log file path
	MaxScanSize           int     `yaml:"max_scan_size"`           // Maximum content size to scan (KB)
	ThreatThreshold       float64 `yaml:"threat_threshold"`        // Threat detection threshold
	CacheEnabled          bool    `yaml:"cache_enabled"`           // Enable security result caching
	CacheMaxAge           string  `yaml:"cache_max_age"`           // Maximum cache age (duration string)
	CacheMaxSize          int     `yaml:"cache_max_size"`          // Maximum cache entries
	EnableBase64Scanning  bool    `yaml:"enable_base64_scanning"`  // Enable base64 content decoding and analysis
	MaxBase64DecodedSize  int     `yaml:"max_base64_decoded_size"` // Maximum size of decoded base64 content (KB)
}

// AccessControl defines file and domain access restrictions
type AccessControl struct {
	DenyFiles   []string `yaml:"deny_files"`
	DenyDomains []string `yaml:"deny_domains"`
}

// Rule represents a security rule with patterns and actions
type Rule struct {
	Description string          `yaml:"description"`
	Patterns    []PatternConfig `yaml:"patterns"`
	Action      string          `yaml:"action"` // "block", "warn_high", "warn", "notify", "ignore"
	Severity    string          `yaml:"severity,omitempty"`
	Exceptions  []string        `yaml:"exceptions,omitempty"`
	Logic       string          `yaml:"logic,omitempty"` // "any" or "all"
	Options     map[string]any  `yaml:"options,omitempty"`
}

// PatternConfig represents different types of pattern matching
type PatternConfig struct {
	// Simple patterns (no escaping needed)
	Literal    string `yaml:"literal,omitempty"`     // Exact match
	Contains   string `yaml:"contains,omitempty"`    // Contains substring
	StartsWith string `yaml:"starts_with,omitempty"` // Prefix match
	EndsWith   string `yaml:"ends_with,omitempty"`   // Suffix match

	// Special semantic patterns
	FilePath string  `yaml:"file_path,omitempty"` // File path patterns
	URL      string  `yaml:"url,omitempty"`       // URL patterns
	Entropy  float64 `yaml:"entropy,omitempty"`   // Entropy threshold

	// Advanced patterns
	Regex string `yaml:"regex,omitempty"` // Raw regex
	Glob  string `yaml:"glob,omitempty"`  // Glob patterns
}

// OverrideConfig represents the override configuration file
type OverrideConfig struct {
	Version   string                      `yaml:"version"`
	Metadata  OverrideMetadata            `yaml:"metadata"`
	Overrides map[string]SecurityOverride `yaml:"overrides"`
	Allowlist AllowlistPatterns           `yaml:"allowlist_patterns"`
}

// OverrideMetadata contains override file metadata
type OverrideMetadata struct {
	Description string `yaml:"description"`
	Note        string `yaml:"note"`
}

// SecurityOverride represents a security override decision
type SecurityOverride struct {
	Type            string    `yaml:"type"`   // "warn", "block", etc.
	Action          string    `yaml:"action"` // "bypass", "allowlist"
	Justification   string    `yaml:"justification"`
	CreatedAt       time.Time `yaml:"created_at"`
	CreatedBy       string    `yaml:"created_by"`
	OriginalPattern string    `yaml:"original_pattern"`
	OriginalSource  string    `yaml:"original_source"`
}

// AllowlistPatterns contains patterns that are permanently allowed
type AllowlistPatterns struct {
	FilePaths []string `yaml:"file_paths"`
	Domains   []string `yaml:"domains"`
	Commands  []string `yaml:"commands"`
}

// SecurityLogEntry represents a logged security event
type SecurityLogEntry struct {
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	Tool      string          `json:"tool"`
	Source    string          `json:"source"`
	Type      string          `json:"type"`
	Action    string          `json:"action"`
	Analysis  *ThreatAnalysis `json:"analysis"`
}
