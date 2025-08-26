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
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
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
func (m *M2ETool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
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

	// Security content analysis for converted text
	source := security.SourceContext{
		Tool:        "murican_to_english",
		URL:         "inline_text",
		ContentType: "converted_text",
	}
	if result, err := security.AnalyseContent(convertedText, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, fmt.Errorf("content blocked by security policy: %s", result.Message)
		case security.ActionWarn:
			// Add security warning to logs
			logger.WithField("security_id", result.ID).Warn(result.Message)
		}
	}

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
	// Security check for file access (both read and write)
	if err := security.CheckFileAccess(request.FilePath); err != nil {
		return nil, err
	}

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

	// Security content analysis for converted text
	source := security.SourceContext{
		Tool:        "murican_to_english",
		URL:         request.FilePath,
		ContentType: "converted_text",
	}
	if result, err := security.AnalyseContent(convertedText, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, fmt.Errorf("content blocked by security policy: %s", result.Message)
		case security.ActionWarn:
			// Add security warning to logs
			logger.WithField("security_id", result.ID).Warn(result.Message)
		}
	}

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
func (m *M2ETool) parseRequest(args map[string]any) (*ConvertRequest, error) {
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
	maxLen := max(len(convertedWords), len(originalWords))

	for i := range maxLen {
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

// ProvideExtendedInfo provides detailed usage information for the m2e tool
func (m *M2ETool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Convert inline text from American to British English",
				Arguments: map[string]any{
					"text": "The color of the aluminum center was gray, and we realized it needed optimization.",
				},
				ExpectedResult: "Returns the converted text with British spellings: 'The colour of the aluminium centre was grey, and we realised it needed optimisation.'",
			},
			{
				Description: "Convert a file in place with British spellings",
				Arguments: map[string]any{
					"file_path": "/Users/username/projects/myapp/README.md",
				},
				ExpectedResult: "Updates the file directly, converting American spellings to British throughout the document and returns a summary of changes made",
			},
			{
				Description: "Convert text while preserving smart quotes",
				Arguments: map[string]any{
					"text":              "The program's behavior was optimized for the organization's needs.",
					"keep_smart_quotes": true,
				},
				ExpectedResult: "Converts spellings but keeps smart quotes intact: 'The program's behaviour was optimised for the organisation's needs.'",
			},
			{
				Description: "Convert text with smart quote normalisation",
				Arguments: map[string]any{
					"text":              "The program's behavior was \"optimized\" for the organization's needs.",
					"keep_smart_quotes": false,
				},
				ExpectedResult: "Converts spellings and normalises smart quotes to standard quotes: 'The program's behaviour was \"optimised\" for the organisation's needs.'",
			},
		},
		CommonPatterns: []string{
			"Use text parameter for quick inline conversions and previews",
			"Use file_path parameter to update documentation files in place",
			"Set keep_smart_quotes to true when working with formatted documents that use typographic quotes",
			"Test with text parameter first before updating important files",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Cannot provide both 'text' and 'file_path' parameters",
				Solution: "Choose either inline mode (text parameter) or file update mode (file_path parameter). Use text for quick conversions, file_path for updating files in place.",
			},
			{
				Problem:  "File path must be fully qualified absolute path",
				Solution: "Ensure file_path starts with / (Unix) or drive letter (Windows). Use complete paths like '/Users/username/project/file.md', not relative paths like './file.md'.",
			},
			{
				Problem:  "Text exceeds maximum length error",
				Solution: "The tool has configurable limits (default 40,000 characters). For larger texts, either increase M2E_MAX_LENGTH environment variable or process in chunks.",
			},
			{
				Problem:  "File content exceeds maximum length",
				Solution: "Large files are rejected for safety. Consider splitting the file or increasing the M2E_MAX_LENGTH environment variable if the file legitimately needs processing.",
			},
		},
		ParameterDetails: map[string]string{
			"text":              "Text to convert inline and return immediately. Cannot be used with file_path. Best for previews, testing, or small conversions where you need the result returned.",
			"file_path":         "Absolute path to file to update in place. File is read, converted, and written back only if changes are needed. Cannot be used with text parameter.",
			"keep_smart_quotes": "Whether to preserve smart quotes and em-dashes (true) or normalise them to standard ASCII quotes (false, default). Useful when working with formatted documents vs plain text.",
		},
		WhenToUse:    "Use for converting documentation, code comments, README files, or any text from American to British English spelling. Ideal for maintaining consistent language standards across international projects.",
		WhenNotToUse: "Don't use for code syntax, variable names, API endpoints, or technical identifiers. Not suitable for languages other than English or for complex linguistic transformations beyond spelling differences.",
	}
}
