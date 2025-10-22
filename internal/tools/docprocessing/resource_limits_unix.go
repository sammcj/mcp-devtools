//go:build !windows

package docprocessing

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// setProcessResourceLimits sets resource limits for a subprocess
// This helps prevent runaway memory usage in document processing
func setProcessResourceLimits(cmd *exec.Cmd, maxMemoryBytes int64) error {
	if maxMemoryBytes <= 0 {
		return nil // No limit set
	}

	// Set up SysProcAttr if not already initialised
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Set resource limits using setrlimit
	// We'll use Setpgid to create a new process group, which helps with cleanup
	cmd.SysProcAttr.Setpgid = true

	// Note: We can't directly set RLIMIT_AS or RLIMIT_DATA in Go's SysProcAttr
	// Instead, we'll use a preexec approach by setting environment variables
	// that the Python process can read and apply itself, or we use the shell wrapper

	// For now, we'll rely on the Python script to honour the memory limit
	// through environment variables
	if cmd.Env == nil {
		// Defensive: if somehow Env is nil, use current environment as base
		cmd.Env = os.Environ()
	}

	// Set environment variable for Python to read
	cmd.Env = append(cmd.Env, fmt.Sprintf("DOCLING_MAX_MEMORY_LIMIT=%d", maxMemoryBytes))

	return nil
}
