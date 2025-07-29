package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/replacement/replace_in_all_files"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceInAllFilesTool_Definition(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	def := tool.Definition()

	assert.Equal(t, "replace_in_all_files", def.Name)
	assert.NotEmpty(t, def.Description)
	assert.Contains(t, def.Description, "Efficiently and accurately find and replace")
	assert.Contains(t, def.Description, "EXACT matches")
}

func TestReplaceInAllFilesTool_ParseRequest(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid request",
			args: map[string]interface{}{
				"path": tempDir,
				"replacement_pairs": []interface{}{
					map[string]interface{}{
						"source": "old",
						"target": "new",
					},
				},
				"dry_run": true,
			},
			wantErr: false,
		},
		{
			name: "missing path",
			args: map[string]interface{}{
				"replacement_pairs": []interface{}{
					map[string]interface{}{
						"source": "old",
						"target": "new",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "non-absolute path",
			args: map[string]interface{}{
				"path": "relative/path",
				"replacement_pairs": []interface{}{
					map[string]interface{}{
						"source": "old",
						"target": "new",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing replacement_pairs",
			args: map[string]interface{}{
				"path": tempDir,
			},
			wantErr: true,
		},
		{
			name: "empty replacement_pairs",
			args: map[string]interface{}{
				"path":              tempDir,
				"replacement_pairs": []interface{}{},
			},
			wantErr: true,
		},
		{
			name: "invalid replacement pair - missing source",
			args: map[string]interface{}{
				"path": tempDir,
				"replacement_pairs": []interface{}{
					map[string]interface{}{
						"target": "new",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid replacement pair - missing target",
			args: map[string]interface{}{
				"path": tempDir,
				"replacement_pairs": []interface{}{
					map[string]interface{}{
						"source": "old",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate source strings",
			args: map[string]interface{}{
				"path": tempDir,
				"replacement_pairs": []interface{}{
					map[string]interface{}{
						"source": "old",
						"target": "new1",
					},
					map[string]interface{}{
						"source": "old",
						"target": "new2",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), logger, &sync.Map{}, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// For valid requests, we might get other errors but not parsing errors
				// The specific error type would depend on the test setup
				assert.NoError(t, err)
			}
		})
	}
}

func TestReplaceInAllFilesTool_DryRun(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "This is old text with old values"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "old",
				"target": "new",
			},
		},
		"dry_run": true,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify dry run didn't modify the file
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))

	// Verify result indicates what would be changed
	assert.NotNil(t, result)
}

func TestReplaceInAllFilesTool_ActualReplacement(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "This is old text with old values"
	expectedContent := "This is new text with new values"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "old",
				"target": "new",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify file was modified
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	// Verify result indicates changes were made
	assert.NotNil(t, result)
}

func TestReplaceInAllFilesTool_MultipleReplacements(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello world! Goodbye world!"
	expectedContent := "Hi universe! Farewell universe!"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "Hello",
				"target": "Hi",
			},
			map[string]interface{}{
				"source": "Goodbye",
				"target": "Farewell",
			},
			map[string]interface{}{
				"source": "world",
				"target": "universe",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify file was modified
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	// Verify result
	assert.NotNil(t, result)
}

func TestReplaceInAllFilesTool_SpecialCharacters(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file with special characters
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := `function test() { return "hello"; } $var = 'test'; [brackets] and (parentheses)`
	expectedContent := `func test() { return "hi"; } $variable = 'test'; [brackets] and (parentheses)`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "function",
				"target": "func",
			},
			map[string]interface{}{
				"source": "\"hello\"",
				"target": "\"hi\"",
			},
			map[string]interface{}{
				"source": "$var",
				"target": "$variable",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify file was modified correctly
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	// Verify result
	assert.NotNil(t, result)
}

func TestReplaceInAllFilesTool_NoMatches(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "This file has no matching text"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "nonexistent",
				"target": "replacement",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify file was not modified
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))

	// Verify result indicates no changes
	assert.NotNil(t, result)
}

func TestReplaceInAllFilesTool_BinaryFileSkipping(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a binary file (contains null bytes)
	binaryFile := filepath.Join(tempDir, "binary.bin")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 'h', 'e', 'l', 'l', 'o', 0x00}
	err = os.WriteFile(binaryFile, binaryContent, 0644)
	require.NoError(t, err)

	// Create a text file
	textFile := filepath.Join(tempDir, "text.txt")
	textContent := "hello world"
	expectedTextContent := "hi world"
	err = os.WriteFile(textFile, []byte(textContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "hello",
				"target": "hi",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify binary file was not modified
	content, err := os.ReadFile(binaryFile)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, content)

	// Verify text file was modified
	content, err = os.ReadFile(textFile)
	require.NoError(t, err)
	assert.Equal(t, expectedTextContent, string(content))

	// Verify result
	assert.NotNil(t, result)
}

func TestReplaceInAllFilesTool_SingleFile(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory and file
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "old content"
	expectedContent := "new content"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Test with single file path instead of directory
	args := map[string]interface{}{
		"path": testFile, // Point to file, not directory
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "old",
				"target": "new",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify file was modified
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	// Verify result
	assert.NotNil(t, result)
}

func TestReplaceInAllFilesTool_MixedQuotesAndSpecialCharacters(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file with mixed quotes, special characters, and edge cases
	testFile := filepath.Join(tempDir, "mixed_quotes.txt")
	testContent := `Testing mixed quotes and special characters:
Single quotes: 'old_value' and "double_quotes"
Mixed: 'single with "double" inside' and "double with 'single' inside"
Dollar vars: $OLD_VAR and ${OLD_VAR}
Regex-like: .*pattern.*, ^start$, end\n, [brackets], {braces}
JSON: {"old_key": "old_value", 'mixed': 'style'}
Code: function old_func() { return "old_string"; }
URLs: https://old-domain.com/path?param=old_value
Escaped: \"escaped_old\" and \'escaped_old\'
Special symbols: @old_symbol, #old_tag, %old_percent%
Newlines and tabs:	old_with_tab
Line with old_value at end`

	expectedContent := `Testing mixed quotes and special characters:
Single quotes: 'new_value' and "double_quotes"
Mixed: 'single with "double" inside' and "double with 'single' inside"
Dollar vars: $NEW_VAR and ${NEW_VAR}
Regex-like: .*pattern.*, ^start$, end\n, [brackets], {braces}
JSON: {"new_key": "new_value", 'mixed': 'style'}
Code: function new_func() { return "new_string"; }
URLs: https://new-domain.com/path?param=new_value
Escaped: \"escaped_new\" and \'escaped_new\'
Special symbols: @new_symbol, #new_tag, %new_percent%
Newlines and tabs:	new_with_tab
Line with new_value at end`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "old_value",
				"target": "new_value",
			},
			map[string]interface{}{
				"source": "$OLD_VAR",
				"target": "$NEW_VAR",
			},
			map[string]interface{}{
				"source": "${OLD_VAR}",
				"target": "${NEW_VAR}",
			},
			map[string]interface{}{
				"source": "old_key",
				"target": "new_key",
			},
			map[string]interface{}{
				"source": "old_func",
				"target": "new_func",
			},
			map[string]interface{}{
				"source": "old_string",
				"target": "new_string",
			},
			map[string]interface{}{
				"source": "old-domain.com",
				"target": "new-domain.com",
			},
			map[string]interface{}{
				"source": "escaped_old",
				"target": "escaped_new",
			},
			map[string]interface{}{
				"source": "@old_symbol",
				"target": "@new_symbol",
			},
			map[string]interface{}{
				"source": "#old_tag",
				"target": "#new_tag",
			},
			map[string]interface{}{
				"source": "%old_percent%",
				"target": "%new_percent%",
			},
			map[string]interface{}{
				"source": "old_with_tab",
				"target": "new_with_tab",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify file was modified correctly
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	// Verify result indicates changes were made
	assert.NotNil(t, result)

	// Parse the JSON result to verify replacement counts
	textContent := result.Content[0].(mcp.TextContent)
	resultText := textContent.Text
	var response map[string]interface{}
	err = json.Unmarshal([]byte(resultText), &response)
	require.NoError(t, err)

	// Check that files were processed
	filesProcessed := response["files_processed"].([]interface{})
	assert.Len(t, filesProcessed, 1)

	// Check that modifications were made
	modifiedFiles := int(response["modified_files"].(float64))
	assert.Equal(t, 1, modifiedFiles)

	// Check summary indicates success
	summary := response["summary"].(string)
	assert.Contains(t, summary, "Successfully modified 1 files")

	// Verify specific replacement counts in the processed file
	processedFile := filesProcessed[0].(map[string]interface{})
	replacementCount := processedFile["replacement_count"].(map[string]interface{})

	// Verify some key replacements were counted correctly
	assert.Contains(t, replacementCount, "old_value")
	assert.Equal(t, float64(4), replacementCount["old_value"]) // Should find 4 occurrences

	assert.Contains(t, replacementCount, "$OLD_VAR")
	assert.Equal(t, float64(1), replacementCount["$OLD_VAR"])

	assert.Contains(t, replacementCount, "${OLD_VAR}")
	assert.Equal(t, float64(1), replacementCount["${OLD_VAR}"])
}

func TestReplaceInAllFilesTool_StringReplacementBehavior(t *testing.T) {
	tool := &replaceinallfiles.ReplaceInAllFilesTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "replace_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file to verify string replacement behavior (includes substring replacements)
	testFile := filepath.Join(tempDir, "string_replacement.txt")
	testContent := `Testing string replacement behavior:
cat, cats, catch, category
dog, dogs, dogma, dogmatic
test, testing, tester, tests
exact, exactly, exactness`

	// String replacement will replace ALL occurrences, including within words
	expectedContent := `Testing string replacement behavior:
kitten, kittens, kittench, kittenegory
puppy, puppys, puppyma, puppymatic
exam, examing, examer, exams
precise, precisely, preciseness`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	args := map[string]interface{}{
		"path": tempDir,
		"replacement_pairs": []interface{}{
			map[string]interface{}{
				"source": "cat",
				"target": "kitten",
			},
			map[string]interface{}{
				"source": "dog",
				"target": "puppy",
			},
			map[string]interface{}{
				"source": "test",
				"target": "exam",
			},
			map[string]interface{}{
				"source": "exact",
				"target": "precise",
			},
		},
		"dry_run": false,
	}

	result, err := tool.Execute(context.Background(), logger, &sync.Map{}, args)
	require.NoError(t, err)

	// Verify file was modified correctly - ALL string occurrences should be replaced
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	// Verify that substring replacements occurred (this is the expected behavior)
	assert.Contains(t, string(content), "kittens")   // "cats" became "kittens" (cat -> kitten)
	assert.Contains(t, string(content), "kittench")  // "catch" became "kittench" (cat -> kitten)
	assert.Contains(t, string(content), "puppys")    // "dogs" became "puppys" (dog -> puppy)
	assert.Contains(t, string(content), "examing")   // "testing" became "examing" (test -> exam)
	assert.Contains(t, string(content), "precisely") // "exactly" became "precisely" (exact -> precise)

	// Verify exact matches were also replaced
	assert.Contains(t, string(content), "kitten,")  // standalone "cat" was replaced
	assert.Contains(t, string(content), "puppy,")   // standalone "dog" was replaced
	assert.Contains(t, string(content), "exam,")    // standalone "test" was replaced
	assert.Contains(t, string(content), "precise,") // standalone "exact" was replaced

	// Verify result
	assert.NotNil(t, result)
}
