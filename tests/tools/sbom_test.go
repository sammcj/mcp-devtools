//go:build sbom_vuln_tools
// +build sbom_vuln_tools

package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/sbom"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSBOMTool_Definition(t *testing.T) {
	tool := &sbom.SBOMTool{}

	definition := tool.Definition()

	assert.Equal(t, "sbom", definition.Name)
	assert.Contains(t, definition.Description, "Software Bill of Materials")
	// Note: Security environment variables were removed from descriptions

	// Check that source parameter is required
	sourceParam := findParameter(definition.InputSchema.Properties, "source")
	require.NotNil(t, sourceParam)
	assert.Contains(t, definition.InputSchema.Required, "source")

	// Check optional parameters exist with proper defaults
	formatParam := findParameter(definition.InputSchema.Properties, "output_format")
	require.NotNil(t, formatParam)
	assert.Equal(t, "syft-json", formatParam.Default)

	devDepsParam := findParameter(definition.InputSchema.Properties, "include_dev_dependencies")
	require.NotNil(t, devDepsParam)
	assert.Equal(t, false, devDepsParam.Default)

	// Verify that timeout parameter was not added (it's handled internally now)
	timeoutParam := findParameter(definition.InputSchema.Properties, "timeout_minutes")
	assert.Nil(t, timeoutParam, "timeout_minutes parameter should not be exposed to users")
}

func TestSBOMTool_Execute_ToolDisabled(t *testing.T) {
	// Ensure SBOM tool is disabled by default
	_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	cache := &sync.Map{}

	args := map[string]interface{}{
		"source": "/tmp", // Dummy path
	}

	ctx := context.Background()

	// Tool should fail immediately when disabled
	result, err := tool.Execute(ctx, logger, cache, args)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "SBOM tool is not enabled")
}

func TestSBOMTool_Execute_InvalidParameters(t *testing.T) {
	// Enable the SBOM tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sbom")

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	cache := &sync.Map{}
	ctx := context.Background()

	tests := []struct {
		name     string
		args     map[string]interface{}
		errorMsg string
	}{
		{
			name:     "missing source",
			args:     map[string]interface{}{},
			errorMsg: "missing or invalid required parameter: source",
		},
		{
			name:     "empty source",
			args:     map[string]interface{}{"source": ""},
			errorMsg: "missing or invalid required parameter: source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, logger, cache, tt.args)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

// getCurrentDir returns the current working directory for test use
func getCurrentDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	// Go up two levels to get to project root which has go.mod
	return filepath.Join(cwd, "..", "..")
}

func TestSBOMTool_Execute_ParameterValidation(t *testing.T) {
	// Enable the SBOM tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sbom")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	cache := &sync.Map{}
	ctx := context.Background()

	// Test with relative path (should fail fast)
	args := map[string]interface{}{
		"source":      "./relative/path",
		"output_file": "/tmp/test-sbom.json",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source path must be absolute")
}

func TestSBOMTool_Execute_RelativeOutputFile(t *testing.T) {
	// Enable the SBOM tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sbom")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	cache := &sync.Map{}
	ctx := context.Background()

	// Test with relative output file path (should fail fast)
	args := map[string]interface{}{
		"source":      getCurrentDir(),
		"output_file": "./relative/output.json", // Relative path
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "output_file path must be absolute")
}

func TestSBOMTool_ValidateSourcePath_Security(t *testing.T) {
	// Enable the SBOM tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sbom")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	cache := &sync.Map{}
	ctx := context.Background()

	// Test path traversal (should fail fast during parameter validation)
	args := map[string]interface{}{
		"source":      "../../../etc",
		"output_file": "/tmp/test-sbom.json",
	}
	result, err := tool.Execute(ctx, logger, cache, args)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source path must be absolute")

	// Test non-existent path (should fail fast during source validation)
	args = map[string]interface{}{
		"source":      "/nonexistent/path/12345",
		"output_file": "/tmp/test-sbom.json",
	}
	result, err = tool.Execute(ctx, logger, cache, args)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source path does not exist")
}

func TestSBOMTool_ProvideExtendedInfo(t *testing.T) {
	tool := &sbom.SBOMTool{}

	extendedInfo := tool.ProvideExtendedInfo()

	require.NotNil(t, extendedInfo)
	assert.NotEmpty(t, extendedInfo.Examples)
	assert.NotEmpty(t, extendedInfo.CommonPatterns)
	assert.NotEmpty(t, extendedInfo.Troubleshooting)
	assert.NotEmpty(t, extendedInfo.ParameterDetails)
	assert.NotEmpty(t, extendedInfo.WhenToUse)
	assert.NotEmpty(t, extendedInfo.WhenNotToUse)

	// Check that examples have required fields
	for i, example := range extendedInfo.Examples {
		assert.NotEmpty(t, example.Description, "Example %d should have description", i)
		assert.NotEmpty(t, example.Arguments, "Example %d should have arguments", i)
		assert.NotEmpty(t, example.ExpectedResult, "Example %d should have expected result", i)
	}

	// Check that troubleshooting tips have required fields
	for i, tip := range extendedInfo.Troubleshooting {
		assert.NotEmpty(t, tip.Problem, "Troubleshooting tip %d should have problem", i)
		assert.NotEmpty(t, tip.Solution, "Troubleshooting tip %d should have solution", i)
	}
}

// Helper function to find a parameter in the schema properties
func findParameter(properties map[string]interface{}, name string) *struct {
	Default interface{} `json:"default,omitempty"`
} {
	prop, exists := properties[name]
	if !exists {
		return nil
	}

	propMap, ok := prop.(map[string]interface{})
	if !ok {
		return nil
	}

	result := &struct {
		Default interface{} `json:"default,omitempty"`
	}{}

	if defaultVal, exists := propMap["default"]; exists {
		result.Default = defaultVal
	}

	return result
}
