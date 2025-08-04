package m2e

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/m2e/pkg/converter"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// M2ETool implements the American to British English converter tool
type M2ETool struct{}

const (
	// DefaultMaxTextLength is the default maximum length for text input
	DefaultMaxTextLength = 40000
	// M2EMaxLengthEnvVar is the environment variable for configuring max text length
	M2EMaxLengthEnvVar = "M2E_MAX_LENGTH"
)

// getMaxTextLength returns the configured maximum text length
func getMaxTextLength() int {
	if envValue := os.Getenv(M2EMaxLengthEnvVar); envValue != "" {
		if value, err := strconv.Atoi(envValue); err == nil && value > 0 {
			return value
		}
	}
	return DefaultMaxTextLength
}

// init registers the m2e tool
func init() {
	registry.Register(&M2ETool{})
}

// Definition returns the tool's definition for MCP registration
func (m *M2ETool) Definition() mcp.Tool {
	return mcp.NewTool(
		"murican_to_english",
		mcp.WithDescription(`Convert American English text to standard International / British English spelling.

Default behaviour: Updates files in place. Provide a file_path to convert a file.
Inline mode: Provide text parameter instead to get converted text returned directly.`),
		mcp.WithString("file_path",
			mcp.Description("Fully qualified absolute path to the file to update in place"),
		),
		mcp.WithString("text",
			mcp.MaxLength(getMaxTextLength()),
			mcp.Description("Text to convert and return inline (if not using file_path)"),
		),
		mcp.WithBoolean("keep_smart_quotes",
			mcp.Description("Whether to keep smart quotes and em-dashes as-is (default: false, as we usually want to normalise them)"),
		),
	)
}

// Execute executes the m2e tool
func (m *M2ETool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse and validate parameters
	request, err := m.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Create converter instance
	conv, err := converter.NewConverter()
	if err != nil {
		return nil, fmt.Errorf("failed to initialise converter: %w", err)
	}

	// Determine mode based on provided parameters
	if request.Text != "" {
		// Inline mode: text provided
		return m.executeInlineMode(conv, request, logger)
	} else if request.FilePath != "" {
		// Update file mode: file_path provided (default)
		return m.executeUpdateFileMode(conv, request, logger)
	} else {
		return nil, fmt.Errorf("either 'text' or 'file_path' parameter must be provided")
	}
}

// executeInlineMode handles inline text conversion
func (m *M2ETool) executeInlineMode(conv *converter.Converter, request *ConvertRequest, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	// Convert the text (note: !KeepSmartQuotes because the converter expects normaliseSmartQuotes bool)
	normaliseSmartQuotes := !request.KeepSmartQuotes
	convertedText := conv.ConvertToBritish(request.Text, normaliseSmartQuotes)

	// Count changes by comparing original and converted text
	changesCount := m.countChanges(request.Text, convertedText)

	// Log the conversion (avoid logging sensitive content)
	logger.WithFields(logrus.Fields{
		"mode":              "inline",
		"text_length":       len(request.Text),
		"changes_count":     changesCount,
		"keep_smart_quotes": request.KeepSmartQuotes,
	}).Debug("Text converted from American to English")

	return mcp.NewToolResultText(fmt.Sprintf("Successfully converted text from American to English.\n\nOriginal text length: %d characters\nChanges made: %d\nSmart quotes normalised: %t\n\nConverted text:\n%s",
		len(request.Text), changesCount, normaliseSmartQuotes, convertedText)), nil
}

// executeUpdateFileMode handles file update operations
func (m *M2ETool) executeUpdateFileMode(conv *converter.Converter, request *ConvertRequest, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	// Read the file
	originalContent, err := os.ReadFile(request.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", request.FilePath, err)
	}

	originalText := string(originalContent)

	// Validate file content length
	maxLength := getMaxTextLength()
	if len(originalText) > maxLength {
		return nil, fmt.Errorf("file content exceeds maximum length of %d characters (got %d)", maxLength, len(originalText))
	}

	// Convert the text (note: !KeepSmartQuotes because the converter expects normaliseSmartQuotes bool)
	normaliseSmartQuotes := !request.KeepSmartQuotes
	convertedText := conv.ConvertToBritish(originalText, normaliseSmartQuotes)

	// Count changes by comparing original and converted text
	changesCount := m.countChanges(originalText, convertedText)

	// Only write the file if there are changes
	if changesCount > 0 {
		err = os.WriteFile(request.FilePath, []byte(convertedText), 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", request.FilePath, err)
		}
	}

	// Log the conversion
	logger.WithFields(logrus.Fields{
		"mode":              "update_file",
		"file_path":         request.FilePath,
		"file_size":         len(originalContent),
		"changes_count":     changesCount,
		"keep_smart_quotes": request.KeepSmartQuotes,
	}).Info("File processed for American to English conversion")

	if changesCount > 0 {
		return mcp.NewToolResultText(fmt.Sprintf("Successfully updated file %s\n\nFile size: %d bytes\nChanges made: %d\nSmart quotes normalised: %t\n\nThe file has been updated in place with English spellings.",
			request.FilePath, len(originalContent), changesCount, normaliseSmartQuotes)), nil
	} else {
		return mcp.NewToolResultText(fmt.Sprintf("No changes needed for file %s\n\nFile size: %d bytes\nChanges made: 0\nSmart quotes normalised: %t\n\nThe file already uses English spellings or contains no American spellings to convert.",
			request.FilePath, len(originalContent), normaliseSmartQuotes)), nil
	}
}

// parseRequest parses and validates the request parameters
func (m *M2ETool) parseRequest(args map[string]interface{}) (*ConvertRequest, error) {
	request := &ConvertRequest{}

	// Parse text (for inline mode)
	if text, ok := args["text"].(string); ok {
		request.Text = text
	}

	// Parse file_path (for update_file mode - default)
	if filePath, ok := args["file_path"].(string); ok {
		request.FilePath = filePath
	}

	// Parse keep_smart_quotes (optional, default false)
	if keepSmartQuotes, ok := args["keep_smart_quotes"].(bool); ok {
		request.KeepSmartQuotes = keepSmartQuotes
	}

	// Validate that exactly one of text or file_path is provided
	if request.Text != "" && request.FilePath != "" {
		return nil, fmt.Errorf("cannot provide both 'text' and 'file_path' parameters - use one or the other")
	}

	if request.Text == "" && request.FilePath == "" {
		return nil, fmt.Errorf("either 'text' or 'file_path' parameter must be provided")
	}

	// Validate parameters based on what was provided
	if request.Text != "" {
		// Inline mode validation
		if strings.TrimSpace(request.Text) == "" {
			return nil, fmt.Errorf("text parameter cannot be empty")
		}
		// Validate text length
		maxLength := getMaxTextLength()
		if len(request.Text) > maxLength {
			return nil, fmt.Errorf("text exceeds maximum length of %d characters (got %d)", maxLength, len(request.Text))
		}
	} else if request.FilePath != "" {
		// Update file mode validation
		if strings.TrimSpace(request.FilePath) == "" {
			return nil, fmt.Errorf("file_path parameter cannot be empty")
		}
		// Validate that the file path is absolute
		if !strings.HasPrefix(request.FilePath, "/") {
			return nil, fmt.Errorf("file_path must be a fully qualified absolute path, got: %s", request.FilePath)
		}
	}

	return request, nil
}

// countChanges counts the number of changes made during conversion
func (m *M2ETool) countChanges(original, converted string) int {
	if original == converted {
		return 0
	}

	// Simple word-based change counting
	originalWords := strings.Fields(original)
	convertedWords := strings.Fields(converted)

	changes := 0
	maxLen := len(originalWords)
	if len(convertedWords) > maxLen {
		maxLen = len(convertedWords)
	}

	for i := 0; i < maxLen; i++ {
		var origWord, convWord string
		if i < len(originalWords) {
			origWord = originalWords[i]
		}
		if i < len(convertedWords) {
			convWord = convertedWords[i]
		}

		if origWord != convWord {
			changes++
		}
	}

	return changes
}
