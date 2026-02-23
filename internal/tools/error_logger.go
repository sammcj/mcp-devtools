package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ToolErrorLogEntry represents a logged tool error
type ToolErrorLogEntry struct {
	Timestamp string         `json:"timestamp"`
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Error     string         `json:"error"`
	Transport string         `json:"transport,omitempty"`
}

// ToolErrorLogger handles logging of tool execution errors
type ToolErrorLogger struct {
	enabled  bool
	logFile  *os.File
	logger   *logrus.Logger
	mu       sync.Mutex
	filePath string
}

var (
	globalErrorLogger *ToolErrorLogger
	errorLoggerOnce   sync.Once
)

const (
	// DefaultLogRetentionDays is the default number of days to retain error logs
	DefaultLogRetentionDays = 60
)

// InitGlobalErrorLogger initialises the global error logger
func InitGlobalErrorLogger(logger *logrus.Logger) error {
	var initErr error
	errorLoggerOnce.Do(func() {
		// Check if error logging is enabled via environment variable
		enabled := os.Getenv("LOG_TOOL_ERRORS") == "true"

		if !enabled {
			globalErrorLogger = &ToolErrorLogger{
				enabled: false,
				logger:  logger,
			}
			return
		}

		// Determine log file path
		homeDir, err := os.UserHomeDir()
		if err != nil {
			initErr = fmt.Errorf("failed to get home directory: %w", err)
			return
		}

		logDir := filepath.Join(homeDir, ".mcp-devtools", "logs")
		if err := os.MkdirAll(logDir, 0700); err != nil {
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}

		logFilePath := filepath.Join(logDir, "tool-errors.log")

		// Open log file with append mode
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			initErr = fmt.Errorf("failed to open tool error log file: %w", err)
			return
		}

		globalErrorLogger = &ToolErrorLogger{
			enabled:  true,
			logFile:  logFile,
			logger:   logger,
			filePath: logFilePath,
		}

		// Perform log rotation in background to avoid blocking startup
		go func() {
			if rotateErr := globalErrorLogger.rotateOldLogs(); rotateErr != nil {
				logger.WithError(rotateErr).Warn("Failed to rotate old tool error logs")
			}
		}()

		// Log initialisation message
		logger.Infof("Tool error logging enabled: %s", logFilePath)
	})

	return initErr
}

// GetGlobalErrorLogger returns the global error logger instance
func GetGlobalErrorLogger() *ToolErrorLogger {
	if globalErrorLogger == nil {
		// Return a disabled logger if not initialised
		return &ToolErrorLogger{
			enabled: false,
		}
	}
	return globalErrorLogger
}

// LogToolError logs a tool execution error
func (l *ToolErrorLogger) LogToolError(toolName string, args map[string]any, err error, transport string) {
	if !l.enabled || l.logFile == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := ToolErrorLogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		ToolName:  toolName,
		Arguments: args,
		Error:     err.Error(),
		Transport: transport,
	}

	// Marshal to JSON
	jsonData, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		if l.logger != nil {
			l.logger.WithError(marshalErr).Error("Failed to marshal tool error log entry")
		}
		return
	}

	// Write to log file with newline
	if _, writeErr := l.logFile.Write(append(jsonData, '\n')); writeErr != nil {
		if l.logger != nil {
			l.logger.WithError(writeErr).Error("Failed to write tool error log entry")
		}
		return
	}

	// Sync to ensure data is written to disk
	if syncErr := l.logFile.Sync(); syncErr != nil {
		if l.logger != nil {
			l.logger.WithError(syncErr).Error("Failed to sync tool error log file")
		}
	}
}

// Close closes the error logger and its log file
func (l *ToolErrorLogger) Close() error {
	if !l.enabled || l.logFile == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.logFile.Close()
}

// IsEnabled returns whether error logging is enabled
func (l *ToolErrorLogger) IsEnabled() bool {
	return l.enabled
}

// GetLogFilePath returns the path to the error log file
func (l *ToolErrorLogger) GetLogFilePath() string {
	return l.filePath
}

// rotateOldLogs removes log entries older than the retention period.
// Safe to call from a goroutine -- holds the mutex for the entire operation
// to prevent LogToolError from writing to a closed file during rotation.
func (l *ToolErrorLogger) rotateOldLogs() error {
	if !l.enabled || l.filePath == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Close current log file
	if l.logFile != nil {
		if err := l.logFile.Close(); err != nil {
			return fmt.Errorf("failed to close log file for rotation: %w", err)
		}
		l.logFile = nil
	}

	// Read all log entries
	file, err := os.Open(l.filePath)
	if err != nil {
		// If file doesn't exist, just reopen and return
		return l.reopenLogFileLocked()
	}

	var validEntries []string
	cutoffTime := time.Now().AddDate(0, 0, -DefaultLogRetentionDays)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse the log entry to check its timestamp
		var entry ToolErrorLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Keep malformed entries to avoid data loss
			validEntries = append(validEntries, line)
			continue
		}

		// Parse the timestamp
		entryTime, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			// Keep entries with unparseable timestamps
			validEntries = append(validEntries, line)
			continue
		}

		// Only keep entries newer than the cutoff
		if entryTime.After(cutoffTime) {
			validEntries = append(validEntries, line)
		}
	}

	scanErr := scanner.Err()
	_ = file.Close()

	if scanErr != nil {
		_ = l.reopenLogFileLocked()
		return fmt.Errorf("error reading log file during rotation: %w", scanErr)
	}

	// Write back only valid entries using atomic file replacement
	tmpPath := l.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(strings.Join(validEntries, "\n")+"\n"), 0600); err != nil {
		_ = l.reopenLogFileLocked()
		return fmt.Errorf("failed to write temporary rotated log file: %w", err)
	}

	// Atomically replace the original log file with the temporary file
	if err := os.Rename(tmpPath, l.filePath); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file on failure
		_ = l.reopenLogFileLocked()
		return fmt.Errorf("failed to rename temporary log file during rotation: %w", err)
	}

	// Reopen the log file in append mode
	return l.reopenLogFileLocked()
}

// reopenLogFileLocked reopens the log file in append mode.
// Caller must hold l.mu.
func (l *ToolErrorLogger) reopenLogFileLocked() error {
	logFile, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to reopen log file: %w", err)
	}

	l.logFile = logFile
	return nil
}
