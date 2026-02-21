package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/collab"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
)

type collabTestEnv struct {
	tool    *collab.CollabTool
	ctx     context.Context
	logger  *logrus.Logger
	cache   *sync.Map
	cleanup func()
}

func setupCollabTest(t *testing.T) *collabTestEnv {
	t.Helper()
	tmpDir := t.TempDir()
	origCollab := os.Getenv("COLLAB_DIR")
	_ = os.Setenv("COLLAB_DIR", tmpDir)

	origEnable := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "collab")

	return &collabTestEnv{
		tool:   &collab.CollabTool{},
		ctx:    testutils.CreateTestContext(),
		logger: testutils.CreateTestLogger(),
		cache:  testutils.CreateTestCache(),
		cleanup: func() {
			if origCollab == "" {
				_ = os.Unsetenv("COLLAB_DIR")
			} else {
				_ = os.Setenv("COLLAB_DIR", origCollab)
			}
			if origEnable == "" {
				_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
			} else {
				_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", origEnable)
			}
		},
	}
}

func (e *collabTestEnv) exec(t *testing.T, args map[string]any) (*mcp.CallToolResult, error) {
	t.Helper()
	return e.tool.Execute(e.ctx, e.logger, e.cache, args)
}

func (e *collabTestEnv) createSession(t *testing.T, topic, name string) string {
	t.Helper()
	result, err := e.exec(t, map[string]any{
		"action": "create_session",
		"topic":  topic,
		"name":   name,
	})
	testutils.AssertNoError(t, err)

	var resp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &resp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	return resp.SessionID
}

func (e *collabTestEnv) joinSession(t *testing.T, sessionID, name string) {
	t.Helper()
	_, err := e.exec(t, map[string]any{
		"action":     "join_session",
		"session_id": sessionID,
		"name":       name,
	})
	testutils.AssertNoError(t, err)
}

func (e *collabTestEnv) postMessage(t *testing.T, sessionID, name, content, msgType string) {
	t.Helper()
	_, err := e.exec(t, map[string]any{
		"action":     "post",
		"session_id": sessionID,
		"content":    content,
		"type":       msgType,
		"name":       name,
	})
	testutils.AssertNoError(t, err)
}

func extractCollabText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}
	textContent, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("Expected TextContent")
	}
	return textContent.Text
}

func TestCollabTool_Definition(t *testing.T) {
	tool := &collab.CollabTool{}
	def := tool.Definition()

	testutils.AssertEqual(t, "collab", def.Name)
	testutils.AssertNotNil(t, def.Description)
	testutils.AssertNotNil(t, def.InputSchema)
}

func TestCollabTool_CreateSession(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	result, err := env.exec(t, map[string]any{
		"action": "create_session",
		"topic":  "Test collaboration topic",
		"name":   "agent-a",
	})
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	var resp struct {
		SessionID   string `json:"session_id"`
		Topic       string `json:"topic"`
		Participant string `json:"participant"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &resp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}

	testutils.AssertEqual(t, "Test collaboration topic", resp.Topic)
	testutils.AssertEqual(t, "agent-a", resp.Participant)
	testutils.AssertEqual(t, "active", resp.Status)
	if resp.SessionID == "" {
		t.Fatal("Expected non-empty session_id")
	}
}

func TestCollabTool_JoinSession(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test join", "agent-a")

	result, err := env.exec(t, map[string]any{
		"action":     "join_session",
		"session_id": sessionID,
		"name":       "agent-b",
	})
	testutils.AssertNoError(t, err)

	var resp struct {
		SessionID    string   `json:"session_id"`
		Topic        string   `json:"topic"`
		Status       string   `json:"status"`
		Participant  string   `json:"participant"`
		Participants []string `json:"participants"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &resp); err != nil {
		t.Fatalf("Failed to parse join response: %v", err)
	}

	testutils.AssertEqual(t, sessionID, resp.SessionID)
	testutils.AssertEqual(t, "agent-b", resp.Participant)
	testutils.AssertEqual(t, "active", resp.Status)
	if len(resp.Participants) != 2 {
		t.Fatalf("Expected 2 participants, got %d", len(resp.Participants))
	}
}

func TestCollabTool_PostAndCheck(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test post", "agent-a")
	env.joinSession(t, sessionID, "agent-b")

	// Post message
	postResult, err := env.exec(t, map[string]any{
		"action":     "post",
		"session_id": sessionID,
		"content":    "Need streaming support",
		"type":       "feature_request",
		"name":       "agent-a",
	})
	testutils.AssertNoError(t, err)

	var postResp struct {
		MessageID int    `json:"message_id"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, postResult)), &postResp); err != nil {
		t.Fatalf("Failed to parse post response: %v", err)
	}
	testutils.AssertEqual(t, 1, postResp.MessageID)

	// Check messages from agent-b's perspective
	checkResult, err := env.exec(t, map[string]any{
		"action":     "check",
		"session_id": sessionID,
		"name":       "agent-b",
	})
	testutils.AssertNoError(t, err)

	var checkResp struct {
		SessionID   string `json:"session_id"`
		HasNew      bool   `json:"has_new"`
		NewMessages []struct {
			ID      int    `json:"id"`
			From    string `json:"from"`
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"new_messages"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, checkResult)), &checkResp); err != nil {
		t.Fatalf("Failed to parse check response: %v", err)
	}

	testutils.AssertTrue(t, checkResp.HasNew)
	if len(checkResp.NewMessages) != 1 {
		t.Fatalf("Expected 1 new message, got %d", len(checkResp.NewMessages))
	}
	testutils.AssertEqual(t, "agent-a", checkResp.NewMessages[0].From)
	testutils.AssertEqual(t, "feature_request", checkResp.NewMessages[0].Type)
	testutils.AssertEqual(t, "Need streaming support", checkResp.NewMessages[0].Content)
}

func TestCollabTool_ReadAllMessages(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test read", "agent-a")
	env.joinSession(t, sessionID, "agent-b")
	env.postMessage(t, sessionID, "agent-a", "First message", "general")
	env.postMessage(t, sessionID, "agent-b", "Second message", "feedback")

	result, err := env.exec(t, map[string]any{
		"action":     "read",
		"session_id": sessionID,
	})
	testutils.AssertNoError(t, err)

	var readResp struct {
		Total    int `json:"total"`
		Messages []struct {
			ID   int    `json:"id"`
			From string `json:"from"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &readResp); err != nil {
		t.Fatalf("Failed to parse read response: %v", err)
	}

	testutils.AssertEqual(t, 2, readResp.Total)
	testutils.AssertEqual(t, "agent-a", readResp.Messages[0].From)
	testutils.AssertEqual(t, "agent-b", readResp.Messages[1].From)
}

func TestCollabTool_ListSessions(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	env.createSession(t, "Session 1", "agent-a")
	env.createSession(t, "Session 2", "agent-b")

	result, err := env.exec(t, map[string]any{
		"action": "list_sessions",
	})
	testutils.AssertNoError(t, err)

	var listResp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &listResp); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	testutils.AssertEqual(t, 2, listResp.Total)
}

func TestCollabTool_ListSessions_StatusFilter(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	closedID := env.createSession(t, "Closed session", "agent-a")
	_, err := env.exec(t, map[string]any{
		"action":     "close",
		"session_id": closedID,
		"summary":    "Done",
	})
	testutils.AssertNoError(t, err)

	env.createSession(t, "Active session", "agent-b")

	result, err := env.exec(t, map[string]any{
		"action": "list_sessions",
		"status": "active",
	})
	testutils.AssertNoError(t, err)

	var listResp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &listResp); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	testutils.AssertEqual(t, 1, listResp.Total)
}

func TestCollabTool_CloseSession(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test close", "agent-a")

	result, err := env.exec(t, map[string]any{
		"action":     "close",
		"session_id": sessionID,
		"summary":    "Collaboration complete",
	})
	testutils.AssertNoError(t, err)

	var closeResp struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
		Summary   string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &closeResp); err != nil {
		t.Fatalf("Failed to parse close response: %v", err)
	}

	testutils.AssertEqual(t, sessionID, closeResp.SessionID)
	testutils.AssertEqual(t, "closed", closeResp.Status)
	testutils.AssertEqual(t, "Collaboration complete", closeResp.Summary)

	// Posting to closed session should fail
	_, err = env.exec(t, map[string]any{
		"action":     "post",
		"session_id": sessionID,
		"content":    "Should fail",
		"name":       "agent-a",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "closed")
}

func TestCollabTool_InvalidSessionID(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	_, err := env.exec(t, map[string]any{
		"action":     "join_session",
		"session_id": "not-a-uuid",
		"name":       "agent-a",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "invalid session_id")
}

func TestCollabTool_MissingAction(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	_, err := env.exec(t, map[string]any{})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "action")
}

func TestCollabTool_MissingTopic(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	_, err := env.exec(t, map[string]any{
		"action": "create_session",
		"name":   "agent-a",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "topic")
}

func TestCollabTool_InvalidParticipantName(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	_, err := env.exec(t, map[string]any{
		"action": "create_session",
		"topic":  "Test",
		"name":   "invalid name with spaces!",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "participant name")
}

func TestCollabTool_PostNonParticipant(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test post", "agent-a")

	_, err := env.exec(t, map[string]any{
		"action":     "post",
		"session_id": sessionID,
		"content":    "Should fail",
		"name":       "agent-b",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "not joined")
}

func TestCollabTool_ContentTooLong(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test", "agent-a")

	longContent := make([]byte, 100001)
	for i := range longContent {
		longContent[i] = 'a'
	}

	_, err := env.exec(t, map[string]any{
		"action":     "post",
		"session_id": sessionID,
		"content":    string(longContent),
		"name":       "agent-a",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "exceeds maximum length")
}

func TestCollabTool_FilesystemLayout(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Layout test", "agent-a")
	env.joinSession(t, sessionID, "agent-c")
	env.postMessage(t, sessionID, "agent-a", "Hello", "general")

	collabDir := os.Getenv("COLLAB_DIR")
	sessionDir := filepath.Join(collabDir, "sessions", sessionID)

	// session.json should have 0600 permissions
	sessionFile := filepath.Join(sessionDir, "session.json")
	info, err := os.Stat(sessionFile)
	if err != nil {
		t.Fatalf("Expected session.json to exist: %v", err)
	}
	testutils.AssertEqual(t, os.FileMode(0600), info.Mode().Perm())

	// msg-001.json should have 0600 permissions
	msgFile := filepath.Join(sessionDir, "msg-001.json")
	info, err = os.Stat(msgFile)
	if err != nil {
		t.Fatalf("Expected msg-001.json to exist: %v", err)
	}
	testutils.AssertEqual(t, os.FileMode(0600), info.Mode().Perm())

	// Session directory should have 0700 permissions
	dirInfo, err := os.Stat(sessionDir)
	if err != nil {
		t.Fatalf("Expected session directory to exist: %v", err)
	}
	testutils.AssertEqual(t, os.FileMode(0700), dirInfo.Mode().Perm())
}

func TestCollabWaitTool_Definition(t *testing.T) {
	tool := &collab.CollabWaitTool{}
	def := tool.Definition()

	testutils.AssertEqual(t, "collab_wait", def.Name)
	testutils.AssertNotNil(t, def.Description)
	testutils.AssertNotNil(t, def.InputSchema)
}

func TestCollabWaitTool_Timeout(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Wait test", "agent-a")

	// Use a very short timeout and poll interval so the test completes quickly
	waitTool := &collab.CollabWaitTool{}
	result, err := waitTool.Execute(env.ctx, env.logger, env.cache, map[string]any{
		"session_id":            sessionID,
		"timeout_seconds":       float64(2),
		"poll_interval_seconds": float64(5), // will be clamped to timeout
	})
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	text := extractCollabText(t, result)
	var resp struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
		NewCount  int    `json:"new_count"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("Failed to parse wait response: %v", err)
	}
	testutils.AssertEqual(t, sessionID, resp.SessionID)
	testutils.AssertEqual(t, "timeout", resp.Status)
	testutils.AssertEqual(t, 0, resp.NewCount)
}

func TestCollabWaitTool_DetectsNewMessages(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Wait detect test", "agent-a")
	env.joinSession(t, sessionID, "agent-b")

	// Post a message after a short delay in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(500 * time.Millisecond)
		env.postMessage(t, sessionID, "agent-a", "Hello from goroutine", "general")
	}()

	waitTool := &collab.CollabWaitTool{}
	result, err := waitTool.Execute(env.ctx, env.logger, env.cache, map[string]any{
		"session_id":            sessionID,
		"timeout_seconds":       float64(10),
		"poll_interval_seconds": float64(5), // minimum poll interval
	})
	testutils.AssertNoError(t, err)

	text := extractCollabText(t, result)
	var resp struct {
		Status   string `json:"status"`
		NewCount int    `json:"new_count"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("Failed to parse wait response: %v", err)
	}
	testutils.AssertEqual(t, "new_messages", resp.Status)
	testutils.AssertEqual(t, 1, resp.NewCount)

	<-done
}

func TestCollabWaitTool_ImmediateReturnOnUnread(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Unread test", "agent-a")
	env.joinSession(t, sessionID, "agent-b")

	// Post a message that agent-b hasn't read
	env.postMessage(t, sessionID, "agent-a", "Unread message", "general")

	// collab_wait with agent-b's name should return immediately
	// because there's already an unread message
	waitTool := &collab.CollabWaitTool{}
	start := time.Now()
	result, err := waitTool.Execute(env.ctx, env.logger, env.cache, map[string]any{
		"session_id":            sessionID,
		"timeout_seconds":       float64(30),
		"poll_interval_seconds": float64(5),
		"name":                  "agent-b",
	})
	elapsed := time.Since(start)
	testutils.AssertNoError(t, err)

	// Should return in well under a second (no polling needed)
	if elapsed > 2*time.Second {
		t.Fatalf("Expected immediate return for unread messages, took %v", elapsed)
	}

	text := extractCollabText(t, result)
	var resp struct {
		Status   string `json:"status"`
		NewCount int    `json:"new_count"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("Failed to parse wait response: %v", err)
	}
	testutils.AssertEqual(t, "new_messages", resp.Status)
	testutils.AssertEqual(t, 1, resp.NewCount)
}

func TestCollabTool_CheckNoNewMessages(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test check empty", "agent-a")

	result, err := env.exec(t, map[string]any{
		"action":     "check",
		"session_id": sessionID,
		"name":       "agent-a",
	})
	testutils.AssertNoError(t, err)

	var checkResp struct {
		HasNew bool `json:"has_new"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &checkResp); err != nil {
		t.Fatalf("Failed to parse check response: %v", err)
	}
	testutils.AssertFalse(t, checkResp.HasNew)
}

func TestCollabTool_NonexistentSession(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	_, err := env.exec(t, map[string]any{
		"action":     "read",
		"session_id": "00000000-0000-0000-0000-000000000000",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "not found")
}

func TestCollabTool_JoinClosedSession(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Test closed join", "agent-a")

	_, err := env.exec(t, map[string]any{
		"action":     "close",
		"session_id": sessionID,
	})
	testutils.AssertNoError(t, err)

	_, err = env.exec(t, map[string]any{
		"action":     "join_session",
		"session_id": sessionID,
		"name":       "agent-b",
	})
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "closed")
}

func TestCollabTool_MultipleMessages(t *testing.T) {
	env := setupCollabTest(t)
	defer env.cleanup()

	sessionID := env.createSession(t, "Multi-message test", "agent-a")
	env.joinSession(t, sessionID, "agent-b")

	// Post several messages
	env.postMessage(t, sessionID, "agent-a", "First", "question")
	env.postMessage(t, sessionID, "agent-b", "Second", "feedback")
	env.postMessage(t, sessionID, "agent-a", "Third", "general")

	// Read all and verify ordering
	result, err := env.exec(t, map[string]any{
		"action":     "read",
		"session_id": sessionID,
	})
	testutils.AssertNoError(t, err)

	var readResp struct {
		Total    int `json:"total"`
		Messages []struct {
			ID int `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(extractCollabText(t, result)), &readResp); err != nil {
		t.Fatalf("Failed to parse read response: %v", err)
	}

	testutils.AssertEqual(t, 3, readResp.Total)
	testutils.AssertEqual(t, 1, readResp.Messages[0].ID)
	testutils.AssertEqual(t, 2, readResp.Messages[1].ID)
	testutils.AssertEqual(t, 3, readResp.Messages[2].ID)
}
