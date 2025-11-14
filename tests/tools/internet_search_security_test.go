package tools

import (
	"testing"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/unified"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternetSearchSecurityIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security integration test in short mode")
	}

	// Create test rules that block certain search content
	rules := &security.SecurityRules{
		Version: "1.0",
		Settings: security.Settings{
			Enabled:             true,
			MaxContentSize:      1024, // 1KB max content size
			MaxEntropySize:      64,   // 64KB max entropy size
			CaseSensitive:       false,
			EnableNotifications: false,
		},
		AccessControl: security.AccessControl{
			DenyDomains: []string{"malicious-site.example"},
			DenyFiles:   []string{},
		},
		Rules: map[string]security.Rule{
			"malicious_content": {
				Description: "Blocks dangerous content in search results",
				Patterns: []security.PatternConfig{
					{Contains: "dangerous malware download"},
				},
				Action: "block",
			},
			"suspicious_content": {
				Description: "Warns about suspicious content in search results",
				Patterns: []security.PatternConfig{
					{Contains: "suspicious link"},
				},
				Action: "warn",
			},
		},
		TrustedDomains: []string{"api.search.brave.com", "html.duckduckgo.com"},
	}

	// Initialise security manager with test rules
	securityManager, err := security.NewSecurityManagerWithRules(rules)
	require.NoError(t, err)
	t.Logf("Created security manager successfully")

	// Set global security manager
	security.GlobalSecurityManager = securityManager

	// Create internet search tool
	searchTool := &unified.InternetSearchTool{}

	t.Run("SecurityEnabledCheck", func(t *testing.T) {
		// Verify security is enabled
		assert.True(t, security.IsEnabled(), "Security should be enabled for testing")
	})

	t.Run("DomainAccessControl", func(t *testing.T) {
		// Test that blocked domains are rejected
		err := security.CheckDomainAccess("malicious-site.example")
		assert.Error(t, err, "Should block access to denied domain")
		assert.Contains(t, err.Error(), "access denied", "Error should mention access denial")
	})

	t.Run("TrustedDomainAccess", func(t *testing.T) {
		// Test that trusted domains are allowed
		err := security.CheckDomainAccess("api.search.brave.com")
		assert.NoError(t, err, "Should allow access to trusted domain")

		err = security.CheckDomainAccess("html.duckduckgo.com")
		assert.NoError(t, err, "Should allow access to DuckDuckGo")
	})

	t.Run("ContentAnalysis", func(t *testing.T) {
		// Test content analysis integration - the main goal is to verify that:
		// 1. Security analysis can be called without errors
		// 2. The SourceContext is properly structured
		// 3. The function returns valid results

		source := security.SourceContext{
			Tool:        "internet_search",
			Domain:      "test_provider",
			ContentType: "search_results",
		}

		// Test that content analysis works without errors (pattern matching logic is tested elsewhere)
		testContent := "This is a normal search result about programming languages and software development practices"
		result, err := security.AnalyseContent(testContent, source)
		require.NoError(t, err)
		assert.NotNil(t, result, "Should return a valid security result")
		assert.Contains(t, []string{security.ActionAllow, security.ActionWarn, security.ActionBlock}, result.Action, "Should return a valid action")
		t.Logf("Content analysis result: action=%s, safe=%v, message=%s", result.Action, result.Safe, result.Message)
	})

	t.Run("SearchToolDefinition", func(t *testing.T) {
		// Test that the tool definition is properly generated
		definition := searchTool.Definition()
		assert.Equal(t, "internet_search", definition.Name)
		assert.NotEmpty(t, definition.Description)
	})
}

func TestInternetSearchSecurityDisabled(t *testing.T) {
	// Test behaviour when security is disabled
	originalManager := security.GlobalSecurityManager
	defer func() {
		security.GlobalSecurityManager = originalManager
	}()

	// Disable security
	security.GlobalSecurityManager = nil

	assert.False(t, security.IsEnabled(), "Security should be disabled")

	// Domain access should be allowed when security is disabled
	err := security.CheckDomainAccess("any-domain.example")
	assert.NoError(t, err, "Should allow access when security is disabled")

	// Content analysis should return safe when security is disabled
	source := security.SourceContext{
		Tool:        "internet_search",
		Domain:      "test_provider",
		ContentType: "search_results",
	}
	result, err := security.AnalyseContent("any content", source)
	assert.NoError(t, err)
	assert.True(t, result.Safe, "Should be safe when security is disabled")
	assert.Equal(t, security.ActionAllow, result.Action, "Should allow when security is disabled")
}
