package projectactions

import "github.com/sammcj/mcp-devtools/internal/tools"

// ProvideExtendedInfo provides detailed usage information for the project_actions tool
func (t *ProjectActionsTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		WhenToUse: "Use this tool to execute project development tasks (tests, linters, formatters) and git operations through a project's Makefile. Ideal for running project-specific commands without knowing the exact tooling setup.",
		WhenNotToUse: "Don't use for arbitrary shell commands, file operations outside the project, or when you need to modify Makefile content. Requires security tool to be enabled.",
		Examples: []tools.ToolExample{
			{
				Description: "Run tests using Makefile target",
				Arguments: map[string]any{
					"operation": "test",
				},
				ExpectedResult: "Executes 'make test' and returns output with exit code",
			},
			{
				Description: "Stage multiple files for commit",
				Arguments: map[string]any{
					"operation": "add",
					"paths":     []string{"file1.go", "file2.go"},
				},
				ExpectedResult: "Executes 'git add' for specified files",
			},
			{
				Description: "Create a commit with message",
				Arguments: map[string]any{
					"operation": "commit",
					"message":   "Fix bug in authentication",
				},
				ExpectedResult: "Creates git commit with provided message",
			},
			{
				Description: "Generate Makefile for Python project",
				Arguments: map[string]any{
					"operation": "generate",
					"language":  "python",
				},
				ExpectedResult: "Creates Makefile with Python targets (test, lint, format, fix)",
			},
			{
				Description: "Preview command without execution",
				Arguments: map[string]any{
					"operation": "test",
					"dry_run":   true,
				},
				ExpectedResult: "Shows command that would be executed without running it",
			},
		},
		CommonPatterns: []string{
			"Run 'generate' first if no Makefile exists",
			"Use 'add' then 'commit' for git workflow",
			"Use dry_run to preview commands before execution",
			"Specify working_directory to operate in different project",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Error: make not found on PATH",
				Solution: "Install make: apt-get install make (Debian/Ubuntu), brew install make (macOS), or yum install make (RHEL/CentOS)",
			},
			{
				Problem:  "Error: git not found on PATH",
				Solution: "Install git: apt-get install git (Debian/Ubuntu), brew install git (macOS), or yum install git (RHEL/CentOS)",
			},
			{
				Problem:  "Error: target 'X' not found in Makefile .PHONY targets",
				Solution: "Check Makefile for available .PHONY targets, or use 'generate' operation to create a new Makefile",
			},
			{
				Problem:  "Error: git command failed - you may need to run 'git init'",
				Solution: "Initialize git repository with 'git init' or specify a different working_directory that contains a git repository",
			},
			{
				Problem:  "Error: path escapes working directory",
				Solution: "Use relative paths within the project directory. Paths like '../file' are blocked for security",
			},
			{
				Problem:  "Error: commit message exceeds KB limit",
				Solution: "Reduce commit message size or set PROJECT_ACTIONS_MAX_COMMIT_SIZE environment variable to increase limit",
			},
			{
				Problem:  "Error: working directory cannot be a system directory",
				Solution: "Specify a project directory, not system directories like /, /bin, /usr, /etc",
			},
		},
		ParameterDetails: map[string]string{
			"operation":         "Required. Either a .PHONY target from Makefile, 'add', 'commit', or 'generate'",
			"working_directory": "Optional. Project directory (default: current directory). Must be writable by current user",
			"paths":             "Required for 'add' operation. Array of relative file paths to stage",
			"message":           "Required for 'commit' operation. Commit message (max 16KB by default)",
			"language":          "Required for 'generate' operation. One of: python, rust, go, nodejs",
			"dry_run":           "Optional. Preview commands without execution (default: false)",
		},
	}
}
