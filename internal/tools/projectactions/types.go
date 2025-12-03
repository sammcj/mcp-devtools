package projectactions

import "time"

// CommandResult contains the result of a command execution
type CommandResult struct {
	Command    string        `json:"command"`
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	ExitCode   int           `json:"exit_code"`
	Duration   time.Duration `json:"duration"`
	WorkingDir string        `json:"working_dir"`
}

// ToolArgs represents the input parameters for the tool
type ToolArgs struct {
	Operation  string   `json:"operation"`
	WorkingDir string   `json:"working_directory,omitempty"`
	Paths      []string `json:"paths,omitempty"`
	Message    string   `json:"message,omitempty"`
	Language   string   `json:"language,omitempty"`
	DryRun     bool     `json:"dry_run,omitempty"`
}
