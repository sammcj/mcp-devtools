package unit_test

import (
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		expectedMajor int
		expectedMinor int
		expectedPatch int
		expectError   bool
	}{
		{
			name:          "simple version",
			version:       "1.2.3",
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
			expectError:   false,
		},
		{
			name:          "version with v prefix",
			version:       "v1.2.3",
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
			expectError:   false,
		},
		{
			name:          "version with V prefix",
			version:       "V1.2.3",
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
			expectError:   false,
		},
		{
			name:          "version with build metadata",
			version:       "1.2.3+build.1",
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
			expectError:   false,
		},
		{
			name:          "version with pre-release",
			version:       "1.2.3-alpha.1",
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
			expectError:   false,
		},
		{
			name:          "major only",
			version:       "1",
			expectedMajor: 1,
			expectedMinor: 0,
			expectedPatch: 0,
			expectError:   false,
		},
		{
			name:          "major and minor only",
			version:       "1.2",
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 0,
			expectError:   false,
		},
		{
			name:        "empty version",
			version:     "",
			expectError: true,
		},
		{
			name:        "invalid major version",
			version:     "abc.2.3",
			expectError: true,
		},
		{
			name:        "invalid minor version",
			version:     "1.abc.3",
			expectError: true,
		},
		{
			name:        "invalid patch version",
			version:     "1.2.abc",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, err := packageversions.ParseVersion(tt.version)

			if tt.expectError {
				testutils.AssertError(t, err)
			} else {
				testutils.AssertNoError(t, err)
				testutils.AssertEqual(t, tt.expectedMajor, major)
				testutils.AssertEqual(t, tt.expectedMinor, minor)
				testutils.AssertEqual(t, tt.expectedPatch, patch)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{
			name:     "equal versions",
			v1:       "1.2.3",
			v2:       "1.2.3",
			expected: 0,
		},
		{
			name:     "v1 greater major",
			v1:       "2.0.0",
			v2:       "1.9.9",
			expected: 1,
		},
		{
			name:     "v1 lesser major",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: -1,
		},
		{
			name:     "v1 greater minor",
			v1:       "1.2.0",
			v2:       "1.1.9",
			expected: 1,
		},
		{
			name:     "v1 lesser minor",
			v1:       "1.1.0",
			v2:       "1.2.0",
			expected: -1,
		},
		{
			name:     "v1 greater patch",
			v1:       "1.2.3",
			v2:       "1.2.2",
			expected: 1,
		},
		{
			name:     "v1 lesser patch",
			v1:       "1.2.2",
			v2:       "1.2.3",
			expected: -1,
		},
		{
			name:     "versions with prefixes",
			v1:       "v1.2.3",
			v2:       "V1.2.3",
			expected: 0,
		},
		{
			name:     "versions with build metadata",
			v1:       "1.2.3+build.1",
			v2:       "1.2.3+build.2",
			expected: 0,
		},
		{
			name:     "versions with pre-release",
			v1:       "1.2.3-alpha.1",
			v2:       "1.2.3-beta.1",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := packageversions.CompareVersions(tt.v1, tt.v2)
			testutils.AssertNoError(t, err)
			testutils.AssertEqual(t, tt.expected, result)
		})
	}
}

func TestCompareVersions_InvalidVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
	}{
		{
			name: "invalid v1",
			v1:   "invalid",
			v2:   "1.2.3",
		},
		{
			name: "invalid v2",
			v1:   "1.2.3",
			v2:   "invalid",
		},
		{
			name: "both invalid",
			v1:   "invalid1",
			v2:   "invalid2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := packageversions.CompareVersions(tt.v1, tt.v2)
			testutils.AssertError(t, err)
		})
	}
}

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "clean version",
			version:  "1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "caret prefix",
			version:  "^1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "tilde prefix",
			version:  "~1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "greater than prefix",
			version:  ">1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "greater than or equal prefix",
			version:  ">=1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "less than prefix",
			version:  "<1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "equals prefix",
			version:  "=1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "multiple prefixes",
			version:  "^>=1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "empty string",
			version:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := packageversions.CleanVersion(tt.version)
			testutils.AssertEqual(t, tt.expected, result)
		})
	}
}

func TestExtractMajorVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expected    int
		expectError bool
	}{
		{
			name:        "simple version",
			version:     "1.2.3",
			expected:    1,
			expectError: false,
		},
		{
			name:        "version with v prefix",
			version:     "v2.1.0",
			expected:    2,
			expectError: false,
		},
		{
			name:        "major only",
			version:     "5",
			expected:    5,
			expectError: false,
		},
		{
			name:        "invalid version",
			version:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := packageversions.ExtractMajorVersion(tt.version)

			if tt.expectError {
				testutils.AssertError(t, err)
			} else {
				testutils.AssertNoError(t, err)
				testutils.AssertEqual(t, tt.expected, result)
			}
		})
	}
}

func TestStringPtr(t *testing.T) {
	str := "test string"
	ptr := packageversions.StringPtr(str)

	testutils.AssertNotNil(t, ptr)
	testutils.AssertEqual(t, str, *ptr)
}

func TestIntPtr(t *testing.T) {
	num := 42
	ptr := packageversions.IntPtr(num)

	testutils.AssertNotNil(t, ptr)
	testutils.AssertEqual(t, num, *ptr)
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		query    string
		expected bool
	}{
		{
			name:     "exact match",
			str:      "hello",
			query:    "hello",
			expected: true,
		},
		{
			name:     "substring match",
			str:      "hello world",
			query:    "world",
			expected: true,
		},
		{
			name:     "fuzzy match",
			str:      "hello world",
			query:    "hlwrld",
			expected: true,
		},
		{
			name:     "partial fuzzy match",
			str:      "javascript",
			query:    "js",
			expected: true,
		},
		{
			name:     "no match",
			str:      "hello",
			query:    "xyz",
			expected: false,
		},
		{
			name:     "empty query",
			str:      "anything",
			query:    "",
			expected: true,
		},
		{
			name:     "empty string",
			str:      "",
			query:    "something",
			expected: false,
		},
		{
			name:     "both empty",
			str:      "",
			query:    "",
			expected: true,
		},
		{
			name:     "case sensitive",
			str:      "Hello",
			query:    "hello",
			expected: false,
		},
		{
			name:     "complex fuzzy match",
			str:      "react-dom",
			query:    "rdom",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := packageversions.FuzzyMatch(tt.str, tt.query)
			testutils.AssertEqual(t, tt.expected, result)
		})
	}
}
