package tools

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// mockResponse stores response data for the mock client
type mockResponse struct {
	statusCode int
	body       string
}

// MockHTTPClientForCooldown implements packageversions.HTTPClient for testing
type MockHTTPClientForCooldown struct {
	responses map[string]mockResponse
	err       error
}

func NewMockHTTPClientForCooldown() *MockHTTPClientForCooldown {
	return &MockHTTPClientForCooldown{
		responses: make(map[string]mockResponse),
	}
}

func (m *MockHTTPClientForCooldown) WithResponse(url string, statusCode int, body string) *MockHTTPClientForCooldown {
	m.responses[url] = mockResponse{
		statusCode: statusCode,
		body:       body,
	}
	return m
}

func (m *MockHTTPClientForCooldown) WithOSVResponse(hasVulns bool) *MockHTTPClientForCooldown {
	body := `{"vulns": []}`
	if hasVulns {
		body = `{"vulns": [{"id": "GHSA-1234", "summary": "Test vulnerability"}]}`
	}
	return m.WithResponse("https://api.osv.dev/v1/query", 200, body)
}

func (m *MockHTTPClientForCooldown) WithError(err error) *MockHTTPClientForCooldown {
	m.err = err
	return m
}

func (m *MockHTTPClientForCooldown) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Check for exact URL match first
	if resp, ok := m.responses[req.URL.String()]; ok {
		return &http.Response{
			StatusCode: resp.statusCode,
			Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
			Header:     make(http.Header),
		}, nil
	}

	// Check for host-based match (for OSV API which may have different query params)
	for url, resp := range m.responses {
		if req.URL.Host != "" && strings.Contains(url, req.URL.Host) {
			return &http.Response{
				StatusCode: resp.statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
				Header:     make(http.Header),
			}, nil
		}
	}

	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewBufferString("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func TestCooldownConfig(t *testing.T) {
	// Not parallel - modifies global environment variables

	tests := []struct {
		name           string
		envHours       string
		envEcosystems  string
		expectedHours  int
		expectedNpm    bool
		expectedPython bool
		expectedGo     bool
	}{
		{
			name:           "default values",
			envHours:       "",
			envEcosystems:  "",
			expectedHours:  72,
			expectedNpm:    true,
			expectedPython: false,
			expectedGo:     false,
		},
		{
			name:           "custom hours",
			envHours:       "168",
			envEcosystems:  "",
			expectedHours:  168,
			expectedNpm:    true,
			expectedPython: false,
			expectedGo:     false,
		},
		{
			name:           "multiple ecosystems",
			envHours:       "72",
			envEcosystems:  "npm,python,go",
			expectedHours:  72,
			expectedNpm:    true,
			expectedPython: true,
			expectedGo:     true,
		},
		{
			name:           "disabled with zero hours",
			envHours:       "0",
			envEcosystems:  "npm",
			expectedHours:  0,
			expectedNpm:    false, // Disabled because hours=0
			expectedPython: false,
			expectedGo:     false,
		},
		{
			name:           "none ecosystem disables all",
			envHours:       "72",
			envEcosystems:  "none",
			expectedHours:  72,
			expectedNpm:    false,
			expectedPython: false,
			expectedGo:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset config singleton
			packageversions.ResetCooldownConfigForTesting()

			// Set environment
			if tt.envHours != "" {
				os.Setenv("PACKAGE_COOLDOWN_HOURS", tt.envHours)
			} else {
				os.Unsetenv("PACKAGE_COOLDOWN_HOURS")
			}
			if tt.envEcosystems != "" {
				os.Setenv("PACKAGE_COOLDOWN_ECOSYSTEMS", tt.envEcosystems)
			} else {
				os.Unsetenv("PACKAGE_COOLDOWN_ECOSYSTEMS")
			}

			// Get config
			config := packageversions.GetCooldownConfig()

			if config.Hours != tt.expectedHours {
				t.Errorf("expected hours %d, got %d", tt.expectedHours, config.Hours)
			}

			if config.IsEcosystemCooldownEnabled("npm") != tt.expectedNpm {
				t.Errorf("expected npm enabled=%v, got %v", tt.expectedNpm, config.IsEcosystemCooldownEnabled("npm"))
			}

			if config.IsEcosystemCooldownEnabled("python") != tt.expectedPython {
				t.Errorf("expected python enabled=%v, got %v", tt.expectedPython, config.IsEcosystemCooldownEnabled("python"))
			}

			if config.IsEcosystemCooldownEnabled("go") != tt.expectedGo {
				t.Errorf("expected go enabled=%v, got %v", tt.expectedGo, config.IsEcosystemCooldownEnabled("go"))
			}
		})
	}

	// Clean up
	os.Unsetenv("PACKAGE_COOLDOWN_HOURS")
	os.Unsetenv("PACKAGE_COOLDOWN_ECOSYSTEMS")
	packageversions.ResetCooldownConfigForTesting()
}

func TestApplyCooldown(t *testing.T) {
	// Not parallel - modifies global environment variables

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	now := time.Now()

	tests := []struct {
		name            string
		envHours        string
		envEcosystems   string
		ecosystem       string
		packageName     string
		versions        []packageversions.VersionWithDate
		latestVersion   string
		osvHasVulns     bool
		expectedVersion string
		expectCooldown  bool
	}{
		{
			name:          "cooldown not enabled for ecosystem",
			envHours:      "72",
			envEcosystems: "python", // npm not enabled
			ecosystem:     "npm",
			packageName:   "lodash",
			versions: []packageversions.VersionWithDate{
				{Version: "4.17.21", PublishedAt: now.Add(-24 * time.Hour)},     // 1 day old
				{Version: "4.17.20", PublishedAt: now.Add(-7 * 24 * time.Hour)}, // 7 days old
			},
			latestVersion:   "4.17.21",
			osvHasVulns:     false,
			expectedVersion: "4.17.21", // No cooldown applied
			expectCooldown:  false,
		},
		{
			name:          "latest version outside cooldown window",
			envHours:      "72",
			envEcosystems: "npm",
			ecosystem:     "npm",
			packageName:   "lodash",
			versions: []packageversions.VersionWithDate{
				{Version: "4.17.21", PublishedAt: now.Add(-7 * 24 * time.Hour)},  // 7 days old
				{Version: "4.17.20", PublishedAt: now.Add(-14 * 24 * time.Hour)}, // 14 days old
			},
			latestVersion:   "4.17.21",
			osvHasVulns:     false,
			expectedVersion: "4.17.21", // No cooldown needed
			expectCooldown:  false,
		},
		{
			name:          "cooldown applied - returns older version",
			envHours:      "72",
			envEcosystems: "npm",
			ecosystem:     "npm",
			packageName:   "lodash",
			versions: []packageversions.VersionWithDate{
				{Version: "4.17.21", PublishedAt: now.Add(-24 * time.Hour)},     // 1 day old - within cooldown
				{Version: "4.17.20", PublishedAt: now.Add(-7 * 24 * time.Hour)}, // 7 days old - outside cooldown
			},
			latestVersion:   "4.17.21",
			osvHasVulns:     false,
			expectedVersion: "4.17.20", // Cooldown version
			expectCooldown:  true,
		},
		{
			name:          "cooldown version has vulnerabilities - bypass cooldown",
			envHours:      "72",
			envEcosystems: "npm",
			ecosystem:     "npm",
			packageName:   "lodash",
			versions: []packageversions.VersionWithDate{
				{Version: "4.17.21", PublishedAt: now.Add(-24 * time.Hour)},     // 1 day old
				{Version: "4.17.20", PublishedAt: now.Add(-7 * 24 * time.Hour)}, // 7 days old
			},
			latestVersion:   "4.17.21",
			osvHasVulns:     true,      // 4.17.20 has vulns
			expectedVersion: "4.17.21", // Latest because cooldown version has vulns
			expectCooldown:  false,
		},
		{
			name:          "all versions within cooldown - return latest",
			envHours:      "168", // 7 days
			envEcosystems: "npm",
			ecosystem:     "npm",
			packageName:   "new-package",
			versions: []packageversions.VersionWithDate{
				{Version: "1.0.2", PublishedAt: now.Add(-24 * time.Hour)},
				{Version: "1.0.1", PublishedAt: now.Add(-48 * time.Hour)},
				{Version: "1.0.0", PublishedAt: now.Add(-72 * time.Hour)},
			},
			latestVersion:   "1.0.2",
			osvHasVulns:     false,
			expectedVersion: "1.0.2", // All within cooldown, return latest
			expectCooldown:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset config singleton and OSV cache
			packageversions.ResetCooldownConfigForTesting()
			packageversions.ClearOSVCacheForTesting()

			// Set environment
			os.Setenv("PACKAGE_COOLDOWN_HOURS", tt.envHours)
			os.Setenv("PACKAGE_COOLDOWN_ECOSYSTEMS", tt.envEcosystems)

			// Create mock client with OSV response
			mockClient := NewMockHTTPClientForCooldown().WithOSVResponse(tt.osvHasVulns)

			// Apply cooldown
			selectedVersion, cooldownInfo, err := packageversions.ApplyCooldown(
				logger,
				mockClient,
				tt.ecosystem,
				tt.packageName,
				tt.versions,
				tt.latestVersion,
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if selectedVersion != tt.expectedVersion {
				t.Errorf("expected version %s, got %s", tt.expectedVersion, selectedVersion)
			}

			if tt.expectCooldown {
				if cooldownInfo == nil {
					t.Error("expected cooldown info, got nil")
				} else if !cooldownInfo.Applied {
					t.Error("expected cooldown to be applied")
				}
			} else if cooldownInfo != nil && cooldownInfo.Applied {
				t.Errorf("did not expect cooldown to be applied, but it was: %+v", cooldownInfo)
			}
		})
	}

	// Clean up
	os.Unsetenv("PACKAGE_COOLDOWN_HOURS")
	os.Unsetenv("PACKAGE_COOLDOWN_ECOSYSTEMS")
	packageversions.ResetCooldownConfigForTesting()
}

func TestOSVEcosystemMapping(t *testing.T) {
	// Not parallel - modifies global environment variables

	// This tests the internal mapping - we do it indirectly via ApplyCooldown
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	now := time.Now()

	ecosystems := []struct {
		ecosystem string
		supported bool
	}{
		{"npm", true},
		{"python", true},
		{"go", true},
		{"rust", true},
		{"java-maven", true},
		{"java-gradle", true},
		{"docker", false},         // Not supported in OSV
		{"github-actions", false}, // Not supported in OSV
	}

	for _, tc := range ecosystems {
		t.Run(tc.ecosystem, func(t *testing.T) {
			packageversions.ResetCooldownConfigForTesting()
			packageversions.ClearOSVCacheForTesting()
			os.Setenv("PACKAGE_COOLDOWN_HOURS", "72")
			os.Setenv("PACKAGE_COOLDOWN_ECOSYSTEMS", tc.ecosystem)

			mockClient := NewMockHTTPClientForCooldown().WithOSVResponse(false)

			versions := []packageversions.VersionWithDate{
				{Version: "2.0.0", PublishedAt: now.Add(-24 * time.Hour)},
				{Version: "1.0.0", PublishedAt: now.Add(-7 * 24 * time.Hour)},
			}

			selectedVersion, _, err := packageversions.ApplyCooldown(
				logger,
				mockClient,
				tc.ecosystem,
				"test-package",
				versions,
				"2.0.0",
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// For supported ecosystems, cooldown should be applied (return 1.0.0)
			// For unsupported ecosystems, we still apply cooldown but OSV check fails gracefully
			if tc.supported {
				if selectedVersion != "1.0.0" {
					t.Errorf("expected cooldown to apply for %s, got version %s", tc.ecosystem, selectedVersion)
				}
			}
		})
	}

	os.Unsetenv("PACKAGE_COOLDOWN_HOURS")
	os.Unsetenv("PACKAGE_COOLDOWN_ECOSYSTEMS")
	packageversions.ResetCooldownConfigForTesting()
}

func TestCooldownDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hours            int
		expectedDuration time.Duration
	}{
		{72, 72 * time.Hour},
		{168, 168 * time.Hour},
		{0, 0},
		{24, 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_hours", tt.hours), func(t *testing.T) {
			config := &packageversions.CooldownConfig{Hours: tt.hours}
			if config.GetCooldownDuration() != tt.expectedDuration {
				t.Errorf("expected duration %v, got %v", tt.expectedDuration, config.GetCooldownDuration())
			}
		})
	}
}
