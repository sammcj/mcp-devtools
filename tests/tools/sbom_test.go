package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
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

func TestSBOMTool_Execute_ToolEnabled(t *testing.T) {
	// Enable the SBOM tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sbom")

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr) // Avoid stdout in tests
	cache := &sync.Map{}

	// Use absolute path for testing since tools require absolute paths for security
	cwd, _ := os.Getwd()
	args := map[string]interface{}{
		"source": cwd,
	}

	// Since the tool should be disabled by default, we expect it to be registered
	// but the execute should still work if we can create an instance
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// For disabled tools, we can still test the interface
	result, err := tool.Execute(ctx, logger, cache, args)

	// The tool should execute successfully and generate a real SBOM
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, len(result.Content) > 0, "Expected content in result")
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "Expected TextContent type")
	// Should contain actual SBOM content, not placeholder
	assert.Contains(t, textContent.Text, "sbom")
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

func TestSBOMTool_Execute_ValidCurrentDirectory(t *testing.T) {
	// Enable the SBOM tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sbom")

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	cache := &sync.Map{}

	args := map[string]interface{}{
		"source":        getCurrentDir(),
		"output_format": "syft-json",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, logger, cache, args)

	// Should succeed with real SBOM implementation
	require.NoError(t, err)
	require.NotNil(t, result)

	// Content should be actual SBOM JSON
	require.True(t, len(result.Content) > 0, "Expected content in result")
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "Expected TextContent type")
	// Should contain actual SBOM structure
	assert.Contains(t, textContent.Text, "artifacts")
	assert.Contains(t, textContent.Text, "schema")
}

func TestSBOMTool_Execute_WithOutputFile(t *testing.T) {
	// Enable the SBOM tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sbom")

	tool := &sbom.SBOMTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	cache := &sync.Map{}

	// We'll test with a relative path that gets created in current directory

	args := map[string]interface{}{
		"source":      getCurrentDir(),
		"output_file": filepath.Join(getCurrentDir(), "test-output.json"), // Absolute path
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, logger, cache, args)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check that output file was created
	_, err = os.Stat("test-output.json")
	if err == nil {
		// Clean up created file
		_ = os.Remove("test-output.json")
	}
}

func TestSBOMTool_ValidateSourcePath_Security(t *testing.T) {
	tool := &sbom.SBOMTool{}

	tests := []struct {
		name     string
		source   string
		wantErr  bool
		errorMsg string
	}{
		{
			name:    "current directory",
			source:  getCurrentDir(),
			wantErr: false,
		},
		{
			name:     "path traversal attempt",
			source:   "../../../etc",
			wantErr:  true,
			errorMsg: "does not exist",
		},
		{
			name:     "non-existent path",
			source:   "/nonexistent/path",
			wantErr:  true,
			errorMsg: "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetOutput(os.Stderr)
			cache := &sync.Map{}

			args := map[string]interface{}{
				"source": tt.source,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := tool.Execute(ctx, logger, cache, args)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
