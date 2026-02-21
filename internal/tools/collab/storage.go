package collab

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/sirupsen/logrus"
)

var participantNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Storage handles filesystem operations for collaboration sessions
type Storage struct {
	basePath string
	logger   *logrus.Logger
}

// NewStorage creates a new storage instance
func NewStorage(logger *logrus.Logger) (*Storage, error) {
	basePath, err := getCollabBasePath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine collab base path: %w", err)
	}

	sessionsDir := filepath.Join(basePath, "sessions")
	if err := ensureDir(sessionsDir); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Storage{
		basePath: basePath,
		logger:   logger,
	}, nil
}

// getCollabBasePath returns the base directory for collaboration data
func getCollabBasePath() (string, error) {
	if envPath := os.Getenv("COLLAB_DIR"); envPath != "" {
		absPath, err := filepath.Abs(envPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve COLLAB_DIR path: %w", err)
		}
		return absPath, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	return filepath.Join(usr.HomeDir, ".mcp-devtools", "collab"), nil
}

// ensureDir creates a directory with 0700 permissions if it doesn't exist
func ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0700)
	}
	return nil
}

// sessionDir returns the directory for a specific session
func (s *Storage) sessionDir(sessionID string) string {
	return filepath.Join(s.basePath, "sessions", sessionID)
}

// sessionFilePath returns the path to session.json for a specific session
func (s *Storage) sessionFilePath(sessionID string) string {
	return filepath.Join(s.sessionDir(sessionID), "session.json")
}

// lockPath returns the path to the lock file for a specific session
func (s *Storage) lockPath(sessionID string) string {
	return filepath.Join(s.sessionDir(sessionID), "session.json.lock")
}

// messageFilePath returns the path to a specific message file
func (s *Storage) messageFilePath(sessionID string, msgID int) string {
	return filepath.Join(s.sessionDir(sessionID), fmt.Sprintf("msg-%03d.json", msgID))
}

// CreateSession creates a new collaboration session
func (s *Storage) CreateSession(sessionID, topic, createdBy string) (*Session, error) {
	dir := s.sessionDir(sessionID)
	if err := ensureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	now := time.Now().UTC()
	session := &Session{
		SessionID: sessionID,
		Topic:     topic,
		Status:    "active",
		Participants: map[string]*Participant{
			createdBy: {
				JoinedAt: now,
				LastRead: 0,
			},
		},
		CreatedBy:    createdBy,
		CreatedAt:    now,
		UpdatedAt:    now,
		MessageCount: 0,
	}

	if err := s.writeSessionAtomic(sessionID, session); err != nil {
		return nil, fmt.Errorf("failed to write session: %w", err)
	}

	return session, nil
}

// LoadSession loads session metadata from disk
func (s *Storage) LoadSession(sessionID string) (*Session, error) {
	// Check if session directory exists before attempting to lock
	if _, err := os.Stat(s.sessionDir(sessionID)); os.IsNotExist(err) {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	fileLock := flock.New(s.lockPath(sessionID))
	locked, err := fileLock.TryRLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire read lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("could not acquire read lock on session file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release read lock")
		}
	}()

	return s.readSessionFile(sessionID)
}

// readSessionFile reads the session file without locking (caller must hold lock)
func (s *Storage) readSessionFile(sessionID string) (*Session, error) {
	data, err := os.ReadFile(s.sessionFilePath(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &session, nil
}

// JoinSession adds a participant to an existing session
func (s *Storage) JoinSession(sessionID, participant string) (*Session, error) {
	fileLock := flock.New(s.lockPath(sessionID))
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire write lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("could not acquire write lock on session file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release write lock")
		}
	}()

	session, err := s.readSessionFile(sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != "active" {
		return nil, fmt.Errorf("session is closed")
	}

	// Only add if not already a participant
	if _, exists := session.Participants[participant]; !exists {
		session.Participants[participant] = &Participant{
			JoinedAt: time.Now().UTC(),
			LastRead: 0,
		}
		session.UpdatedAt = time.Now().UTC()

		if err := s.writeSessionFileUnlocked(sessionID, session); err != nil {
			return nil, err
		}
	}

	return session, nil
}

// PostMessage adds a new message to a session
func (s *Storage) PostMessage(sessionID, from, msgType, content string) (*Message, error) {
	fileLock := flock.New(s.lockPath(sessionID))
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire write lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("could not acquire write lock on session file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release write lock")
		}
	}()

	session, err := s.readSessionFile(sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != "active" {
		return nil, fmt.Errorf("session is closed")
	}

	// Check that the sender is a participant
	if _, exists := session.Participants[from]; !exists {
		return nil, fmt.Errorf("participant '%s' has not joined this session", from)
	}

	// Create message
	msgID := session.MessageCount + 1
	msg := &Message{
		ID:        msgID,
		From:      from,
		Type:      msgType,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}

	// Write message file
	msgData, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	msgPath := s.messageFilePath(sessionID, msgID)
	if err := writeFileAtomic(msgPath, msgData); err != nil {
		return nil, fmt.Errorf("failed to write message file: %w", err)
	}

	// Update session metadata
	session.MessageCount = msgID
	session.UpdatedAt = time.Now().UTC()

	if err := s.writeSessionFileUnlocked(sessionID, session); err != nil {
		return nil, err
	}

	return msg, nil
}

// GetMessagesSince returns messages with IDs greater than sinceID
func (s *Storage) GetMessagesSince(sessionID string, sinceID int) ([]Message, error) {
	session, err := s.LoadSession(sessionID)
	if err != nil {
		return nil, err
	}

	var messages []Message
	for i := sinceID + 1; i <= session.MessageCount; i++ {
		msg, err := s.readMessage(sessionID, i)
		if err != nil {
			s.logger.WithError(err).WithField("msg_id", i).Warn("Failed to read message")
			continue
		}
		messages = append(messages, *msg)
	}

	return messages, nil
}

// GetAllMessages returns all messages in a session
func (s *Storage) GetAllMessages(sessionID string) ([]Message, error) {
	return s.GetMessagesSince(sessionID, 0)
}

// UpdateLastRead updates the last_read marker for a participant
func (s *Storage) UpdateLastRead(sessionID, participant string, lastRead int) error {
	fileLock := flock.New(s.lockPath(sessionID))
	locked, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire write lock on session file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release write lock")
		}
	}()

	session, err := s.readSessionFile(sessionID)
	if err != nil {
		return err
	}

	p, exists := session.Participants[participant]
	if !exists {
		return fmt.Errorf("participant '%s' not found in session", participant)
	}

	p.LastRead = lastRead
	session.UpdatedAt = time.Now().UTC()

	return s.writeSessionFileUnlocked(sessionID, session)
}

// CloseSession marks a session as closed
func (s *Storage) CloseSession(sessionID, summary string) error {
	fileLock := flock.New(s.lockPath(sessionID))
	locked, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire write lock on session file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release write lock")
		}
	}()

	session, err := s.readSessionFile(sessionID)
	if err != nil {
		return err
	}

	session.Status = "closed"
	session.Summary = summary
	session.UpdatedAt = time.Now().UTC()

	return s.writeSessionFileUnlocked(sessionID, session)
}

// ListSessions returns all sessions, optionally filtered by status
func (s *Storage) ListSessions(statusFilter string) ([]Session, error) {
	sessionsDir := filepath.Join(s.basePath, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		session, err := s.LoadSession(sessionID)
		if err != nil {
			s.logger.WithError(err).WithField("session_id", sessionID).Warn("Failed to load session")
			continue
		}

		if statusFilter != "" && session.Status != statusFilter {
			continue
		}

		sessions = append(sessions, *session)
	}

	// Sort by updated_at descending (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// GetMessageCount returns the current message count for a session (for polling)
func (s *Storage) GetMessageCount(sessionID string) (int, error) {
	session, err := s.LoadSession(sessionID)
	if err != nil {
		return 0, err
	}
	return session.MessageCount, nil
}

// readMessage reads a single message file
func (s *Storage) readMessage(sessionID string, msgID int) (*Message, error) {
	data, err := os.ReadFile(s.messageFilePath(sessionID, msgID))
	if err != nil {
		return nil, fmt.Errorf("failed to read message file: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message file: %w", err)
	}

	return &msg, nil
}

// writeSessionAtomic writes session metadata with locking
func (s *Storage) writeSessionAtomic(sessionID string, session *Session) error {
	fileLock := flock.New(s.lockPath(sessionID))
	locked, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire write lock on session file")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			s.logger.WithError(err).Warn("Failed to release write lock")
		}
	}()

	return s.writeSessionFileUnlocked(sessionID, session)
}

// writeSessionFileUnlocked writes session metadata without locking (caller must hold lock)
func (s *Storage) writeSessionFileUnlocked(sessionID string, session *Session) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	return writeFileAtomic(s.sessionFilePath(sessionID), data)
}

// writeFileAtomic writes data to a file using temp file + rename for atomicity
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()

	// Set permissions before writing content
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// validateParticipantName validates and normalises a participant name
func validateParticipantName(name string) (string, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "", fmt.Errorf("participant name cannot be empty")
	}
	if len(name) > maxParticipantLength {
		return "", fmt.Errorf("participant name exceeds maximum length of %d characters", maxParticipantLength)
	}
	if !participantNameRegexp.MatchString(name) {
		return "", fmt.Errorf("participant name must contain only lowercase alphanumeric characters, hyphens, underscores, and dots")
	}
	return name, nil
}
