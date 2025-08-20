package unit

import (
	"encoding/json"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageDetailsStructure(t *testing.T) {
	// Test that the new PackageDetails structure marshals/unmarshals correctly
	original := packageversions.PackageVersion{
		Name:           "serde",
		CurrentVersion: packageversions.StringPtr("1.0.0"),
		LatestVersion:  "1.0.210",
		Registry:       "crates.io",
		Details: &packageversions.PackageDetails{
			Description:   packageversions.StringPtr("A generic serialisation/deserialisation framework"),
			Homepage:      packageversions.StringPtr("https://serde.rs"),
			Repository:    packageversions.StringPtr("https://github.com/serde-rs/serde"),
			Documentation: packageversions.StringPtr("https://docs.rs/serde"),
			License:       packageversions.StringPtr("MIT OR Apache-2.0"),
			Downloads:     packageversions.Int64Ptr(1000000000),
			Keywords:      []string{"serde", "serialisation", "no_std"},
			Publisher:     packageversions.StringPtr("dtolnay"),
			Rust: &packageversions.RustDetails{
				Edition:         packageversions.StringPtr("2018"),
				RustVersion:     packageversions.StringPtr("1.60"),
				CrateSize:       packageversions.Int64Ptr(45000),
				Categories:      []string{"encoding", "no-std"},
				RecentDownloads: packageversions.Int64Ptr(50000000),
			},
		},
	}

	// Test JSON marshalling
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	// Verify structure in JSON
	assert.Contains(t, string(jsonData), `"name":"serde"`)
	assert.Contains(t, string(jsonData), `"latestVersion":"1.0.210"`)
	assert.Contains(t, string(jsonData), `"details":{`)
	assert.Contains(t, string(jsonData), `"rust":{`)
	assert.Contains(t, string(jsonData), `"edition":"2018"`)

	// Test JSON unmarshalling
	var unmarshalled packageversions.PackageVersion
	err = json.Unmarshal(jsonData, &unmarshalled)
	require.NoError(t, err)

	// Verify core fields
	assert.Equal(t, "serde", unmarshalled.Name)
	assert.Equal(t, "1.0.210", unmarshalled.LatestVersion)
	assert.Equal(t, "crates.io", unmarshalled.Registry)
	require.NotNil(t, unmarshalled.Details)

	// Verify common details
	assert.Equal(t, "A generic serialisation/deserialisation framework", *unmarshalled.Details.Description)
	assert.Equal(t, "https://serde.rs", *unmarshalled.Details.Homepage)
	assert.Equal(t, int64(1000000000), *unmarshalled.Details.Downloads)
	assert.Equal(t, []string{"serde", "serialisation", "no_std"}, unmarshalled.Details.Keywords)

	// Verify Rust-specific details
	require.NotNil(t, unmarshalled.Details.Rust)
	assert.Equal(t, "2018", *unmarshalled.Details.Rust.Edition)
	assert.Equal(t, "1.60", *unmarshalled.Details.Rust.RustVersion)
	assert.Equal(t, int64(45000), *unmarshalled.Details.Rust.CrateSize)
	assert.Equal(t, []string{"encoding", "no-std"}, unmarshalled.Details.Rust.Categories)
}

func TestPackageVersionWithoutDetails(t *testing.T) {
	// Test basic package version without details
	basic := packageversions.PackageVersion{
		Name:          "tokio",
		LatestVersion: "1.40.0",
		Registry:      "crates.io",
	}

	jsonData, err := json.Marshal(basic)
	require.NoError(t, err)

	// Should not contain details field when nil
	assert.NotContains(t, string(jsonData), `"details"`)

	var unmarshalled packageversions.PackageVersion
	err = json.Unmarshal(jsonData, &unmarshalled)
	require.NoError(t, err)

	assert.Equal(t, "tokio", unmarshalled.Name)
	assert.Equal(t, "1.40.0", unmarshalled.LatestVersion)
	assert.Nil(t, unmarshalled.Details)
}

func TestPackageVersionSkipped(t *testing.T) {
	// Test skipped package structure
	skipped := packageversions.PackageVersion{
		Name:          "nonexistent-crate",
		LatestVersion: "unknown",
		Registry:      "crates.io",
		Skipped:       true,
		SkipReason:    "Failed to fetch crate info: 404 Not Found",
	}

	jsonData, err := json.Marshal(skipped)
	require.NoError(t, err)

	assert.Contains(t, string(jsonData), `"skipped":true`)
	assert.Contains(t, string(jsonData), `"skipReason":"Failed to fetch crate info: 404 Not Found"`)

	var unmarshalled packageversions.PackageVersion
	err = json.Unmarshal(jsonData, &unmarshalled)
	require.NoError(t, err)

	assert.True(t, unmarshalled.Skipped)
	assert.Equal(t, "Failed to fetch crate info: 404 Not Found", unmarshalled.SkipReason)
}
