package projectactions

import "fmt"

// ErrorType represents the category of error
type ErrorType int

const (
	ErrorInvalidDirectory ErrorType = iota
	ErrorToolNotFound
	ErrorInvalidTarget
	ErrorInvalidPath
	ErrorCommandFailed
	ErrorMakefileInvalid
	ErrorCommitTooLarge
)

// ProjectActionsError represents an error from the project actions tool
type ProjectActionsError struct {
	Type    ErrorType
	Message string
	Cause   error
}

func (e *ProjectActionsError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ProjectActionsError) Unwrap() error {
	return e.Cause
}

// Error message constants
const (
	ErrMsgSystemDir       = "working directory cannot be a system directory: %s"
	ErrMsgNotWritable     = "working directory not writable by current user: %s"
	ErrMsgMakeNotFound    = "make not found on PATH - install make to use this tool"
	ErrMsgGitNotFound     = "git not found on PATH - install git to use git operations"
	ErrMsgInvalidTarget   = "target '%s' not found in Makefile .PHONY targets"
	ErrMsgPathEscape      = "path escapes working directory: %s"
	ErrMsgCommitTooLarge  = "commit message exceeds %d KB limit (use PROJECT_ACTIONS_MAX_COMMIT_SIZE to adjust)"
	ErrMsgGitFailed       = "git command failed - you may need to run 'git init' or specify another directory"
)
