package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sammcj/mcp-devtools/internal/telemetry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Global security manager instance
var (
	GlobalSecurityManager  *SecurityManager
	globalManagerMutex     sync.RWMutex
	securityTracingEnabled bool
)

func init() {
	// Cache security tracing enabled check at startup
	securityTracingEnabled = os.Getenv("MCP_TRACING_SECURITY_ENABLED") == "true"
}

// NewSecurityManager creates a new security manager instance
// NewSecurityManagerWithRules creates a security manager with provided rules (for testing)
func NewSecurityManagerWithRules(rules *SecurityRules) (*SecurityManager, error) {
	// Create test config
	config := &SecurityConfig{
		Enabled:      rules.Settings.Enabled,
		RulesPath:    ":memory:",
		LogPath:      ":memory:",
		CacheMaxSize: 1000,
		CacheMaxAge:  1 * time.Hour,
	}

	// Create cache
	cache := &Cache{
		maxSize: config.CacheMaxSize,
		maxAge:  config.CacheMaxAge,
	}

	// Create rule engine with provided rules
	ruleEngine := &YAMLRuleEngine{
		rules:     rules,
		compiled:  make(map[string]PatternMatcher),
		rulesPath: ":memory:",
		mutex:     sync.RWMutex{},
	}

	// Compile patterns
	if err := ruleEngine.compilePatterns(rules); err != nil {
		return nil, fmt.Errorf("failed to compile patterns: %w", err)
	}

	// Create override manager with temporary paths
	overrideManager, err := NewOverrideManager(os.TempDir()+"/test_overrides.yaml", os.TempDir()+"/test_security.log")
	if err != nil {
		return nil, fmt.Errorf("failed to create override manager: %w", err)
	}

	// Create source trust manager
	sourceTrust := &SourceTrust{
		trustedDomains:   rules.TrustedDomains,
		domainCategories: make(map[string]string),
	}

	// Create threat analyser
	threatAnalyser := &ThreatAnalyser{
		patterns: make(map[string]PatternMatcher),
	}

	// Create deny list checker
	denyChecker := &DenyListChecker{
		filePatterns:   rules.AccessControl.DenyFiles,
		domainPatterns: rules.AccessControl.DenyDomains,
	}
	if err := denyChecker.compilePatterns(); err != nil {
		return nil, fmt.Errorf("failed to compile deny patterns: %w", err)
	}

	// Create security advisor
	advisor := &SecurityAdvisor{
		config:     config,
		ruleEngine: ruleEngine,
		analyser:   threatAnalyser,
		trust:      sourceTrust,
		cache:      cache,
		overrides:  overrideManager,
	}

	return &SecurityManager{
		enabled:     rules.Settings.Enabled,
		advisor:     advisor,
		denyChecker: denyChecker,
		ruleEngine:  ruleEngine,
		overrides:   overrideManager,
		cache:       cache,
		config:      config,
		mutex:       sync.RWMutex{},
	}, nil
}

func NewSecurityManager() (*SecurityManager, error) {
	logrus.Debug("Loading security configuration")
	config, err := loadSecurityConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load security config: %w", err)
	}
	logrus.Debug("Security configuration loaded successfully")

	// Create cache
	logrus.Debug("Creating security cache")
	cache := &Cache{
		maxSize: config.CacheMaxSize,
		maxAge:  config.CacheMaxAge,
	}

	// Create rule engine
	logrus.WithField("rules_path", config.RulesPath).Debug("Creating YAML rule engine")
	ruleEngine, err := NewYAMLRuleEngine(config.RulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create rule engine: %w", err)
	}
	logrus.Debug("YAML rule engine created successfully")

	// Create override manager
	logrus.Debug("Creating override manager")
	overrideManager, err := NewOverrideManager(
		filepath.Join(filepath.Dir(config.RulesPath), "overrides.yaml"),
		config.LogPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create override manager: %w", err)
	}
	logrus.Debug("Override manager created successfully")

	// Create source trust manager
	logrus.Debug("Creating source trust manager")
	sourceTrust := &SourceTrust{
		trustedDomains:    config.TrustedDomains,
		suspiciousDomains: config.SuspiciousDomains,
		domainCategories:  make(map[string]string),
	}

	// Create threat analyser
	logrus.Debug("Creating threat analyser")
	threatAnalyser := &ThreatAnalyser{
		patterns: make(map[string]PatternMatcher),
	}

	// Create security advisor
	logrus.Debug("Creating security advisor")
	advisor := &SecurityAdvisor{
		config:     config,
		ruleEngine: ruleEngine,
		analyser:   threatAnalyser,
		trust:      sourceTrust,
		cache:      cache,
		overrides:  overrideManager,
	}

	// Create deny list checker
	logrus.Debug("Creating deny list checker")
	denyChecker := &DenyListChecker{
		filePatterns:   config.DenyFiles,
		domainPatterns: config.DenyDomains,
	}
	if err := denyChecker.compilePatterns(); err != nil {
		return nil, fmt.Errorf("failed to compile deny patterns: %w", err)
	}
	logrus.Debug("Deny list checker created successfully")

	logrus.Debug("Assembling security manager")
	manager := &SecurityManager{
		enabled:     config.Enabled,
		advisor:     advisor,
		denyChecker: denyChecker,
		ruleEngine:  ruleEngine,
		overrides:   overrideManager,
		cache:       cache,
		config:      config,
	}

	// Start cleanup routine if caching is enabled
	if config.CacheEnabled {
		logrus.Debug("Starting cache cleanup routine")
		cache.StartCleanup()
	}

	logrus.Debug("Security manager creation complete")
	return manager, nil
}

// IsEnabled returns whether the security system is enabled
func (m *SecurityManager) IsEnabled() bool {
	if m == nil {
		return false
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.enabled
}

// CheckFileAccess verifies if file access is allowed
func (m *SecurityManager) CheckFileAccess(filePath string) error {
	if !m.IsEnabled() {
		return nil
	}

	if m.denyChecker.IsFileBlocked(filePath) {
		LogAccessControlBlock("file_access_denied", filePath, "filesystem")
		return fmt.Errorf("access denied: %s is in deny list (sensitive credential file). This is an access control policy that cannot be overridden by agents. The user may change this behaviour in their MCP DevTools configuration if required", filePath)
	}

	return nil
}

// CheckDomainAccess verifies if domain access is allowed
func (m *SecurityManager) CheckDomainAccess(domain string) error {
	if !m.IsEnabled() {
		return nil
	}

	if m.denyChecker.IsDomainBlocked(domain) {
		LogAccessControlBlock("domain_access_denied", domain, "webfetch")
		return fmt.Errorf("access denied: %s is in domain deny list. This is an access control policy that cannot be overridden by agents. The user may change this behaviour in their MCP DevTools configuration if required", domain)
	}

	return nil
}

// AnalyseContentWithContext performs security analysis on content with tracing support
func (m *SecurityManager) AnalyseContentWithContext(ctx context.Context, content string, source SourceContext) (*SecurityResult, error) {
	// Only create span if enabled (cached at init)
	if !securityTracingEnabled {
		// Security tracing not enabled, use standard method
		return m.AnalyseContent(content, source)
	}

	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, telemetry.SpanNameSecurityCheck,
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	// Add span attributes
	span.SetAttributes(
		attribute.Int(telemetry.AttrSecurityContentSize, len(content)),
	)

	// Perform the analysis
	result, err := m.AnalyseContent(content, source)

	// Add result attributes to span
	if result != nil {
		span.SetAttributes(
			attribute.String(telemetry.AttrSecurityAction, string(result.Action)),
		)
		if result.Analysis != nil && len(result.Analysis.RiskFactors) > 0 {
			// Get first risk factor as the matched rule
			span.SetAttributes(
				attribute.String(telemetry.AttrSecurityRuleMatched, result.Analysis.RiskFactors[0]),
			)
		}
	}

	return result, err
}

// AnalyseContent performs security analysis on content
func (m *SecurityManager) AnalyseContent(content string, source SourceContext) (*SecurityResult, error) {
	if !m.IsEnabled() {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithFields(logrus.Fields{
				"content_length": len(content),
				"source_domain":  source.Domain,
				"source_tool":    source.Tool,
			}).Debug("Security manager disabled, allowing content without analysis")
		}
		return &SecurityResult{Safe: true, Action: ActionAllow}, nil
	}

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"content_length":  len(content),
			"source_domain":   source.Domain,
			"source_url":      source.URL,
			"source_tool":     source.Tool,
			"content_type":    source.ContentType,
			"content_preview": content[:min(100, len(content))],
		}).Debug("Starting security analysis of content")
	}

	result, err := m.advisor.AnalyseContent(content, source)

	if logrus.GetLevel() <= logrus.DebugLevel {
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error":          err.Error(),
				"content_length": len(content),
				"source_domain":  source.Domain,
			}).Debug("Security analysis failed with error")
		} else {
			fields := logrus.Fields{
				"result_safe":    result.Safe,
				"result_action":  result.Action,
				"result_message": result.Message,
				"result_id":      result.ID,
			}

			if result.Analysis != nil {
				fields["risk_score"] = result.Analysis.RiskScore
				fields["source_trust"] = result.Analysis.SourceTrust
				fields["commands_detected"] = len(result.Analysis.Commands)
				fields["risk_factors"] = len(result.Analysis.RiskFactors)
			} else {
				fields["risk_score"] = 0.0
				fields["source_trust"] = 0.0
				fields["commands_detected"] = 0
				fields["risk_factors"] = 0
			}

			logrus.WithFields(fields).Debug("Security analysis completed")
		}
	}

	return result, err
}

// loadSecurityConfig loads configuration from YAML rules file
func loadSecurityConfig() (*SecurityConfig, error) {
	// Get user config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".mcp-devtools")
	rulesPath := filepath.Join(configDir, "security.yaml")

	// Override rules path from environment if specified (only this is configurable via env)
	if envRulesPath := os.Getenv("MCP_SECURITY_RULES_PATH"); envRulesPath != "" {
		rulesPath = expandPath(envRulesPath)
	}

	// Load the rules file to get configuration
	ruleEngine, err := NewYAMLRuleEngine(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load rules for config: %w", err)
	}

	rules := ruleEngine.rules
	settings := rules.Settings

	// Determine log path: use config value or default
	logPath := settings.LogPath
	if logPath == "" {
		logPath = filepath.Join(configDir, "logs", "security.log")
	} else {
		logPath = expandPath(logPath)
	}

	// Parse cache max age from string
	cacheMaxAge := time.Hour
	if settings.CacheMaxAge != "" {
		if parsedAge, err := time.ParseDuration(settings.CacheMaxAge); err == nil {
			cacheMaxAge = parsedAge
		}
	}

	config := &SecurityConfig{
		Enabled:                settings.Enabled,
		RulesPath:              rulesPath,
		LogPath:                logPath,
		AutoReload:             settings.AutoReload,
		MaxScanSize:            settings.MaxScanSize,
		ThreatThreshold:        settings.ThreatThreshold,
		EnableDestinationCheck: true, // Keep this enabled by default
		EnableSecretDetection:  true, // Keep this enabled by default
		CacheEnabled:           settings.CacheEnabled,
		CacheMaxAge:            cacheMaxAge,
		CacheMaxSize:           settings.CacheMaxSize,
		EnableNotifications:    settings.EnableNotifications,
		EnableBase64Scanning:   settings.EnableBase64Scanning,
		MaxBase64DecodedSize:   settings.MaxBase64DecodedSize,
		TrustedDomains:         rules.TrustedDomains,
		SuspiciousDomains:      []string{}, // Not configurable via YAML currently
		DenyFiles:              rules.AccessControl.DenyFiles,
		DenyDomains:            rules.AccessControl.DenyDomains,
	}

	return config, nil
}

// InitGlobalSecurityManager initialises the global security manager
func InitGlobalSecurityManager() error {
	logrus.Debug("InitGlobalSecurityManager called")

	globalManagerMutex.Lock()
	defer globalManagerMutex.Unlock()

	// Check if already initialised to avoid double initialisation
	if GlobalSecurityManager != nil {
		logrus.Debug("Security system already initialised, skipping")
		return nil
	}

	// Only initialise if security is enabled via ENABLE_ADDITIONAL_TOOLS
	securityEnabled := tools.IsToolEnabled("security")
	logrus.WithField("enabled", securityEnabled).Debug("Checking security tool enablement")

	if !securityEnabled {
		logrus.Debug("Security system not enabled in ENABLE_ADDITIONAL_TOOLS")
		return nil
	}

	logrus.Debug("Creating new security manager")
	manager, err := NewSecurityManager()
	if err != nil {
		logrus.WithError(err).Debug("Failed to create security manager")
		logrus.WithError(err).Warn("Failed to initialise security manager, continuing without security")
		return nil // Don't fail startup
	}

	logrus.Debug("Security manager created successfully")
	GlobalSecurityManager = manager
	// Only log if not in stdio mode (stdio mode sets ErrorLevel to prevent MCP protocol pollution)
	if logrus.GetLevel() >= logrus.InfoLevel {
		logrus.Info("Security system initialised successfully")
	}
	return nil
}

// Global convenience functions for easy integration

// IsEnabled returns whether the global security system is enabled
func IsEnabled() bool {
	globalManagerMutex.RLock()
	defer globalManagerMutex.RUnlock()
	return GlobalSecurityManager != nil && GlobalSecurityManager.IsEnabled()
}

// CheckFileAccess checks file access via global manager
func CheckFileAccess(filePath string) error {
	globalManagerMutex.RLock()
	manager := GlobalSecurityManager
	globalManagerMutex.RUnlock()

	if manager == nil {
		return nil
	}
	return manager.CheckFileAccess(filePath)
}

// CheckDomainAccess checks domain access via global manager
func CheckDomainAccess(domain string) error {
	globalManagerMutex.RLock()
	manager := GlobalSecurityManager
	globalManagerMutex.RUnlock()

	if manager == nil {
		return nil
	}
	return manager.CheckDomainAccess(domain)
}

// AnalyseContent analyses content via global manager
func AnalyseContent(content string, source SourceContext) (*SecurityResult, error) {
	globalManagerMutex.RLock()
	manager := GlobalSecurityManager
	globalManagerMutex.RUnlock()

	if manager == nil {
		return &SecurityResult{Safe: true, Action: ActionAllow}, nil
	}
	return manager.AnalyseContent(content, source)
}

// Utility functions for environment variable parsing

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// GetOverrideManager returns the override manager for the security system
func (m *SecurityManager) GetOverrideManager() *OverrideManager {
	if m == nil {
		return nil
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.overrides
}

// LogAccessControlBlock logs access control blocks
func LogAccessControlBlock(eventType, source, tool string) {
	// This will be implemented when we add logging
	logrus.WithFields(logrus.Fields{
		"event_type": eventType,
		"source":     source,
		"tool":       tool,
	}).Info("Access control block")
}

// HandleSecurityWarning provides standardised security warning handling across all tools
// It logs the warning and returns a formatted security notice string for inclusion in responses
func HandleSecurityWarning(result *SecurityResult, logger *logrus.Logger) string {
	if result == nil || result.Action != ActionWarn {
		return ""
	}

	// Log the security warning
	if logger != nil {
		logger.WithField("security_id", result.ID).Warn(result.Message)
	}

	// Return formatted security notice for inclusion in user responses
	return fmt.Sprintf("⚠️  Security Warning [ID: %s]: %s", result.ID, result.Message)
}
