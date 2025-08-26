package security

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

//go:embed default_config.yaml
var defaultConfigTemplate string

// NewYAMLRuleEngine creates a new YAML rule engine
func NewYAMLRuleEngine(rulesPath string) (*YAMLRuleEngine, error) {
	logrus.WithField("rules_path", rulesPath).Debug("Creating YAML rule engine")
	engine := &YAMLRuleEngine{
		rulesPath: rulesPath,
		compiled:  make(map[string]PatternMatcher),
	}

	// Ensure rules file exists
	logrus.Debug("Ensuring security rules file exists")
	if err := engine.ensureRulesFile(); err != nil {
		return nil, fmt.Errorf("failed to ensure rules file: %w", err)
	}
	logrus.Debug("Security rules file exists")

	// Load initial rules
	logrus.Debug("Loading initial security rules")
	if err := engine.LoadRules(); err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}
	logrus.Debug("Security rules loaded successfully")

	// Start file watcher for auto-reload (non-blocking)
	if engine.rules.Settings.AutoReload {
		logrus.Debug("Starting file watcher for auto-reload (non-blocking)")
		go func() {
			if err := engine.startFileWatcher(); err != nil {
				logrus.WithError(err).Warn("Failed to start rule file watcher, auto-reload disabled")
			}
		}()
	} else {
		logrus.Debug("Auto-reload disabled, skipping file watcher")
	}

	logrus.Debug("YAML rule engine creation complete")
	return engine, nil
}

// ensureRulesFile creates default rules file if it doesn't exist
func (r *YAMLRuleEngine) ensureRulesFile() error {
	if _, err := os.Stat(r.rulesPath); os.IsNotExist(err) {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(r.rulesPath), 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Generate minimal default rules
		defaultRules := r.generateMinimalRules()

		// Write rules file
		if err := os.WriteFile(r.rulesPath, []byte(defaultRules), 0600); err != nil {
			return fmt.Errorf("failed to create default rules: %w", err)
		}

		// Only log if not in stdio mode (stdio mode sets ErrorLevel to prevent MCP protocol pollution)
		if logrus.GetLevel() >= logrus.InfoLevel {
			logrus.Infof("Created default security rules at %s", r.rulesPath)
		}
	} else {
		// Rules file exists, manage default configuration file
		if err := r.manageDefaultConfigFile(); err != nil {
			logrus.WithError(err).Warn("Failed to manage default configuration file")
		}
	}
	return nil
}

// generateMinimalRules creates the default minimal rules configuration
func (r *YAMLRuleEngine) generateMinimalRules() string {
	// Replace the template timestamp placeholder with current time
	config := strings.ReplaceAll(defaultConfigTemplate, "{{.Timestamp}}", time.Now().Format(time.RFC3339))
	return config
}

// manageDefaultConfigFile creates or updates security_default.yaml if user has custom security.yaml
func (r *YAMLRuleEngine) manageDefaultConfigFile() error {
	// Only manage default config if the main config exists and appears to be custom
	if _, err := os.Stat(r.rulesPath); os.IsNotExist(err) {
		return nil // Main config doesn't exist, nothing to do
	}

	// Read current user config
	userData, err := os.ReadFile(r.rulesPath)
	if err != nil {
		return fmt.Errorf("failed to read user config: %w", err)
	}

	// Generate current default rules
	defaultRules := r.generateMinimalRules()

	// Check if user config is different from default
	if string(userData) == defaultRules {
		return nil // User config is same as default, no need for separate default file
	}

	// Path for default config file
	defaultConfigPath := r.getDefaultConfigPath()

	// Check if default config already exists and is up to date
	if existingDefault, err := os.ReadFile(defaultConfigPath); err == nil {
		if string(existingDefault) == defaultRules {
			return nil // Default config already exists and is current
		}
	}

	// Write/update the default config file
	if err := os.WriteFile(defaultConfigPath, []byte(defaultRules), 0600); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	// Only log if not in stdio mode (stdio mode sets ErrorLevel to prevent MCP protocol pollution)
	if logrus.GetLevel() >= logrus.InfoLevel {
		logrus.Infof("Updated default security configuration at %s", defaultConfigPath)
	}
	return nil
}

// getDefaultConfigPath returns the path for the default configuration file
func (r *YAMLRuleEngine) getDefaultConfigPath() string {
	dir := filepath.Dir(r.rulesPath)
	return filepath.Join(dir, "security_default.yaml")
}

// LoadRules loads rules from the YAML file
func (r *YAMLRuleEngine) LoadRules() error {
	logrus.Debug("Acquiring rules mutex lock")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	logrus.Debug("Rules mutex acquired")

	// Read rules file
	logrus.WithField("rules_path", r.rulesPath).Debug("Reading security rules file")
	data, err := os.ReadFile(r.rulesPath)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}
	logrus.WithField("bytes", len(data)).Debug("Security rules file read successfully")

	// Parse YAML
	logrus.Debug("Parsing security rules YAML")
	var rules SecurityRules
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("failed to parse YAML rules: %w", err)
	}
	logrus.Debug("Security rules YAML parsed successfully")

	// Validate rules and auto-fix invalid regex patterns
	logrus.Debug("Validating and fixing security rules")
	modified, err := r.validateAndFixRules(&rules, string(data))
	if err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}
	logrus.WithField("modified", modified).Debug("Security rules validation completed")

	// If file was modified due to invalid regex, reload from the corrected file
	if modified {
		logrus.Debug("Security rules were modified, reloading corrected file")
		// Only log if not in stdio mode (stdio mode sets ErrorLevel to prevent MCP protocol pollution)
		if logrus.GetLevel() >= logrus.InfoLevel {
			logrus.Info("Security rules file was automatically corrected due to invalid regex patterns")
		}

		// Re-read the corrected file
		data, err = os.ReadFile(r.rulesPath)
		if err != nil {
			return fmt.Errorf("failed to re-read corrected rules file: %w", err)
		}

		// Re-parse the corrected YAML
		if err := yaml.Unmarshal(data, &rules); err != nil {
			return fmt.Errorf("failed to parse corrected YAML rules: %w", err)
		}

		// Re-validate (should pass now)
		if _, err := r.validateAndFixRules(&rules, string(data)); err != nil {
			return fmt.Errorf("corrected rule validation failed: %w", err)
		}
		logrus.Debug("Corrected security rules reloaded successfully")
	}

	// Compile patterns
	logrus.Debug("Compiling security rule patterns")
	if err := r.compilePatterns(&rules); err != nil {
		return fmt.Errorf("pattern compilation failed: %w", err)
	}
	logrus.Debug("Security rule patterns compiled successfully")

	// Update rule engine state
	logrus.Debug("Updating rule engine state")
	r.rules = &rules
	r.lastModified = time.Now()

	// Clear security cache when rules are reloaded to ensure new rules take effect immediately
	// Skip cache clearing during initial setup to avoid deadlock with globalManagerMutex
	logrus.Debug("Checking if cache clearing is safe")

	// Try to acquire the lock with a timeout to avoid deadlock during initialization
	done := make(chan bool, 1)
	var manager *SecurityManager

	go func() {
		globalManagerMutex.RLock()
		manager = GlobalSecurityManager
		globalManagerMutex.RUnlock()
		done <- true
	}()

	select {
	case <-done:
		logrus.Debug("Successfully checked global security manager")
		if manager != nil && manager.cache != nil {
			oldSize := manager.cache.Size()
			manager.cache.Clear()
			if oldSize > 0 {
				logrus.WithField("cache_entries_cleared", oldSize).Debug("Cleared security cache due to rule reload")
			}
			logrus.WithField("entries_cleared", oldSize).Debug("Security cache cleared")
		} else {
			logrus.Debug("No security cache to clear (manager not ready)")
		}
	case <-time.After(100 * time.Millisecond):
		// Timeout - likely during initialization, skip cache clearing
		logrus.Debug("Skipping cache clear due to mutex timeout (likely during initialization)")
	}

	logrus.Debug("Security rules loaded successfully")
	return nil
}

// validateAndFixRules validates rules and automatically fixes invalid regex patterns
func (r *YAMLRuleEngine) validateAndFixRules(rules *SecurityRules, originalContent string) (bool, error) {
	// First try standard validation
	if err := r.validateRules(rules); err == nil {
		return false, nil // No modifications needed
	}

	// If validation failed, try to fix invalid regex patterns
	logrus.Warn("Found invalid regex patterns in security rules, attempting auto-fix")

	lines := strings.Split(originalContent, "\n")
	modified := false
	var invalidPatterns []string

	// Check each rule for invalid regex patterns
	for ruleName, rule := range rules.Rules {
		for i, pattern := range rule.Patterns {
			if pattern.Regex != "" {
				if _, err := regexp.Compile(pattern.Regex); err != nil {
					invalidPatterns = append(invalidPatterns, fmt.Sprintf("rule %s pattern %d: %s", ruleName, i, err.Error()))

					// Find and comment out the problematic regex line in the YAML
					regexPattern := fmt.Sprintf(`regex:\s*["|']?%s["|']?`, regexp.QuoteMeta(pattern.Regex))
					for lineNum, line := range lines {
						if matched, _ := regexp.MatchString(regexPattern, line); matched {
							// Add comment above the problematic line
							commentLine := fmt.Sprintf("    # Rule automatically disabled due to invalid regex: %s", err.Error())

							// Comment out the problematic line
							commentedLine := fmt.Sprintf("    # %s", strings.TrimSpace(line))

							// Insert the comment and replace the line
							lines = insertLine(lines, lineNum, commentLine)
							lines[lineNum+1] = commentedLine
							modified = true
							break
						}
					}
				}
			}
		}
	}

	if modified {
		// Write the corrected file
		correctedContent := strings.Join(lines, "\n")
		if err := os.WriteFile(r.rulesPath, []byte(correctedContent), 0600); err != nil {
			return false, fmt.Errorf("failed to write corrected rules file: %w", err)
		}

		logrus.WithField("invalid_patterns", invalidPatterns).Warn("Automatically commented out invalid regex patterns in security rules")
		return true, nil
	}

	// If we couldn't fix the validation errors, return the original error
	return false, r.validateRules(rules)
}

// insertLine inserts a new line at the specified index
func insertLine(lines []string, index int, newLine string) []string {
	if index < 0 || index > len(lines) {
		return lines
	}

	// Create a new slice with one more element
	result := make([]string, len(lines)+1)

	// Copy elements before the insertion point
	copy(result[:index], lines[:index])

	// Insert the new line
	result[index] = newLine

	// Copy remaining elements
	copy(result[index+1:], lines[index:])

	return result
}

// validateRules validates the loaded rules for correctness
func (r *YAMLRuleEngine) validateRules(rules *SecurityRules) error {
	// Validate version
	if rules.Version == "" {
		return fmt.Errorf("rules version is required")
	}

	// Validate each rule
	for name, rule := range rules.Rules {
		if len(rule.Patterns) == 0 {
			return fmt.Errorf("rule %s has no patterns", name)
		}

		// Validate action
		switch rule.Action {
		case "allow", "warn", "warn_high", "block", "notify", "ignore":
			// Valid actions
		default:
			return fmt.Errorf("rule %s has invalid action: %s", name, rule.Action)
		}

		// Validate patterns
		for i, pattern := range rule.Patterns {
			if err := r.validatePattern(pattern, name, i); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePattern validates a single pattern configuration
func (r *YAMLRuleEngine) validatePattern(pattern PatternConfig, ruleName string, patternIndex int) error {
	// Count non-empty pattern fields
	count := 0
	if pattern.Literal != "" {
		count++
	}
	if pattern.Contains != "" {
		count++
	}
	if pattern.StartsWith != "" {
		count++
	}
	if pattern.EndsWith != "" {
		count++
	}
	if pattern.FilePath != "" {
		count++
	}
	if pattern.URL != "" {
		count++
	}
	if pattern.Entropy > 0 {
		count++
	}
	if pattern.Regex != "" {
		count++
	}
	if pattern.Glob != "" {
		count++
	}

	if count == 0 {
		return fmt.Errorf("rule %s pattern %d has no match criteria", ruleName, patternIndex)
	}
	if count > 1 {
		return fmt.Errorf("rule %s pattern %d has multiple match criteria (only one allowed)", ruleName, patternIndex)
	}

	// Validate regex if present
	if pattern.Regex != "" {
		if _, err := regexp.Compile(pattern.Regex); err != nil {
			return fmt.Errorf("rule %s pattern %d has invalid regex: %w", ruleName, patternIndex, err)
		}
	}

	// Validate entropy threshold
	if pattern.Entropy > 0 && (pattern.Entropy < 1.0 || pattern.Entropy > 8.0) {
		return fmt.Errorf("rule %s pattern %d has invalid entropy threshold (must be 1.0-8.0)", ruleName, patternIndex)
	}

	return nil
}

// compilePatterns compiles all patterns for efficient matching
func (r *YAMLRuleEngine) compilePatterns(rules *SecurityRules) error {
	r.compiled = make(map[string]PatternMatcher)

	// Compile rule patterns
	for ruleName, rule := range rules.Rules {
		for i, patternConfig := range rule.Patterns {
			matcher, err := r.createPatternMatcher(patternConfig, rules)
			if err != nil {
				return fmt.Errorf("failed to compile rule %s pattern %d: %w", ruleName, i, err)
			}

			key := fmt.Sprintf("%s_%d", ruleName, i)
			r.compiled[key] = matcher
		}
	}

	return nil
}

// createPatternMatcher creates a pattern matcher from configuration
func (r *YAMLRuleEngine) createPatternMatcher(config PatternConfig, rules *SecurityRules) (PatternMatcher, error) {
	switch {
	case config.Literal != "":
		return NewLiteralMatcher(config.Literal), nil
	case config.Contains != "":
		return NewContainsMatcher(config.Contains), nil
	case config.StartsWith != "":
		return NewPrefixMatcher(config.StartsWith), nil
	case config.EndsWith != "":
		return NewSuffixMatcher(config.EndsWith), nil
	case config.FilePath != "":
		// Redirect file_path to enhanced contains matcher
		return NewContainsMatcher(config.FilePath), nil
	case config.URL != "":
		return NewURLMatcher(config.URL), nil
	case config.Entropy > 0:
		maxSizeKB := 64 // Default 64KB
		if rules.Settings.MaxEntropySize > 0 {
			maxSizeKB = rules.Settings.MaxEntropySize
		}
		maxSizeBytes := maxSizeKB * 1024 // Convert KB to bytes
		return NewEntropyMatcherWithMaxSize(config.Entropy, maxSizeBytes), nil
	case config.Regex != "":
		return NewRegexMatcher(config.Regex)
	case config.Glob != "":
		return NewGlobMatcher(config.Glob), nil
	default:
		return nil, fmt.Errorf("no valid pattern configuration found")
	}
}

// startFileWatcher starts watching the rules file for changes
func (r *YAMLRuleEngine) startFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Use a channel to handle watcher.Add with timeout
	done := make(chan error, 1)
	go func() {
		done <- watcher.Add(r.rulesPath)
	}()

	// Wait for watcher.Add to complete with timeout
	select {
	case err := <-done:
		if err != nil {
			if closeErr := watcher.Close(); closeErr != nil {
				logrus.WithError(closeErr).Warn("Failed to close watcher after add error")
			}
			return fmt.Errorf("failed to watch rules file: %w", err)
		}
	case <-time.After(5 * time.Second):
		if closeErr := watcher.Close(); closeErr != nil {
			logrus.WithError(closeErr).Warn("Failed to close watcher after timeout")
		}
		return fmt.Errorf("timeout adding rules file to watcher")
	}

	go func() {
		defer func() {
			if closeErr := watcher.Close(); closeErr != nil {
				// Log error but don't fail the operation
				_ = closeErr
			}
		}()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					logrus.Debug("Security rules file changed, reloading")
					if err := r.LoadRules(); err != nil {
						logrus.WithError(err).Error("Failed to reload security rules")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logrus.WithError(err).Error("Security rules file watcher error")
			}
		}
	}()

	return nil
}

// EvaluateContent evaluates content against all rules
func (r *YAMLRuleEngine) EvaluateContent(content string, source SourceContext) (*SecurityResult, error) {
	return r.EvaluateContentWithConfig(content, source, nil)
}

// EvaluateContentWithConfig evaluates content against all rules with optional config for base64 processing
func (r *YAMLRuleEngine) EvaluateContentWithConfig(content string, source SourceContext, config *SecurityConfig) (*SecurityResult, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.rules == nil {
		return &SecurityResult{Safe: true, Action: ActionAllow}, nil
	}

	// Check for size limit violations and handle according to size_exceeded_behaviour
	sizeCheckResult := r.checkSizeLimits(content, source)
	if sizeCheckResult != nil {
		return sizeCheckResult, nil
	}

	// Apply content size limits before evaluation (for "allow" behavior)
	evaluationContent := r.applyContentSizeLimits(content)

	// Check rules in priority order: allow/ignore first, then others
	// This ensures allowlist patterns can override deny/warn rules
	rulesByPriority := r.sortRulesByPriority()

	for _, ruleInfo := range rulesByPriority {
		matched, err := r.evaluateRuleWithConfig(ruleInfo.Name, ruleInfo.Rule, evaluationContent, source, config)
		if err != nil {
			logrus.WithError(err).Warnf("Error evaluating rule %s", ruleInfo.Name)
			continue
		}

		if matched {
			// Generate security result
			securityID := GenerateSecurityID(ruleInfo.Rule.Action)

			return &SecurityResult{
				Safe:      ruleInfo.Rule.Action == "allow" || ruleInfo.Rule.Action == "ignore",
				Action:    mapRuleActionToSecurityAction(ruleInfo.Rule.Action),
				Message:   r.formatSecurityMessage(ruleInfo.Rule, ruleInfo.Name, securityID),
				ID:        securityID,
				Timestamp: time.Now(),
			}, nil
		}
	}

	// No rules matched, content is safe
	return &SecurityResult{Safe: true, Action: ActionAllow}, nil
}

// RuleInfo holds rule information for priority-based processing
type RuleInfo struct {
	Name     string
	Rule     Rule
	Priority int
}

// sortRulesByPriority sorts rules by priority to ensure allowlist patterns are evaluated first
func (r *YAMLRuleEngine) sortRulesByPriority() []RuleInfo {
	var ruleInfos []RuleInfo

	// Convert map to slice and assign priorities
	for ruleName, rule := range r.rules.Rules {
		priority := r.getActionPriority(rule.Action)
		ruleInfos = append(ruleInfos, RuleInfo{
			Name:     ruleName,
			Rule:     rule,
			Priority: priority,
		})
	}

	// Sort by priority (lower numbers = higher priority = evaluated first)
	sort.Slice(ruleInfos, func(i, j int) bool {
		// First sort by priority
		if ruleInfos[i].Priority != ruleInfos[j].Priority {
			return ruleInfos[i].Priority < ruleInfos[j].Priority
		}
		// Then sort by name for consistent ordering
		return ruleInfos[i].Name < ruleInfos[j].Name
	})

	return ruleInfos
}

// getActionPriority returns the priority level for different rule actions
// Lower numbers = higher priority = evaluated first
func (r *YAMLRuleEngine) getActionPriority(action string) int {
	switch action {
	case "allow":
		return 1 // Highest priority - allowlist patterns override everything
	case "ignore":
		return 2 // Second highest - also overrides warnings/blocks
	case "block":
		return 3 // High priority - blocks override warnings
	case "warn_high":
		return 4 // Medium-high priority
	case "warn":
		return 5 // Medium priority
	case "notify":
		return 6 // Low priority - just notifications
	default:
		return 7 // Lowest priority - unknown actions
	}
}

// checkSizeLimits checks content against size limits and returns appropriate security result
func (r *YAMLRuleEngine) checkSizeLimits(content string, source SourceContext) *SecurityResult {
	if r.rules == nil {
		return nil // No rules configured
	}

	contentSizeBytes := len(content)
	behaviour := strings.ToLower(r.rules.Settings.SizeExceededBehaviour)
	if behaviour == "" {
		behaviour = "allow" // Default to allow for backward compatibility
	}

	// Check max_content_size (for content analysis)
	if r.rules.Settings.MaxContentSize > 0 {
		maxContentBytes := r.rules.Settings.MaxContentSize * 1024
		if contentSizeBytes > maxContentBytes {
			return r.handleSizeLimitExceeded(contentSizeBytes, r.rules.Settings.MaxContentSize, "max_content_size", behaviour, source)
		}
	}

	// Check max_scan_size (overall content size limit)
	if r.rules.Settings.MaxScanSize > 0 {
		maxScanBytes := r.rules.Settings.MaxScanSize * 1024
		if contentSizeBytes > maxScanBytes {
			return r.handleSizeLimitExceeded(contentSizeBytes, r.rules.Settings.MaxScanSize, "max_scan_size", behaviour, source)
		}
	}

	return nil // No size limits exceeded
}

// handleSizeLimitExceeded handles the configured behavior when size limits are exceeded
func (r *YAMLRuleEngine) handleSizeLimitExceeded(contentSize, limitKB int, limitType, behaviour string, source SourceContext) *SecurityResult {
	switch behaviour {
	case "block":
		securityID := GenerateSecurityID("block")
		return &SecurityResult{
			Safe:      false,
			Action:    ActionBlock,
			Message:   fmt.Sprintf("Content size (%d bytes) exceeds %s limit (%d KB). Use security_override tool with ID %s if this is intentional.", contentSize, limitType, limitKB, securityID),
			ID:        securityID,
			Timestamp: time.Now(),
		}
	case "warn":
		securityID := GenerateSecurityID("warn")
		logrus.WithFields(logrus.Fields{
			"content_size_bytes": contentSize,
			"limit_kb":           limitKB,
			"limit_type":         limitType,
			"security_id":        securityID,
			"source":             source.URL,
		}).Warn("Content size exceeds configured limit")
		return &SecurityResult{
			Safe:      true, // Allow processing but with warning
			Action:    ActionWarn,
			Message:   fmt.Sprintf("Content size (%d bytes) exceeds %s limit (%d KB) but processing continued. Use security_override tool with ID %s if this is intentional.", contentSize, limitType, limitKB, securityID),
			ID:        securityID,
			Timestamp: time.Now(),
		}
	case "allow":
		// Log the truncation but allow processing
		logrus.WithFields(logrus.Fields{
			"content_size_bytes": contentSize,
			"limit_kb":           limitKB,
			"limit_type":         limitType,
		}).Debug("Content size exceeds limit but configured to allow")
		return nil // Continue with normal processing
	default:
		logrus.Warnf("Invalid size_exceeded_behaviour: %s, defaulting to 'allow'", behaviour)
		return nil // Default to allow
	}
}

// applyContentSizeLimits applies size limits to content before evaluation
func (r *YAMLRuleEngine) applyContentSizeLimits(content string) string {
	if r.rules == nil || r.rules.Settings.MaxContentSize <= 0 {
		return content // No limits configured
	}

	maxSizeKB := r.rules.Settings.MaxContentSize
	maxSize := maxSizeKB * 1024 // Convert KB to bytes
	if len(content) <= maxSize {
		return content // Content is within limits
	}

	// Truncate content but try to preserve structure
	truncated := content[:maxSize]

	// Try to break at a sensible boundary (newline, space) near the limit
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxSize-1000 {
		truncated = content[:lastNewline]
	} else if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxSize-100 {
		truncated = content[:lastSpace]
	}

	logrus.WithFields(logrus.Fields{
		"original_size_bytes":  len(content),
		"truncated_size_bytes": len(truncated),
		"max_size_kb":          maxSizeKB,
		"max_size_bytes":       maxSize,
	}).Debug("Content truncated for security analysis")

	return truncated
}

// evaluateRuleWithConfig evaluates a single rule against content with optional config for base64 processing
func (r *YAMLRuleEngine) evaluateRuleWithConfig(ruleName string, rule Rule, content string, source SourceContext, config *SecurityConfig) (bool, error) {
	// Check exceptions first
	if r.isSourceExcepted(source, rule.Exceptions) {
		return false, nil
	}

	// Logic defaults to "any" if not specified
	logic := rule.Logic
	if logic == "" {
		logic = "any"
	}

	// Check if base64 decode_base64 is enabled for this rule
	decodeAndScan := false
	if rule.Options != nil {
		if val, exists := rule.Options["decode_base64"]; exists {
			if boolVal, ok := val.(bool); ok {
				decodeAndScan = boolVal
			}
		}
	}

	// Process content for pattern matching
	contentToMatch := content
	if decodeAndScan && config != nil && config.EnableBase64Scanning {
		// Detect and decode base64 content, append to original content
		decodedContent := r.detectAndDecodeBase64ContentWithConfig(content, config)
		if decodedContent != "" {
			contentToMatch = content + "\n" + decodedContent
		}
	}

	matchCount := 0
	for i := range rule.Patterns {
		key := fmt.Sprintf("%s_%d", ruleName, i)
		matcher, exists := r.compiled[key]
		if !exists {
			continue
		}

		if matcher.Match(contentToMatch) {
			matchCount++
			if logic == "any" {
				return true, nil // First match is enough for "any" logic
			}
		}
	}

	// For "all" logic, all patterns must match
	return logic == "all" && matchCount == len(rule.Patterns), nil
}

// isSourceExcepted checks if source is in exception list
func (r *YAMLRuleEngine) isSourceExcepted(source SourceContext, exceptions []string) bool {
	for _, exception := range exceptions {
		// Check against trusted domains
		if exception == "trusted_domains" {
			for _, domain := range r.rules.TrustedDomains {
				if r.domainMatches(source.Domain, domain) {
					return true
				}
			}
		}
	}
	return false
}

// domainMatches checks if domain matches pattern (supports wildcards)
func (r *YAMLRuleEngine) domainMatches(domain, pattern string) bool {
	if after, ok := strings.CutPrefix(pattern, "*."); ok {
		baseDomain := after
		return domain == baseDomain || strings.HasSuffix(domain, "."+baseDomain)
	}
	return domain == pattern
}

// mapRuleActionToSecurityAction maps rule actions to security actions
func mapRuleActionToSecurityAction(ruleAction string) string {
	switch ruleAction {
	case "allow", "ignore":
		return ActionAllow
	case "warn", "warn_high", "notify":
		return ActionWarn
	case "block":
		return ActionBlock
	default:
		return ActionWarn
	}
}

// formatSecurityMessage creates a user-friendly security message
func (r *YAMLRuleEngine) formatSecurityMessage(rule Rule, ruleName, securityID string) string {
	action := mapRuleActionToSecurityAction(rule.Action)

	switch action {
	case ActionBlock:
		return fmt.Sprintf("Security Block [ID: %s]: %s. Check with the user if you may use security_override tool with ID %s and justification.", securityID, rule.Description, securityID)
	case ActionWarn:
		return fmt.Sprintf("Security Warning [ID: %s]: %s. Use security_override tool with ID %s if this is intentional.", securityID, rule.Description, securityID)
	default:
		return fmt.Sprintf("Security Notice [ID: %s]: %s", securityID, rule.Description)
	}
}

// GenerateSecurityID generates a unique security event ID
func GenerateSecurityID(action string) string {
	timestamp := time.Now().Unix()
	randomSuffix := generateRandomString(6)
	return fmt.Sprintf("sec_%s_%d_%s", action, timestamp, randomSuffix)
}

// generateRandomString generates a random string for security IDs
func generateRandomString(length int) string {
	// Simple random string generation
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

// GenerateDefaultConfig generates the default security configuration
func GenerateDefaultConfig() string {
	tempEngine := &YAMLRuleEngine{}
	return tempEngine.generateMinimalRules()
}

// ValidateSecurityConfig validates a security configuration
func ValidateSecurityConfig(configData []byte) (*SecurityRules, error) {
	var rules SecurityRules
	if err := yaml.Unmarshal(configData, &rules); err != nil {
		return nil, fmt.Errorf("YAML parsing failed: %w", err)
	}

	// Create temporary engine for validation
	tempEngine := &YAMLRuleEngine{}

	// Validate the rules structure
	if err := tempEngine.validateRules(&rules); err != nil {
		return nil, fmt.Errorf("rules validation failed: %w", err)
	}

	// Validate individual patterns
	for ruleName, rule := range rules.Rules {
		for i, pattern := range rule.Patterns {
			if err := tempEngine.validatePattern(pattern, ruleName, i); err != nil {
				return nil, fmt.Errorf("rule '%s' pattern %d validation failed: %w", ruleName, i, err)
			}
		}
	}

	return &rules, nil
}

// detectAndDecodeBase64ContentWithConfig detects and decodes base64 content with provided config
func (r *YAMLRuleEngine) detectAndDecodeBase64ContentWithConfig(content string, config *SecurityConfig) string {
	if config == nil || !config.EnableBase64Scanning {
		return ""
	}

	var decodedParts []string
	lines := strings.Split(content, "\n")
	maxSize := config.MaxBase64DecodedSize * 1024 // Convert KB to bytes

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 16 { // Skip very short lines
			continue
		}

		// Check if line looks like base64
		if isLikelyBase64(line) {
			// Attempt to decode recursively (up to 3 levels to prevent infinite loops)
			decodedContent := r.recursiveBase64Decode(line, maxSize, 3)
			if decodedContent != "" {
				decodedParts = append(decodedParts, fmt.Sprintf("Line %d decoded: %s", lineNum+1, decodedContent))
			}
		}
	}

	return strings.Join(decodedParts, "\n")
}

// recursiveBase64Decode attempts to decode base64 content recursively to handle nested encoding
func (r *YAMLRuleEngine) recursiveBase64Decode(content string, maxSize int, maxDepth int) string {
	if maxDepth <= 0 {
		return ""
	}

	var allDecoded []string

	// Try to decode the current content
	if decoded, success := safeBase64Decode(content, maxSize); success && len(decoded) > 0 {
		decodedStr := string(decoded)
		allDecoded = append(allDecoded, decodedStr)

		// Check if the decoded content itself contains base64
		lines := strings.SplitSeq(decodedStr, "\n")
		for line := range lines {
			line = strings.TrimSpace(line)

			// Check if entire line is base64
			if len(line) >= 16 && isLikelyBase64(line) {
				nested := r.recursiveBase64Decode(line, maxSize, maxDepth-1)
				if nested != "" {
					allDecoded = append(allDecoded, nested)
				}
			} else {
				// Look for base64 patterns within the line
				r.extractAndDecodeEmbeddedBase64(line, maxSize, maxDepth-1, &allDecoded)
			}
		}
	}

	return strings.Join(allDecoded, "\n")
}

// extractAndDecodeEmbeddedBase64 finds base64 strings embedded within text and decodes them
func (r *YAMLRuleEngine) extractAndDecodeEmbeddedBase64(line string, maxSize int, maxDepth int, allDecoded *[]string) {
	// Look for base64 patterns: sequences of 16+ chars that are mostly base64 characters
	// This handles cases like: echo "base64string" | base64 -d
	words := strings.FieldsSeq(line)

	for word := range words {
		// Remove common surrounding characters
		cleaned := strings.Trim(word, `"'()[]{}`)

		if len(cleaned) >= 16 && isLikelyBase64(cleaned) {
			nested := r.recursiveBase64Decode(cleaned, maxSize, maxDepth)
			if nested != "" {
				*allDecoded = append(*allDecoded, nested)
			}
		}
	}
}
