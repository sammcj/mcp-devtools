package code_rename

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/sammcj/mcp-devtools/internal/security"
	"go.lsp.dev/protocol"
)

// NewRenameTransaction creates a new transaction for atomic rename operations
func NewRenameTransaction() (*RenameTransaction, error) {
	transactionID := uuid.New().String()
	backupDir := filepath.Join(os.TempDir(), fmt.Sprintf("mcp-rename-%s", transactionID))

	// Create backup directory with restricted permissions
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &RenameTransaction{
		backupDir:     backupDir,
		backups:       make(map[string]backupEntry),
		modified:      make([]string, 0),
		checksums:     make(map[string]string),
		transactionID: transactionID,
	}, nil
}

// PreflightCheck validates all files can be modified before starting transaction
func (tx *RenameTransaction) PreflightCheck(edit *protocol.WorkspaceEdit) error {
	// Collect all file paths from both edit formats
	filePaths := make(map[string]bool)

	// Legacy Changes format
	for uriStr := range edit.Changes {
		filePath := uriToPath(string(uriStr))
		filePaths[filePath] = true
	}

	// Modern DocumentChanges format
	for _, textDocEdit := range edit.DocumentChanges {
		filePath := uriToPath(string(textDocEdit.TextDocument.URI))
		filePaths[filePath] = true
	}

	// Validate each file
	for filePath := range filePaths {
		// Security check
		if err := security.CheckFileAccess(filePath); err != nil {
			return fmt.Errorf("pre-flight security check failed for %s: %w", filePath, err)
		}

		// Check file exists
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("pre-flight check failed for %s: %w", filePath, err)
		}

		// Check file is writable
		if fileInfo.Mode()&0200 == 0 {
			return fmt.Errorf("pre-flight check failed: %s is not writable", filePath)
		}

		// Check we can read the file
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("pre-flight check failed: cannot read %s: %w", filePath, err)
		}

		// Calculate and store checksum
		checksum := calculateChecksum(content)
		tx.checksums[filePath] = checksum
	}

	return nil
}

// BackupFile creates a backup of a file before modification
func (tx *RenameTransaction) BackupFile(filePath string) error {
	// Check if already backed up (idempotent)
	if _, exists := tx.backups[filePath]; exists {
		return nil // Already backed up
	}

	// Read original file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Get file info for permissions
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file for backup: %w", err)
	}

	// Create unique backup filename using counter to avoid collisions
	// This handles files with same basename from different directories
	counter := len(tx.backups)
	backupFileName := fmt.Sprintf("%d_%s.bak", counter, filepath.Base(filePath))
	backupPath := filepath.Join(tx.backupDir, backupFileName)

	// Write backup with restricted permissions
	if err := os.WriteFile(backupPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// Calculate checksum
	checksum := calculateChecksum(content)

	// Store backup info
	tx.backups[filePath] = backupEntry{
		backupPath: backupPath,
		checksum:   checksum,
		mode:       fileInfo.Mode().String(),
	}

	return nil
}

// ApplyWithTracking applies edits to a file and tracks the modification
func (tx *RenameTransaction) ApplyWithTracking(filePath string, edits []protocol.TextEdit) error {
	// Note: Large file size check (>2MB) could be added here if needed
	// Current implementation handles all file sizes - checksums provide safety

	// Backup file before modification
	if err := tx.BackupFile(filePath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Apply edits using existing function
	if err := applyEditsToFile(filePath, edits); err != nil {
		return err
	}

	// Track successful modification
	tx.modified = append(tx.modified, filePath)

	return nil
}

// Rollback restores all modified files from backups
func (tx *RenameTransaction) Rollback() ([]string, error) {
	reverted := make([]string, 0)
	var lastErr error

	// Restore files in reverse order of modification
	for i := len(tx.modified) - 1; i >= 0; i-- {
		filePath := tx.modified[i]
		backup, exists := tx.backups[filePath]
		if !exists {
			lastErr = fmt.Errorf("no backup found for %s", filePath)
			continue
		}

		// Read backup content
		backupContent, err := os.ReadFile(backup.backupPath)
		if err != nil {
			lastErr = fmt.Errorf("failed to read backup for %s: %w", filePath, err)
			continue
		}

		// Verify backup checksum
		if backup.checksum != "" {
			actualChecksum := calculateChecksum(backupContent)
			if actualChecksum != backup.checksum {
				lastErr = fmt.Errorf("backup checksum mismatch for %s", filePath)
				continue
			}
		}

		// Get original file info to preserve permissions
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			// File might not exist if it was created during operation
			fileInfo = nil
		}

		mode := os.FileMode(0600)
		if fileInfo != nil {
			mode = fileInfo.Mode()
		}

		// Restore file
		if err := os.WriteFile(filePath, backupContent, mode); err != nil {
			lastErr = fmt.Errorf("failed to restore %s: %w", filePath, err)
			continue
		}

		// Verify restored file checksum
		restoredContent, err := os.ReadFile(filePath)
		if err == nil {
			restoredChecksum := calculateChecksum(restoredContent)
			if restoredChecksum != backup.checksum {
				lastErr = fmt.Errorf("restored file checksum mismatch for %s", filePath)
				continue
			}
		}

		reverted = append(reverted, filePath)
	}

	if lastErr != nil {
		return reverted, fmt.Errorf("rollback completed with errors: %w", lastErr)
	}

	return reverted, nil
}

// Cleanup removes the backup directory on successful completion
func (tx *RenameTransaction) Cleanup() error {
	if tx.backupDir != "" {
		return os.RemoveAll(tx.backupDir)
	}
	return nil
}

// KeepBackups preserves the backup directory for debugging
func (tx *RenameTransaction) KeepBackups() string {
	return tx.backupDir
}

// calculateChecksum computes SHA256 checksum of content
func calculateChecksum(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// calculateFileChecksum computes SHA256 checksum of a file
func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// checkFileModificationTime verifies file hasn't been modified since analysis
func checkFileModificationTime(filePath string, originalChecksum string) error {
	currentChecksum, err := calculateFileChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate current checksum: %w", err)
	}

	if currentChecksum != originalChecksum {
		return fmt.Errorf("file modified since analysis: %s. Please retry", filePath)
	}

	return nil
}
