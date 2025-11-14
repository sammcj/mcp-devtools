package code_rename

import (
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// ProcessTracker tracks LSP server processes for cleanup
type ProcessTracker struct {
	mu        sync.Mutex
	processes map[int]*trackedProcess // PID -> process info
	logger    *logrus.Logger
}

// trackedProcess stores information about a tracked process
type trackedProcess struct {
	pid       int
	command   string
	startTime time.Time
	process   *os.Process
}

var (
	globalTracker     *ProcessTracker
	globalTrackerOnce sync.Once
)

// GetProcessTracker returns the global process tracker instance
func GetProcessTracker(logger *logrus.Logger) *ProcessTracker {
	globalTrackerOnce.Do(func() {
		// Ensure we always have a valid logger
		if logger == nil {
			logger = logrus.New()
			logger.SetOutput(io.Discard) // Silent logger if none provided
		}
		globalTracker = &ProcessTracker{
			processes: make(map[int]*trackedProcess),
			logger:    logger,
		}
	})
	return globalTracker
}

// Register registers a process for tracking
func (pt *ProcessTracker) Register(pid int, command string, process *os.Process) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.processes[pid] = &trackedProcess{
		pid:       pid,
		command:   command,
		startTime: time.Now(),
		process:   process,
	}

	pt.logger.WithFields(logrus.Fields{
		"pid":     pid,
		"command": command,
	}).Debug("Registered LSP server process")
}

// Deregister removes a process from tracking
func (pt *ProcessTracker) Deregister(pid int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if proc, exists := pt.processes[pid]; exists {
		pt.logger.WithFields(logrus.Fields{
			"pid":     pid,
			"command": proc.command,
		}).Debug("Deregistered LSP server process")
		delete(pt.processes, pid)
	}
}

// CleanupOrphaned kills any orphaned LSP server processes
func (pt *ProcessTracker) CleanupOrphaned() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for pid, proc := range pt.processes {
		pt.logger.WithFields(logrus.Fields{
			"pid":     pid,
			"command": proc.command,
			"age":     time.Since(proc.startTime),
		}).Warn("Cleaning up orphaned LSP server process")

		if err := pt.killProcess(proc); err != nil {
			pt.logger.WithError(err).WithField("pid", pid).Error("Failed to kill orphaned process")
		}
	}

	// Clear all tracked processes
	pt.processes = make(map[int]*trackedProcess)
}

// killProcess attempts to gracefully kill a process, then forcefully if needed
func (pt *ProcessTracker) killProcess(proc *trackedProcess) error {
	if proc.process == nil {
		return nil
	}

	// Try SIGTERM first (graceful shutdown)
	if err := proc.process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		return nil
	}

	// Wait briefly for graceful shutdown
	done := make(chan error, 1)
	go func() {
		_, err := proc.process.Wait()
		done <- err
	}()

	select {
	case <-time.After(2 * time.Second):
		// Timeout - force kill with SIGKILL
		pt.logger.WithField("pid", proc.pid).Debug("Graceful shutdown timeout, sending SIGKILL")
		if err := proc.process.Kill(); err != nil {
			return err
		}
		// Wait for process to actually die
		_, _ = proc.process.Wait()
	case <-done:
		// Process exited gracefully
		pt.logger.WithField("pid", proc.pid).Debug("Process exited gracefully")
	}

	return nil
}

// CleanupStale kills processes that have been running too long (>5 minutes)
func (pt *ProcessTracker) CleanupStale(maxAge time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()
	for pid, proc := range pt.processes {
		age := now.Sub(proc.startTime)
		if age > maxAge {
			pt.logger.WithFields(logrus.Fields{
				"pid":     pid,
				"command": proc.command,
				"age":     age,
			}).Warn("Cleaning up stale LSP server process")

			if err := pt.killProcess(proc); err != nil {
				pt.logger.WithError(err).WithField("pid", pid).Error("Failed to kill stale process")
			}

			delete(pt.processes, pid)
		}
	}
}
