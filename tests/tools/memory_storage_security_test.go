package tools_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/memory"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
)

func TestMemoryStorage_DefaultLimits(t *testing.T) {
	// Save original environment variables
	originalMaxStorageSize := os.Getenv("MEMORY_MAX_STORAGE_SIZE")
	originalDataRetentionDays := os.Getenv("MEMORY_DATA_RETENTION_DAYS")
	originalEncryptionPassword := os.Getenv("MEMORY_ENCRYPTION_PASSWORD")
	defer func() {
		if originalMaxStorageSize == "" {
			_ = os.Unsetenv("MEMORY_MAX_STORAGE_SIZE")
		} else {
			_ = os.Setenv("MEMORY_MAX_STORAGE_SIZE", originalMaxStorageSize)
		}
		if originalDataRetentionDays == "" {
			_ = os.Unsetenv("MEMORY_DATA_RETENTION_DAYS")
		} else {
			_ = os.Setenv("MEMORY_DATA_RETENTION_DAYS", originalDataRetentionDays)
		}
		if originalEncryptionPassword == "" {
			_ = os.Unsetenv("MEMORY_ENCRYPTION_PASSWORD")
		} else {
			_ = os.Setenv("MEMORY_ENCRYPTION_PASSWORD", originalEncryptionPassword)
		}
	}()

	// Clear environment variables to test defaults
	_ = os.Unsetenv("MEMORY_MAX_STORAGE_SIZE")
	_ = os.Unsetenv("MEMORY_DATA_RETENTION_DAYS")
	_ = os.Unsetenv("MEMORY_ENCRYPTION_PASSWORD")

	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise during tests

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)

	// Test that we can create and use storage with default settings
	testutils.AssertNotNil(t, storage)
}

func TestMemoryStorage_CustomLimits(t *testing.T) {
	// Save original environment variables
	originalMaxStorageSize := os.Getenv("MEMORY_MAX_STORAGE_SIZE")
	originalDataRetentionDays := os.Getenv("MEMORY_DATA_RETENTION_DAYS")
	defer func() {
		if originalMaxStorageSize == "" {
			_ = os.Unsetenv("MEMORY_MAX_STORAGE_SIZE")
		} else {
			_ = os.Setenv("MEMORY_MAX_STORAGE_SIZE", originalMaxStorageSize)
		}
		if originalDataRetentionDays == "" {
			_ = os.Unsetenv("MEMORY_DATA_RETENTION_DAYS")
		} else {
			_ = os.Setenv("MEMORY_DATA_RETENTION_DAYS", originalDataRetentionDays)
		}
	}()

	// Set custom limits
	err := os.Setenv("MEMORY_MAX_STORAGE_SIZE", "1048576") // 1MB
	testutils.AssertNoError(t, err)
	err = os.Setenv("MEMORY_DATA_RETENTION_DAYS", "30")
	testutils.AssertNoError(t, err)

	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, storage)
}

func TestMemoryStorage_InvalidEnvironmentVariables(t *testing.T) {
	// Save original environment variables
	originalMaxStorageSize := os.Getenv("MEMORY_MAX_STORAGE_SIZE")
	originalDataRetentionDays := os.Getenv("MEMORY_DATA_RETENTION_DAYS")
	defer func() {
		if originalMaxStorageSize == "" {
			_ = os.Unsetenv("MEMORY_MAX_STORAGE_SIZE")
		} else {
			_ = os.Setenv("MEMORY_MAX_STORAGE_SIZE", originalMaxStorageSize)
		}
		if originalDataRetentionDays == "" {
			_ = os.Unsetenv("MEMORY_DATA_RETENTION_DAYS")
		} else {
			_ = os.Setenv("MEMORY_DATA_RETENTION_DAYS", originalDataRetentionDays)
		}
	}()

	// Set invalid values - should fall back to defaults
	err := os.Setenv("MEMORY_MAX_STORAGE_SIZE", "invalid")
	testutils.AssertNoError(t, err)
	err = os.Setenv("MEMORY_DATA_RETENTION_DAYS", "not_a_number")
	testutils.AssertNoError(t, err)

	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, storage)
}

func TestMemoryStorage_EncryptionEnabled(t *testing.T) {
	// Save original environment variable
	originalEncryptionPassword := os.Getenv("MEMORY_ENCRYPTION_PASSWORD")
	defer func() {
		if originalEncryptionPassword == "" {
			_ = os.Unsetenv("MEMORY_ENCRYPTION_PASSWORD")
		} else {
			_ = os.Setenv("MEMORY_ENCRYPTION_PASSWORD", originalEncryptionPassword)
		}
	}()

	// Enable encryption
	err := os.Setenv("MEMORY_ENCRYPTION_PASSWORD", "test-password-123")
	testutils.AssertNoError(t, err)

	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)

	// Create a simple knowledge graph
	graph := &memory.KnowledgeGraph{
		Entities: []memory.Entity{
			{
				Name:         "test_entity",
				EntityType:   "concept",
				Observations: []string{"test observation"},
			},
		},
		Relations: []memory.Relation{},
	}

	// Save the graph
	err = storage.SaveGraph(graph)
	testutils.AssertNoError(t, err)

	// Load the graph back
	loadedGraph, err := storage.LoadGraph()
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, loadedGraph)
	testutils.AssertEqual(t, 1, len(loadedGraph.Entities))
	testutils.AssertEqual(t, "test_entity", loadedGraph.Entities[0].Name)

	// Verify that the file on disk is encrypted (should not contain plain text)
	fileData, err := os.ReadFile(storage.GetFilePath())
	testutils.AssertNoError(t, err)

	// The encrypted file should not contain our test string
	testutils.AssertFalse(t, strings.Contains(string(fileData), "test_entity"))
	testutils.AssertFalse(t, strings.Contains(string(fileData), "test observation"))
}

func TestMemoryStorage_EncryptionDisabled(t *testing.T) {
	// Save original environment variable
	originalEncryptionPassword := os.Getenv("MEMORY_ENCRYPTION_PASSWORD")
	defer func() {
		if originalEncryptionPassword == "" {
			_ = os.Unsetenv("MEMORY_ENCRYPTION_PASSWORD")
		} else {
			_ = os.Setenv("MEMORY_ENCRYPTION_PASSWORD", originalEncryptionPassword)
		}
	}()

	// Disable encryption (empty password)
	_ = os.Unsetenv("MEMORY_ENCRYPTION_PASSWORD")

	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)

	// Create a simple knowledge graph
	graph := &memory.KnowledgeGraph{
		Entities: []memory.Entity{
			{
				Name:         "plain_entity",
				EntityType:   "concept",
				Observations: []string{"plain observation"},
			},
		},
		Relations: []memory.Relation{},
	}

	// Save the graph
	err = storage.SaveGraph(graph)
	testutils.AssertNoError(t, err)

	// Load the graph back
	loadedGraph, err := storage.LoadGraph()
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, loadedGraph)
	testutils.AssertEqual(t, 1, len(loadedGraph.Entities))
	testutils.AssertEqual(t, "plain_entity", loadedGraph.Entities[0].Name)

	// Verify that the file on disk contains plain text (not encrypted)
	fileData, err := os.ReadFile(storage.GetFilePath())
	testutils.AssertNoError(t, err)

	// The unencrypted file should contain our test strings
	testutils.AssertTrue(t, strings.Contains(string(fileData), "plain_entity"))
	testutils.AssertTrue(t, strings.Contains(string(fileData), "plain observation"))
}

func TestMemoryStorage_DataRetentionCheck(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	// Set short retention period for testing
	originalDataRetentionDays := os.Getenv("MEMORY_DATA_RETENTION_DAYS")
	defer func() {
		if originalDataRetentionDays == "" {
			_ = os.Unsetenv("MEMORY_DATA_RETENTION_DAYS")
		} else {
			_ = os.Setenv("MEMORY_DATA_RETENTION_DAYS", originalDataRetentionDays)
		}
	}()
	err := os.Setenv("MEMORY_DATA_RETENTION_DAYS", "1") // 1 day
	testutils.AssertNoError(t, err)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)

	// Create and save a graph first
	graph := &memory.KnowledgeGraph{
		Entities:  []memory.Entity{},
		Relations: []memory.Relation{},
	}
	err = storage.SaveGraph(graph)
	testutils.AssertNoError(t, err)

	// Manually modify the file timestamp to be older than retention period
	oldTime := time.Now().Add(-48 * time.Hour) // 2 days ago
	err = os.Chtimes(storage.GetFilePath(), oldTime, oldTime)
	testutils.AssertNoError(t, err)

	// Loading should work but log a warning about retention
	loadedGraph, err := storage.LoadGraph()
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, loadedGraph)
}

func TestMemoryStorage_ZeroValues(t *testing.T) {
	// Save original environment variables
	originalMaxStorageSize := os.Getenv("MEMORY_MAX_STORAGE_SIZE")
	originalDataRetentionDays := os.Getenv("MEMORY_DATA_RETENTION_DAYS")
	defer func() {
		if originalMaxStorageSize == "" {
			_ = os.Unsetenv("MEMORY_MAX_STORAGE_SIZE")
		} else {
			_ = os.Setenv("MEMORY_MAX_STORAGE_SIZE", originalMaxStorageSize)
		}
		if originalDataRetentionDays == "" {
			_ = os.Unsetenv("MEMORY_DATA_RETENTION_DAYS")
		} else {
			_ = os.Setenv("MEMORY_DATA_RETENTION_DAYS", originalDataRetentionDays)
		}
	}()

	// Set zero values - should fall back to defaults
	err := os.Setenv("MEMORY_MAX_STORAGE_SIZE", "0")
	testutils.AssertNoError(t, err)
	err = os.Setenv("MEMORY_DATA_RETENTION_DAYS", "0")
	testutils.AssertNoError(t, err)

	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, storage)
}

func TestMemoryStorage_NegativeValues(t *testing.T) {
	// Save original environment variables
	originalMaxStorageSize := os.Getenv("MEMORY_MAX_STORAGE_SIZE")
	originalDataRetentionDays := os.Getenv("MEMORY_DATA_RETENTION_DAYS")
	defer func() {
		if originalMaxStorageSize == "" {
			_ = os.Unsetenv("MEMORY_MAX_STORAGE_SIZE")
		} else {
			_ = os.Setenv("MEMORY_MAX_STORAGE_SIZE", originalMaxStorageSize)
		}
		if originalDataRetentionDays == "" {
			_ = os.Unsetenv("MEMORY_DATA_RETENTION_DAYS")
		} else {
			_ = os.Setenv("MEMORY_DATA_RETENTION_DAYS", originalDataRetentionDays)
		}
	}()

	// Set negative values - should fall back to defaults
	err := os.Setenv("MEMORY_MAX_STORAGE_SIZE", "-1000")
	testutils.AssertNoError(t, err)
	err = os.Setenv("MEMORY_DATA_RETENTION_DAYS", "-30")
	testutils.AssertNoError(t, err)

	// Create temporary directory for test
	tempDir := t.TempDir()
	_ = os.Setenv("MEMORY_FILE_PATH", tempDir)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	storage, err := memory.NewStorage(logger)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, storage)
}

func TestMemoryStorage_Constants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "MEMORY_MAX_STORAGE_SIZE", memory.MemoryMaxStorageSizeEnvVar)
	testutils.AssertEqual(t, "MEMORY_DATA_RETENTION_DAYS", memory.MemoryDataRetentionDaysEnvVar)
	testutils.AssertEqual(t, "MEMORY_ENCRYPTION_PASSWORD", memory.MemoryEncryptionPasswordEnvVar)
	testutils.AssertEqual(t, int64(500*1024*1024), memory.DefaultMaxStorageSize)
	testutils.AssertEqual(t, 180, memory.DefaultDataRetentionDays)
}
