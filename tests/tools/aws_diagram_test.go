package tools

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/diagrams/awsdiagram"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWSDiagramTool_EnablementCheck(t *testing.T) {
	tool := &awsdiagram.AWSDiagramTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cache := &sync.Map{}

	// Test without enablement
	_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

	args := map[string]interface{}{
		"action": "examples",
	}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aws-diagram tool is not enabled")
}

func TestAWSDiagramTool_ExamplesAction(t *testing.T) {
	tool := &awsdiagram.AWSDiagramTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cache := &sync.Map{}

	// Enable the tool
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws-diagram")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	args := map[string]interface{}{
		"action":       "examples",
		"diagram_type": "aws",
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.Content)
}

func TestAWSDiagramTool_ListIconsAction(t *testing.T) {
	tool := &awsdiagram.AWSDiagramTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cache := &sync.Map{}

	// Enable the tool
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws-diagram")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	args := map[string]interface{}{
		"action":   "list_icons",
		"provider": "aws",
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.Content)
}

func TestAWSDiagramTool_InvalidAction(t *testing.T) {
	tool := &awsdiagram.AWSDiagramTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cache := &sync.Map{}

	// Enable the tool
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws-diagram")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	args := map[string]interface{}{
		"action": "invalid",
	}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action")
}

func TestAWSDiagramTool_MissingAction(t *testing.T) {
	tool := &awsdiagram.AWSDiagramTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cache := &sync.Map{}

	// Enable the tool
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws-diagram")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	args := map[string]interface{}{}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required parameter: action")
}

func TestAWSDiagramTool_Definition(t *testing.T) {
	tool := &awsdiagram.AWSDiagramTool{}
	def := tool.Definition()

	assert.Equal(t, "aws_diagram", def.Name)
	assert.Contains(t, def.Description, "Generate AWS architecture diagrams")

	// Just check that we have properties - the schema structure is handled by mcp-go
	assert.NotNil(t, def.InputSchema)
	assert.NotNil(t, def.InputSchema.Properties)

	// Check that the action parameter exists
	_, hasAction := def.InputSchema.Properties["action"]
	assert.True(t, hasAction, "action parameter should be defined")
}
