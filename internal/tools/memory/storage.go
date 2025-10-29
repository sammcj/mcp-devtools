package memory

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/sirupsen/logrus"
)

const (
	// Memory storage security limits
	DefaultMaxStorageSize          = int64(500 * 1024 * 1024) // 500MB default storage limit
	DefaultDataRetentionDays       = 180                      // 180 days default retention
	MemoryMaxStorageSizeEnvVar     = "MEMORY_MAX_STORAGE_SIZE"
	MemoryDataRetentionDaysEnvVar  = "MEMORY_DATA_RETENTION_DAYS"
	MemoryEncryptionPasswordEnvVar = "MEMORY_ENCRYPTION_PASSWORD"
)

// Storage handles file I/O operations for the knowledge graph
type Storage struct {
	basePath           string
	namespace          string
	filePath           string
	logger             *logrus.Logger
	maxStorageSize     int64
	dataRetentionDays  int
	encryptionPassword string
	encryptionEnabled  bool
}

// NewStorage creates a new storage instance with the configured file path and default namespace
func NewStorage(logger *logrus.Logger) (*Storage, error) {
	return NewStorageWithNamespace(logger, "default")
}

// NewStorageWithNamespace creates a new storage instance with the specified namespace
func NewStorageWithNamespace(logger *logrus.Logger, namespace string) (*Storage, error) {
	basePath, err := getMemoryBasePath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine memory base path: %w", err)
	}

	storage := &Storage{
		basePath:  basePath,
		namespace: namespace,
		logger:    logger,
	}

	// Load security configuration
	storage.loadSecurityConfig()

	// Update file path for the namespace
	if err := storage.updateFilePath(); err != nil {
		return nil, fmt.Errorf("failed to update file path: %w", err)
	}

	return storage, nil
}

// loadSecurityConfig loads security configuration from environment variables
func (s *Storage) loadSecurityConfig() {
	// Load max storage size
	s.maxStorageSize = DefaultMaxStorageSize
	if sizeStr := os.Getenv(MemoryMaxStorageSizeEnvVar); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size > 0 {
			s.maxStorageSize = size
		}
	}

	// Load data retention days
	s.dataRetentionDays = DefaultDataRetentionDays
	if daysStr := os.Getenv(MemoryDataRetentionDaysEnvVar); daysStr != "" {
		if days, err := strconv.Atoi(daysStr); err == nil && days > 0 {
			s.dataRetentionDays = days
		}
	}

	// Load encryption configuration
	s.encryptionPassword = os.Getenv(MemoryEncryptionPasswordEnvVar)
	s.encryptionEnabled = s.encryptionPassword != ""
}

// encrypt encrypts data using AES-GCM with the configured password
func (s *Storage) encrypt(data []byte) ([]byte, error) {
	if !s.encryptionEnabled {
		return data, nil
	}

	// Create a key from the password using SHA-256
	key := sha256.Sum256([]byte(s.encryptionPassword))

	// Create AES cipher
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM with the configured password
func (s *Storage) decrypt(data []byte) ([]byte, error) {
	if !s.encryptionEnabled {
		return data, nil
	}

	// Create a key from the password using SHA-256
	key := sha256.Sum256([]byte(s.encryptionPassword))

	// Create AES cipher
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce and ciphertext
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// validateStorageSize validates that the storage size is within limits
func (s *Storage) validateStorageSize() error {
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil // File doesn't exist yet, no size to validate
	}

	fileInfo, err := os.Stat(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to stat memory file: %w", err)
	}

	if fileInfo.Size() > s.maxStorageSize {
		sizeMB := float64(fileInfo.Size()) / (1024 * 1024)
		maxSizeMB := float64(s.maxStorageSize) / (1024 * 1024)
		return fmt.Errorf("memory storage size %.1fMB exceeds maximum allowed size of %.1fMB (use %s environment variable to adjust limit)", sizeMB, maxSizeMB, MemoryMaxStorageSizeEnvVar)
	}

	return nil
}

// checkDataRetention checks for old data and provides cleanup notifications
func (s *Storage) checkDataRetention() error {
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil // File doesn't exist yet
	}

	fileInfo, err := os.Stat(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to stat memory file: %w", err)
	}

	// Check if file is older than retention period
	retentionDuration := time.Duration(s.dataRetentionDays) * 24 * time.Hour
	if time.Since(fileInfo.ModTime()) > retentionDuration {
		s.logger.WithFields(logrus.Fields{
			"file_path":      s.filePath,
			"age_days":       int(time.Since(fileInfo.ModTime()).Hours() / 24),
			"retention_days": s.dataRetentionDays,
			"namespace":      s.namespace,
		}).Warn("Memory data exceeds retention period - consider cleanup")
	}

	return nil
}

// SetNamespace changes the namespace for this storage instance
func (s *Storage) SetNamespace(namespace string) error {
	s.namespace = namespace
	return s.updateFilePath()
}

// updateFilePath updates the file path based on current namespace
func (s *Storage) updateFilePath() error {
	// Create namespace-specific path
	namespaceDir := filepath.Join(s.basePath, s.namespace)
	s.filePath = filepath.Join(namespaceDir, "memory.json")

	// Ensure the namespace directory exists
	if err := ensureDirectoryExists(namespaceDir); err != nil {
		return fmt.Errorf("failed to create namespace directory: %w", err)
	}

	return nil
}

// getMemoryBasePath determines the base memory directory path from environment or default
func getMemoryBasePath() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv("MEMORY_FILE_PATH"); envPath != "" {
		if filepath.IsAbs(envPath) {
			// If it's a file path, return the directory
			if filepath.Ext(envPath) != "" {
				return filepath.Dir(envPath), nil
			}
			// If it's already a directory, use it as base
			return envPath, nil
		}
		// If relative path, make it relative to current working directory
		absPath, err := filepath.Abs(envPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve relative path: %w", err)
		}
		if filepath.Ext(absPath) != "" {
			return filepath.Dir(absPath), nil
		}
		return absPath, nil
	}

	// Default to ~/.mcp-devtools/
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	return filepath.Join(usr.HomeDir, ".mcp-devtools"), nil
}

// ensureDirectoryExists creates the directory if it doesn't exist
func ensureDirectoryExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0700)
	}
	return nil
}

// LoadGraph loads the complete knowledge graph from storage
func (s *Storage) LoadGraph() (*KnowledgeGraph, error) {
	// Check data retention before loading
	if err := s.checkDataRetention(); err != nil {
		return nil, fmt.Errorf("data retention check failed: %w", err)
	}

	// Use file locking to ensure consistent reads
	fileLock := flock.New(s.filePath + ".lock")
	locked, err := fileLock.TryRLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire read lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("could not acquire read lock on memory file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release read lock")
		}
	}()

	// Check if file exists
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		// Return empty graph if file doesn't exist
		return &KnowledgeGraph{
			Entities:  []Entity{},
			Relations: []Relation{},
		}, nil
	}

	file, err := os.Open(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open memory file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close memory file")
		}
	}()

	// Read and decrypt file contents
	fileData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory file: %w", err)
	}

	// Decrypt data if encryption is enabled
	decryptedData, err := s.decrypt(fileData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt memory file: %w", err)
	}

	graph := &KnowledgeGraph{
		Entities:  []Entity{},
		Relations: []Relation{},
	}

	scanner := bufio.NewScanner(strings.NewReader(string(decryptedData)))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // Skip empty lines
		}

		// Parse the line to determine type
		var typeCheck struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &typeCheck); err != nil {
			s.logger.WithError(err).WithField("line", lineNum).Warn("Failed to parse line type, skipping")
			continue
		}

		switch typeCheck.Type {
		case "entity":
			var storedEntity StoredEntity
			if err := json.Unmarshal([]byte(line), &storedEntity); err != nil {
				s.logger.WithError(err).WithField("line", lineNum).Warn("Failed to parse entity, skipping")
				continue
			}
			graph.Entities = append(graph.Entities, Entity{
				Name:         storedEntity.Name,
				EntityType:   storedEntity.EntityType,
				Observations: storedEntity.Observations,
			})

		case "relation":
			var storedRelation StoredRelation
			if err := json.Unmarshal([]byte(line), &storedRelation); err != nil {
				s.logger.WithError(err).WithField("line", lineNum).Warn("Failed to parse relation, skipping")
				continue
			}
			graph.Relations = append(graph.Relations, Relation{
				From:         storedRelation.From,
				To:           storedRelation.To,
				RelationType: storedRelation.RelationType,
			})

		default:
			s.logger.WithField("type", typeCheck.Type).WithField("line", lineNum).Warn("Unknown type, skipping")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading memory file: %w", err)
	}

	return graph, nil
}

// SaveGraph saves the complete knowledge graph to storage using atomic operations
func (s *Storage) SaveGraph(graph *KnowledgeGraph) error {
	// Validate storage size before saving
	if err := s.validateStorageSize(); err != nil {
		return fmt.Errorf("storage size validation failed: %w", err)
	}

	// Use file locking to ensure atomic writes
	fileLock := flock.New(s.filePath + ".lock")
	locked, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire write lock on memory file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release write lock")
		}
	}()

	// Build content in memory first for encryption
	var contentBuilder strings.Builder

	// Write entities to content builder
	for _, entity := range graph.Entities {
		storedEntity := StoredEntity{
			Type:         "entity",
			Name:         entity.Name,
			EntityType:   entity.EntityType,
			Observations: entity.Observations,
		}
		data, err := json.Marshal(storedEntity)
		if err != nil {
			return fmt.Errorf("failed to marshal entity %s: %w", entity.Name, err)
		}
		contentBuilder.Write(data)
		contentBuilder.WriteString("\n")
	}

	// Write relations to content builder
	for _, relation := range graph.Relations {
		storedRelation := StoredRelation{
			Type:         "relation",
			From:         relation.From,
			To:           relation.To,
			RelationType: relation.RelationType,
		}
		data, err := json.Marshal(storedRelation)
		if err != nil {
			return fmt.Errorf("failed to marshal relation %s->%s: %w", relation.From, relation.To, err)
		}
		contentBuilder.Write(data)
		contentBuilder.WriteString("\n")
	}

	// Encrypt content
	encryptedData, err := s.encrypt([]byte(contentBuilder.String()))
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}

	// Write to temporary file first for atomic operation
	tempFile := s.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close temporary file")
		}
		// Clean up temp file if it still exists
		if _, err := os.Stat(tempFile); err == nil {
			if err := os.Remove(tempFile); err != nil {
				s.logger.WithError(err).Warn("Failed to remove temporary file")
			}
		}
	}()

	// Write encrypted data to file
	if _, err := file.Write(encryptedData); err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}

	// Close the file before rename
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	// Validate storage size after saving
	if err := s.validateStorageSize(); err != nil {
		s.logger.WithError(err).Warn("Storage size validation warning after save - consider reducing data")
	}

	return nil
}

// GetFilePath returns the configured file path
func (s *Storage) GetFilePath() string {
	return s.filePath
}

// FileExists checks if the memory file exists
func (s *Storage) FileExists() bool {
	_, err := os.Stat(s.filePath)
	return !os.IsNotExist(err)
}

// GetFileInfo returns information about the memory file
func (s *Storage) GetFileInfo() (os.FileInfo, error) {
	return os.Stat(s.filePath)
}

// BackupFile creates a backup of the current memory file
func (s *Storage) BackupFile() error {
	if !s.FileExists() {
		return nil // Nothing to backup
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := s.filePath + ".backup." + timestamp

	// Use file locking during backup
	fileLock := flock.New(s.filePath + ".lock")
	locked, err := fileLock.TryRLock()
	if err != nil {
		return fmt.Errorf("failed to acquire read lock for backup: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire read lock for backup")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release read lock after backup")
		}
	}()

	// Copy file
	sourceFile, err := os.Open(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to open source file for backup: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close source file during backup")
		}
	}()

	destFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close backup file")
		}
	}()

	// Copy contents
	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	s.logger.WithField("backup_path", backupPath).Info("Memory file backed up successfully")
	return nil
}
