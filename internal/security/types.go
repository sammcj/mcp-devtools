package security

import (
	"sync"
	"time"
)

// SecurityManager is the main security coordinator
type SecurityManager struct {
	enabled     bool
	advisor     *SecurityAdvisor
	denyChecker *DenyListChecker
	ruleEngine  *YAMLRuleEngine
	overrides   *OverrideManager
	cache       *Cache
	config      *SecurityConfig
	mutex       sync.RWMutex
}

// SecurityAdvisor provides threat analysis and security advice
type SecurityAdvisor struct {
	config     *SecurityConfig
	ruleEngine *YAMLRuleEngine
	analyser   *ThreatAnalyser
	trust      *SourceTrust
	cache      *Cache
	overrides  *OverrideManager
}

// ThreatAnalyser performs Intent-Context-Destination analysis
type ThreatAnalyser struct {
	patterns    map[string]PatternMatcher
	shellParser *ShellParser
}

// SourceTrust manages domain trust scoring and categorisation
type SourceTrust struct {
	trustedDomains    []string
	suspiciousDomains []string
	domainCategories  map[string]string
	mutex             sync.RWMutex
}

// YAMLRuleEngine manages YAML-based security rules
type YAMLRuleEngine struct {
	rules *SecurityRules
	// patterns     *PatternLibrary // TODO: Implement pattern library
	compiled     map[string]PatternMatcher
	rulesPath    string
	lastModified time.Time
	// watcher      *FileWatcher // TODO: Implement file watching
	mutex sync.RWMutex
}

// DenyListChecker enforces file and domain access controls
type DenyListChecker struct {
	filePatterns    []string
	domainPatterns  []string
	compiledFiles   []PatternMatcher
	compiledDomains []PatternMatcher
	mutex           sync.RWMutex
}

// OverrideManager handles security overrides and audit trail
type OverrideManager struct {
	overridesPath string
	logPath       string
	overrides     *OverrideConfig
	mutex         sync.RWMutex
}

// Cache provides in-memory security analysis caching
type Cache struct {
	data    sync.Map
	maxSize int
	maxAge  time.Duration
	size    int64
}

// CacheEntry represents a cached security analysis result
type CacheEntry struct {
	Result  *SecurityResult
	Created time.Time
}

// SecurityResult contains the outcome of security analysis
type SecurityResult struct {
	Safe      bool            `json:"safe"`
	Action    string          `json:"action"` // "allow", "warn", "block"
	Message   string          `json:"message"`
	ID        string          `json:"id"`
	Analysis  *ThreatAnalysis `json:"analysis,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// ThreatAnalysis contains detailed threat assessment
type ThreatAnalysis struct {
	Commands    []ParsedCommand `json:"commands"`
	SourceTrust float64         `json:"source_trust"`
	RiskScore   float64         `json:"risk_score"`
	Context     string          `json:"context"`
	RiskFactors []string        `json:"risk_factors"`
}

// ParsedCommand represents a detected shell command
type ParsedCommand struct {
	Raw         string            `json:"raw"`
	Executable  string            `json:"executable"`
	Arguments   []CommandArgument `json:"arguments"`
	Destination *Destination      `json:"destination,omitempty"`
	Pipes       []PipeOperation   `json:"pipes,omitempty"`
}

// CommandArgument represents a command argument with analysis
type CommandArgument struct {
	Value           string       `json:"value"`
	Type            ArgumentType `json:"type"`
	EntropyScore    float64      `json:"entropy_score"`
	ContainsSecrets bool         `json:"contains_secrets"`
	IsVariable      bool         `json:"is_variable"`
	TrustScore      float64      `json:"trust_score"`
}

// Destination represents a command's target destination
type Destination struct {
	URL             string              `json:"url"`
	Host            string              `json:"host"`
	IPAddress       string              `json:"ip_address,omitempty"`
	ReputationScore float64             `json:"reputation_score"`
	Category        DestinationCategory `json:"category"`
}

// PipeOperation represents a shell pipe operation
type PipeOperation struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	IsShell     bool   `json:"is_shell"`
	IsDangerous bool   `json:"is_dangerous"`
}

// SourceContext provides context about content source
type SourceContext struct {
	URL         string `json:"url"`
	Domain      string `json:"domain"`
	ContentType string `json:"content_type"`
	Tool        string `json:"tool"`
}

// SecurityConfig holds all security configuration
type SecurityConfig struct {
	Enabled                bool          `json:"enabled"`
	RulesPath              string        `json:"rules_path"`
	LogPath                string        `json:"log_path"`
	AutoReload             bool          `json:"auto_reload"`
	MaxScanSize            int           `json:"max_scan_size"`
	ThreatThreshold        float64       `json:"threat_threshold"`
	EnableDestinationCheck bool          `json:"enable_destination_check"`
	EnableSecretDetection  bool          `json:"enable_secret_detection"`
	CacheEnabled           bool          `json:"cache_enabled"`
	CacheMaxAge            time.Duration `json:"cache_max_age"`
	CacheMaxSize           int           `json:"cache_max_size"`
	EnableNotifications    bool          `json:"enable_notifications"`
	TrustedDomains         []string      `json:"trusted_domains"`
	SuspiciousDomains      []string      `json:"suspicious_domains"`
	DenyFiles              []string      `json:"deny_files"`
	DenyDomains            []string      `json:"deny_domains"`
}

// PatternMatcher interface for different pattern matching strategies
type PatternMatcher interface {
	Match(content string) bool
	String() string
}

// ArgumentType enum for command arguments
type ArgumentType string

const (
	ArgumentTypeURL      ArgumentType = "url"
	ArgumentTypeFile     ArgumentType = "file"
	ArgumentTypeFlag     ArgumentType = "flag"
	ArgumentTypeVariable ArgumentType = "variable"
	ArgumentTypeString   ArgumentType = "string"
)

// DestinationCategory enum for destination trust levels
type DestinationCategory string

const (
	DestinationOfficial   DestinationCategory = "official"
	DestinationCDN        DestinationCategory = "cdn"
	DestinationCommunity  DestinationCategory = "community"
	DestinationUnknown    DestinationCategory = "unknown"
	DestinationSuspicious DestinationCategory = "suspicious"
)

// Security actions
const (
	ActionAllow = "allow"
	ActionWarn  = "warn"
	ActionBlock = "block"
)

// ShellParser handles shell command parsing
type ShellParser struct {
	// Implementation will use google/shlex
}

// FileWatcher monitors rule file changes
type FileWatcher struct {
	// Implementation will use fsnotify
}

// PatternLibrary holds reusable patterns
type PatternLibrary struct {
	Patterns map[string]string `yaml:"patterns"`
}
