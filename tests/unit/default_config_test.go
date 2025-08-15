package unit

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/security"
	"gopkg.in/yaml.v3"
)

// TestDefaultConfigValidation ensures the embedded default configuration is always valid
func TestDefaultConfigValidation(t *testing.T) {
	// Generate the default config
	defaultConfig := security.GenerateDefaultConfig()

	// Validate it
	_, err := security.ValidateSecurityConfig([]byte(defaultConfig))
	if err != nil {
		t.Fatalf("Default security configuration is invalid: %v", err)
	}
}

// TestDefaultConfigNotEmpty ensures the default config is not empty
func TestDefaultConfigNotEmpty(t *testing.T) {
	defaultConfig := security.GenerateDefaultConfig()

	if len(defaultConfig) == 0 {
		t.Fatal("Default configuration is empty")
	}

	// Should contain basic sections
	requiredSections := []string{
		"version:",
		"settings:",
		"access_control:",
		"trusted_domains:",
		"rules:",
	}

	for _, section := range requiredSections {
		if !strings.Contains(defaultConfig, section) {
			t.Fatalf("Default config missing required section: %s", section)
		}
	}
}

// TestDefaultConfigRegexPatterns ensures all regex patterns in default config are valid
func TestDefaultConfigRegexPatterns(t *testing.T) {
	// Generate and parse the default config
	defaultConfig := security.GenerateDefaultConfig()

	var rules security.SecurityRules
	err := yaml.Unmarshal([]byte(defaultConfig), &rules)
	if err != nil {
		t.Fatalf("Failed to parse default config: %v", err)
	}

	// Check all regex patterns
	for ruleName, rule := range rules.Rules {
		for i, pattern := range rule.Patterns {
			if pattern.Regex != "" {
				_, err := regexp.Compile(pattern.Regex)
				if err != nil {
					t.Fatalf("Invalid regex in rule %s pattern %d: %s - Error: %v",
						ruleName, i, pattern.Regex, err)
				}
			}
		}
	}
}
