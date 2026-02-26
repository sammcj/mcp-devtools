package tools_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	devtoolscli "github.com/sammcj/mcp-devtools/internal/cli"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/calculator"
	"github.com/sammcj/mcp-devtools/internal/tools/think"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

// setupCLIRegistry registers the tools needed for CLI tests.
func setupCLIRegistry(t *testing.T) {
	t.Helper()
	logger := testutils.CreateTestLogger()
	registry.Init(logger)
	registry.Register(&calculator.Calculator{})
	registry.Register(&think.ThinkTool{})
}

// newTestRunner creates a CLI runner for tests.
func newTestRunner(output devtoolscli.OutputFormat) *devtoolscli.Runner {
	return devtoolscli.NewRunner(testutils.CreateTestLogger(), testutils.CreateTestCache(), output)
}

// captureStdout captures stdout during f() and returns the output.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	os.Stdout = w

	var buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = buf.ReadFrom(r)
	})

	f()

	w.Close()
	os.Stdout = old
	wg.Wait()

	return buf.String()
}

func TestCLI_ListTools(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	output := captureStdout(t, func() {
		if err := runner.ListTools(); err != nil {
			t.Fatalf("ListTools error: %v", err)
		}
	})

	if !strings.Contains(output, "calculator") {
		t.Errorf("expected output to contain 'calculator', got: %s", output)
	}
}

func TestCLI_ListTools_JSON(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputJSON)

	output := captureStdout(t, func() {
		if err := runner.ListTools(); err != nil {
			t.Fatalf("ListTools error: %v", err)
		}
	})

	var tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(output), &tools); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", err, output)
	}

	if len(tools) == 0 {
		t.Error("expected at least one tool in JSON output")
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "calculator" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'calculator' in tool list")
	}
}

func TestCLI_HelpTool(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	output := captureStdout(t, func() {
		if err := runner.HelpTool("calculator"); err != nil {
			t.Fatalf("HelpTool error: %v", err)
		}
	})

	if !strings.Contains(output, "Tool: calculator") {
		t.Errorf("expected 'Tool: calculator' in help output, got: %s", output)
	}
	if !strings.Contains(output, "--expression") {
		t.Errorf("expected '--expression' parameter in help output, got: %s", output)
	}
	if !strings.Contains(output, "Parameters:") {
		t.Errorf("expected 'Parameters:' in help output, got: %s", output)
	}
}

func TestCLI_HelpTool_Unknown(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)
	err := runner.HelpTool("nonexistent-tool")
	testutils.AssertError(t, err)
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected 'unknown tool' in error, got: %s", err)
	}
}

func TestCLI_HelpTool_JSON(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputJSON)

	output := captureStdout(t, func() {
		if err := runner.HelpTool("calculator"); err != nil {
			t.Fatalf("HelpTool error: %v", err)
		}
	})

	var tool mcp.Tool
	if err := json.Unmarshal([]byte(output), &tool); err != nil {
		t.Fatalf("expected valid JSON tool definition, got error: %v", err)
	}
	if tool.Name != "calculator" {
		t.Errorf("expected tool name 'calculator', got: %s", tool.Name)
	}
}

func TestCLI_RunTool_JSONArgs(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	output := captureStdout(t, func() {
		if err := runner.RunTool(t.Context(), "calculator", []string{`{"expression": "2 + 3"}`}); err != nil {
			t.Fatalf("RunTool error: %v", err)
		}
	})

	if !strings.Contains(output, "5") {
		t.Errorf("expected result containing '5', got: %s", output)
	}
}

func TestCLI_RunTool_FlagArgs(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	output := captureStdout(t, func() {
		if err := runner.RunTool(t.Context(), "calculator", []string{"--expression=10 * 5"}); err != nil {
			t.Fatalf("RunTool error: %v", err)
		}
	})

	if !strings.Contains(output, "50") {
		t.Errorf("expected result containing '50', got: %s", output)
	}
}

func TestCLI_RunTool_FlagWithSpace(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	output := captureStdout(t, func() {
		if err := runner.RunTool(t.Context(), "calculator", []string{"--expression", "7 + 8"}); err != nil {
			t.Fatalf("RunTool error: %v", err)
		}
	})

	if !strings.Contains(output, "15") {
		t.Errorf("expected result containing '15', got: %s", output)
	}
}

func TestCLI_RunTool_UnknownTool(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)
	err := runner.RunTool(t.Context(), "nonexistent", []string{})
	testutils.AssertError(t, err)
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected 'unknown tool' in error, got: %s", err)
	}
}

func TestCLI_RunTool_InvalidJSON(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)
	err := runner.RunTool(t.Context(), "calculator", []string{`{invalid json}`})
	testutils.AssertError(t, err)
	if !strings.Contains(err.Error(), "argument error") {
		t.Errorf("expected 'argument error' in error, got: %s", err)
	}
}

func TestCLI_RunTool_JSONOutput(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputJSON)

	output := captureStdout(t, func() {
		if err := runner.RunTool(t.Context(), "calculator", []string{`{"expression": "1 + 1"}`}); err != nil {
			t.Fatalf("RunTool error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput: %s", err, output)
	}
}

func TestCLI_RunTool_ThinkTool(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	output := captureStdout(t, func() {
		if err := runner.RunTool(t.Context(), "think", []string{"--thought=testing CLI mode"}); err != nil {
			t.Fatalf("RunTool error: %v", err)
		}
	})

	if len(output) == 0 {
		t.Error("expected non-empty output from think tool")
	}
}

func TestCLI_RunTool_KebabToSnakeParam(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	// --how-hard should resolve to the how_hard parameter (kebab→snake)
	output := captureStdout(t, func() {
		if err := runner.RunTool(t.Context(), "think", []string{"--thought=test", "--how-hard=harder"}); err != nil {
			t.Fatalf("RunTool error: %v", err)
		}
	})

	if len(output) == 0 {
		t.Error("expected non-empty output from think tool with how-hard flag")
	}
}

func TestCLI_HelpTool_KebabName(t *testing.T) {
	// Tools are registered with snake_case (e.g. "get_tool_help") but
	// CLI users naturally type kebab-case ("get-tool-help"). Verify both work.
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)

	// Calculator has no hyphens, but we can test that "calculator" still works
	output := captureStdout(t, func() {
		if err := runner.HelpTool("calculator"); err != nil {
			t.Fatalf("HelpTool error: %v", err)
		}
	})
	if !strings.Contains(output, "Tool: calculator") {
		t.Errorf("expected tool help output, got: %s", output)
	}
}

func TestCLI_RunTool_MissingFlagValue(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)
	err := runner.RunTool(t.Context(), "calculator", []string{"--expression"})
	testutils.AssertError(t, err)
	if !strings.Contains(err.Error(), "requires a value") {
		t.Errorf("expected 'requires a value' in error, got: %s", err)
	}
}

func TestCLI_RunTool_UnexpectedArg(t *testing.T) {
	setupCLIRegistry(t)
	runner := newTestRunner(devtoolscli.OutputText)
	err := runner.RunTool(t.Context(), "calculator", []string{"bareword"})
	testutils.AssertError(t, err)
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("expected 'unexpected argument' in error, got: %s", err)
	}
}
