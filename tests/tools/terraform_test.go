package tools

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/terraform_documentation"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestTerraformDocumentationTool_Execute_ToolEnabled(t *testing.T) {
	// Enable the terraform_documentation tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "terraform_documentation")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &terraform_documentation.TerraformDocumentationTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress output during tests
	cache := &sync.Map{}

	// Test missing action parameter - should fail with parameter error, not enablement error
	args := map[string]any{}
	result, err := tool.Execute(context.Background(), logger, cache, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required parameter: action")
	assert.NotContains(t, err.Error(), "terraform documentation tool is not enabled")
	assert.Nil(t, result)
}

func TestTerraformDocumentationTool_Definition(t *testing.T) {
	tool := &terraform_documentation.TerraformDocumentationTool{}
	definition := tool.Definition()

	assert.Equal(t, "terraform_documentation", definition.Name)
	assert.Contains(t, definition.Description, "Terraform Registry APIs")

	// Check that required parameters exist
	inputSchema := definition.InputSchema
	assert.NotNil(t, inputSchema)

	properties := inputSchema.Properties
	assert.NotNil(t, properties)

	// Action parameter should be required
	actionParam := properties["action"]
	assert.NotNil(t, actionParam)

	// Check required parameters list
	required := inputSchema.Required
	assert.Contains(t, required, "action")
}
