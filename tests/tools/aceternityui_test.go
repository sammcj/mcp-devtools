package tools

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/aceternityui"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAceternityUITool(t *testing.T) {
	// Enable the tool for testing
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aceternity_ui")
	defer os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	tool := &aceternityui.AceternityUITool{}

	t.Run("Definition", func(t *testing.T) {
		def := tool.Definition()
		assert.Equal(t, "aceternity_ui", def.Name)
		assert.NotEmpty(t, def.Description)
		assert.NotEmpty(t, def.InputSchema.Properties)
	})

	t.Run("List Action", func(t *testing.T) {
		args := map[string]any{
			"action": "list",
		}

		result, err := tool.Execute(context.Background(), logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should return JSON array of components
		assert.NotEmpty(t, result.Content)
	})

	t.Run("Search Action", func(t *testing.T) {
		args := map[string]any{
			"action": "search",
			"query":  "grid",
		}

		result, err := tool.Execute(context.Background(), logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should return JSON array of matching components
		assert.NotEmpty(t, result.Content)
	})

	t.Run("Details Action", func(t *testing.T) {
		args := map[string]any{
			"action":        "details",
			"componentName": "bento-grid",
		}

		result, err := tool.Execute(context.Background(), logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should return JSON object with component details
		assert.NotEmpty(t, result.Content)
	})

	t.Run("Categories Action", func(t *testing.T) {
		args := map[string]any{
			"action": "categories",
		}

		result, err := tool.Execute(context.Background(), logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should return JSON array of categories
		assert.NotEmpty(t, result.Content)
	})

	t.Run("Invalid Action", func(t *testing.T) {
		args := map[string]any{
			"action": "invalid",
		}

		_, err := tool.Execute(context.Background(), logger, cache, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid action")
	})

	t.Run("Missing Query for Search", func(t *testing.T) {
		args := map[string]any{
			"action": "search",
		}

		_, err := tool.Execute(context.Background(), logger, cache, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query parameter is required")
	})

	t.Run("Missing ComponentName for Details", func(t *testing.T) {
		args := map[string]any{
			"action": "details",
		}

		_, err := tool.Execute(context.Background(), logger, cache, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "componentName parameter is required")
	})

	t.Run("Component Not Found", func(t *testing.T) {
		args := map[string]any{
			"action":        "details",
			"componentName": "nonexistent-component",
		}

		_, err := tool.Execute(context.Background(), logger, cache, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "component not found")
	})

	t.Run("Extended Help", func(t *testing.T) {
		extendedHelp := tool.ProvideExtendedInfo()
		assert.NotNil(t, extendedHelp)
		assert.NotEmpty(t, extendedHelp.Examples)
		assert.NotEmpty(t, extendedHelp.CommonPatterns)
		assert.NotEmpty(t, extendedHelp.Troubleshooting)
		assert.NotEmpty(t, extendedHelp.WhenToUse)
		assert.NotEmpty(t, extendedHelp.WhenNotToUse)
	})
}
