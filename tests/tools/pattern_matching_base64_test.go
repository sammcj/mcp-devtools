package tools

import (
	"encoding/base64"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiteralMatcherWithBase64Content(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		content     string
		shouldMatch bool
	}{
		{
			name:        "literal pattern matches in combined base64 + decoded content",
			pattern:     "rm -rf /",
			content:     "c3VkbyBybSAtcmYgLw== sudo rm -rf /",
			shouldMatch: true,
		},
		{
			name:        "test pattern matches in combined base64 + decoded content",
			pattern:     "BASE64_TEST_PATTERN_DETECTED",
			content:     "QkFTRTY0X1RFU1RfUEFUVEVSTl9ERVRFQ1RFRAo= BASE64_TEST_PATTERN_DETECTED",
			shouldMatch: true,
		},
		{
			name:        "pattern only in base64 part should not match literal matcher",
			pattern:     "rm -rf /",
			content:     "c3VkbyBybSAtcmYgLw==",
			shouldMatch: false,
		},
		{
			name:        "pattern only in decoded part should match",
			pattern:     "rm -rf /",
			content:     "some text rm -rf / more text",
			shouldMatch: true,
		},
		{
			name:        "case sensitive - uppercase pattern should not match lowercase content",
			pattern:     "RM -RF /",
			content:     "base64content rm -rf / moretext",
			shouldMatch: false,
		},
		{
			name:        "pattern with spaces should match",
			pattern:     "rm -rf /",
			content:     "prefix  rm -rf /  suffix",
			shouldMatch: true,
		},
		{
			name:        "partial pattern should not match",
			pattern:     "rm -rf /home",
			content:     "content rm -rf / other",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := security.NewLiteralMatcher(tt.pattern)
			result := matcher.Match(tt.content)
			assert.Equal(t, tt.shouldMatch, result,
				"Pattern '%s' in content '%s' should match: %t",
				tt.pattern, tt.content, tt.shouldMatch)
		})
	}
}

func TestSecurityAdvisorBase64Detection(t *testing.T) {
	// Create a security manager with base64 scanning enabled
	rules := &security.SecurityRules{
		Settings: security.Settings{
			Enabled:              true,
			MaxContentSize:       10240,
			MaxEntropySize:       1024,
			CaseSensitive:        false,
			EnableNotifications:  false,
			EnableBase64Scanning: true,
			MaxBase64DecodedSize: 1024,
		},
		AccessControl: security.AccessControl{
			DenyFiles:   []string{},
			DenyDomains: []string{},
		},
		Rules: map[string]security.Rule{
			"base64_test": {
				Description: "Test base64 pattern detection",
				Patterns: []security.PatternConfig{
					{Literal: "rm -rf /"},
					{Literal: "BASE64_TEST_PATTERN_DETECTED"},
				},
				Action: "warn",
			},
		},
		TrustedDomains: []string{},
	}

	manager, err := security.NewSecurityManagerWithRules(rules)
	require.NoError(t, err)

	tests := []struct {
		name             string
		originalText     string
		expectedInResult string
		description      string
	}{
		{
			name:             "dangerous command in base64",
			originalText:     "sudo rm -rf /",
			expectedInResult: "rm -rf /",
			description:      "Should detect dangerous command when base64 encoded",
		},
		{
			name:             "test pattern in base64",
			originalText:     "BASE64_TEST_PATTERN_DETECTED",
			expectedInResult: "BASE64_TEST_PATTERN_DETECTED",
			description:      "Should detect test pattern when base64 encoded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode the original text as base64
			encoded := base64.StdEncoding.EncodeToString([]byte(tt.originalText))

			// Add padding to ensure content is > 50 characters (required for analysis)
			// This simulates real-world usage where base64 content appears in larger responses
			paddedContent := "Response contains suspicious base64 content: " + encoded + " - investigate"

			// Analyse the padded content containing base64
			source := security.SourceContext{
				Tool:        "test",
				Domain:      "test_domain",
				ContentType: "base64_test",
			}
			result, err := manager.AnalyseContent(paddedContent, source)
			require.NoError(t, err)

			// The analysis should detect the pattern in the decoded content
			// The base64 content should be decoded and patterns matched against the decoded version
			assert.False(t, result.Safe, "Analysis should flag content containing dangerous base64 as unsafe")

			// Log the result for verification
			t.Logf("Test %s: Action=%s, Safe=%v, Message=%s", tt.name, result.Action, result.Safe, result.Message)
			t.Logf("  Original: %s", tt.originalText)
			t.Logf("  Base64: %s", encoded)
			t.Logf("  Padded content: %s", paddedContent)
		})
	}
}

func TestLiteralMatcherStringMethod(t *testing.T) {
	pattern := "rm -rf /"
	matcher := security.NewLiteralMatcher(pattern)
	expected := "literal:" + pattern
	assert.Equal(t, expected, matcher.String())
}

func TestBase64PatternMatchingIntegration(t *testing.T) {
	// Integration test that combines base64 detection with pattern matching
	// This simulates what would happen during actual security analysis

	rules := &security.SecurityRules{
		Settings: security.Settings{
			Enabled:              true,
			MaxContentSize:       10240,
			MaxEntropySize:       1024,
			CaseSensitive:        false,
			EnableNotifications:  false,
			EnableBase64Scanning: true,
			MaxBase64DecodedSize: 1024,
		},
		AccessControl: security.AccessControl{
			DenyFiles:   []string{},
			DenyDomains: []string{},
		},
		Rules: map[string]security.Rule{
			"dangerous_commands": {
				Description: "Test dangerous command detection",
				Patterns: []security.PatternConfig{
					{Literal: "rm -rf /"},
				},
				Action: "warn",
			},
		},
		TrustedDomains: []string{},
	}

	manager, err := security.NewSecurityManagerWithRules(rules)
	require.NoError(t, err)

	// Test content with base64 encoded dangerous commands
	dangerousCommand := "sudo rm -rf /"
	encodedDangerous := base64.StdEncoding.EncodeToString([]byte(dangerousCommand))

	// Create content > 50 characters to ensure analysis is triggered
	testContent := "Analysis response from server contains suspicious base64: " + encodedDangerous + " - please review carefully"

	// Analyse the content - this should decode the base64 and detect the pattern
	source := security.SourceContext{
		Tool:        "test",
		Domain:      "test_integration",
		ContentType: "mixed_content",
	}
	result, err := manager.AnalyseContent(testContent, source)
	require.NoError(t, err)

	// The analysis should detect the dangerous pattern in the decoded content
	assert.False(t, result.Safe,
		"Security analysis should flag content as unsafe when it contains 'rm -rf /' in base64")

	// Verify the analysis detected the threat
	t.Logf("Integration test result: Action=%s, Safe=%v, Message=%s", result.Action, result.Safe, result.Message)
	t.Logf("  Test content: %s", testContent)
	t.Logf("  Encoded dangerous command: %s", encodedDangerous)
	t.Logf("  Original command: %s", dangerousCommand)
}
