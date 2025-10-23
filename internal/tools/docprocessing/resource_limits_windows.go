//go:build windows

package docprocessing

import (
	"fmt"
	"os"
	"os/exec"
)

// setProcessResourceLimits sets resource limits for a subprocess
// This helps prevent runaway memory usage in document processing
func setProcessResourceLimits(cmd *exec.Cmd, maxMemoryBytes int64) error {
	if maxMemoryBytes <= 0 {
		return nil // No limit set
	}

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
