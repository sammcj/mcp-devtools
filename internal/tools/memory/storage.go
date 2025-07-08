package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/sirupsen/logrus"
)

// Storage handles file I/O operations for the knowledge graph
type Storage struct {
	basePath  string
	namespace string
	filePath  string
	logger    *logrus.Logger
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

	// Update file path for the namespace
	if err := storage.updateFilePath(); err != nil {
		return nil, fmt.Errorf("failed to update file path: %w", err)
	}

	return storage, nil
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
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// LoadGraph loads the complete knowledge graph from storage
func (s *Storage) LoadGraph() (*KnowledgeGraph, error) {
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

	graph := &KnowledgeGraph{
		Entities:  []Entity{},
		Relations: []Relation{},
	}

	scanner := bufio.NewScanner(file)
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

	writer := bufio.NewWriter(file)

	// Write entities
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
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("failed to write entity %s: %w", entity.Name, err)
		}
		if _, err := writer.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline after entity %s: %w", entity.Name, err)
		}
	}

	// Write relations
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
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("failed to write relation %s->%s: %w", relation.From, relation.To, err)
		}
		if _, err := writer.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline after relation %s->%s: %w", relation.From, relation.To, err)
		}
	}

	// Flush the writer
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	// Close the file before rename
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
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
