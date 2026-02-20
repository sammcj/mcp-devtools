package collab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// CollabTool implements cross-agent collaboration via filesystem-based mailbox
type CollabTool struct {
	storageOnce sync.Once
	storage     *Storage
	storageErr  error
}

// init registers the collab tool
func init() {
	registry.Register(&CollabTool{})
}

// Definition returns the tool's definition for MCP registration
func (c *CollabTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"collab",
		mcp.WithDescription(`Cross-agent collaboration tool for session-based message exchange between AI coding agents.

Enables two agents working on related projects to exchange structured messages (feature requests, implementation summaries, questions, feedback, bug reports, API changes) via a shared filesystem mailbox.

Workflow:
1. Agent A: create_session (gets UUID) -> human relays UUID to Agent B
2. Agent B: join_session (by UUID) -> sees topic and existing messages
3. Both agents: post and check messages within the session
4. Either agent: close the session when done`),

		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform"),
			mcp.Enum(
				"create_session",
				"join_session",
				"post",
				"check",
				"read",
				"list_sessions",
				"close",
			),
		),

		mcp.WithString("session_id",
			mcp.Description("Session UUID (required for join_session, post, check, read, close)"),
		),

		mcp.WithString("topic",
			mcp.Description("Session topic (required for create_session, max 500 chars)"),
		),

		mcp.WithString("name",
			mcp.Description("Participant name (optional, auto-detected from MCP roots if not provided). Lowercase alphanumeric with hyphens/underscores/dots, max 128 chars"),
		),

		mcp.WithString("content",
			mcp.Description("Message content (required for post, max 100000 chars but keep it concise)"),
		),

		mcp.WithString("type",
			mcp.Description("Message type for post action (default: general)"),
			mcp.Enum("feature_request", "implementation_summary", "question", "feedback", "bug_report", "api_change", "general"),
		),

		mcp.WithString("status",
			mcp.Description("Status filter for list_sessions (active or closed)"),
			mcp.Enum("active", "closed"),
		),

		mcp.WithString("summary",
			mcp.Description("Optional summary when closing a session (max 2000 chars)"),
		),

		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

// Execute routes the action to the appropriate handler
func (c *CollabTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Initialise storage lazily (thread-safe)
	c.storageOnce.Do(func() {
		c.storage, c.storageErr = NewStorage(logger)
	})
	if c.storageErr != nil {
		return nil, fmt.Errorf("failed to initialise collab storage: %w", c.storageErr)
	}

	action, ok := args["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("missing required parameter: action")
	}

	switch action {
	case "create_session":
		return c.handleCreateSession(ctx, logger, args)
	case "join_session":
		return c.handleJoinSession(ctx, logger, args)
	case "post":
		return c.handlePost(ctx, logger, args)
	case "check":
		return c.handleCheck(ctx, logger, args)
	case "read":
		return c.handleRead(args)
	case "list_sessions":
		return c.handleListSessions(args)
	case "close":
		return c.handleClose(args)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// handleCreateSession creates a new collaboration session
func (c *CollabTool) handleCreateSession(ctx context.Context, logger *logrus.Logger, args map[string]any) (*mcp.CallToolResult, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return nil, fmt.Errorf("missing required parameter: topic")
	}
	if len(topic) > maxTopicLength {
		return nil, fmt.Errorf("topic exceeds maximum length of %d characters", maxTopicLength)
	}

	name := c.resolveParticipantName(ctx, logger, args)
	participant, err := validateParticipantName(name)
	if err != nil {
		return nil, fmt.Errorf("invalid participant name: %w", err)
	}

	sessionID := uuid.New().String()

	session, err := c.storage.CreateSession(sessionID, topic, participant)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	resp := createSessionResponse{
		SessionID:   session.SessionID,
		Topic:       session.Topic,
		Participant: participant,
		Status:      session.Status,
		Hints:       "Share this session_id with the other agent. Use collab_wait to wait for them to join and respond, or collab check to poll manually.",
	}

	return toToolResult(resp)
}

// handleJoinSession joins an existing session
func (c *CollabTool) handleJoinSession(ctx context.Context, logger *logrus.Logger, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, err := c.requireSessionID(args)
	if err != nil {
		return nil, err
	}

	name := c.resolveParticipantName(ctx, logger, args)
	participant, err := validateParticipantName(name)
	if err != nil {
		return nil, fmt.Errorf("invalid participant name: %w", err)
	}

	session, err := c.storage.JoinSession(sessionID, participant)
	if err != nil {
		return nil, fmt.Errorf("failed to join session: %w", err)
	}

	// Read existing messages
	messages, err := c.storage.GetAllMessages(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages: %w", err)
	}

	// Update last_read to current count
	if err := c.storage.UpdateLastRead(sessionID, participant, session.MessageCount); err != nil {
		logger.WithError(err).Warn("Failed to update last_read after join")
	}

	participants := make([]string, 0, len(session.Participants))
	for p := range session.Participants {
		participants = append(participants, p)
	}

	resp := joinSessionResponse{
		SessionID:    session.SessionID,
		Topic:        session.Topic,
		Status:       session.Status,
		Participant:  participant,
		Participants: participants,
		MessageCount: session.MessageCount,
		Messages:     messages,
		Hints:        "Use collab post to send messages. Use collab_wait to wait for replies, or collab check to poll manually. Use collab close when done.",
	}

	return toToolResult(resp)
}

// handlePost posts a message to a session
func (c *CollabTool) handlePost(ctx context.Context, logger *logrus.Logger, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, err := c.requireSessionID(args)
	if err != nil {
		return nil, err
	}

	content, _ := args["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("missing required parameter: content")
	}
	if len(content) > maxContentLength {
		return nil, fmt.Errorf("content exceeds maximum length of %d characters", maxContentLength)
	}

	msgType := "general"
	if t, ok := args["type"].(string); ok && t != "" {
		if !validMessageTypes[t] {
			return nil, fmt.Errorf("invalid message type: %s", t)
		}
		msgType = t
	}

	name := c.resolveParticipantName(ctx, logger, args)
	participant, err := validateParticipantName(name)
	if err != nil {
		return nil, fmt.Errorf("invalid participant name: %w", err)
	}

	msg, err := c.storage.PostMessage(sessionID, participant, msgType, content)
	if err != nil {
		return nil, fmt.Errorf("failed to post message: %w", err)
	}

	// Send best-effort notification
	sendMessageNotification(ctx, logger, sessionID, participant, msg.ID)

	resp := postResponse{
		MessageID: msg.ID,
		SessionID: sessionID,
	}

	return toToolResult(resp)
}

// handleCheck checks for new messages since last read
func (c *CollabTool) handleCheck(ctx context.Context, logger *logrus.Logger, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, err := c.requireSessionID(args)
	if err != nil {
		return nil, err
	}

	name := c.resolveParticipantName(ctx, logger, args)
	participant, err := validateParticipantName(name)
	if err != nil {
		// Fall back to reading all messages
		participant = ""
	}

	// Load session to get last_read for this participant
	session, err := c.storage.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	sinceID := 0
	isParticipant := false
	if participant != "" {
		if p, exists := session.Participants[participant]; exists {
			sinceID = p.LastRead
			isParticipant = true
		}
	}

	messages, err := c.storage.GetMessagesSince(sessionID, sinceID)
	if err != nil {
		return nil, fmt.Errorf("failed to check messages: %w", err)
	}

	// Only update last_read if the caller is actually a session participant
	if isParticipant && len(messages) > 0 {
		lastMsgID := messages[len(messages)-1].ID
		if err := c.storage.UpdateLastRead(sessionID, participant, lastMsgID); err != nil {
			logger.WithError(err).Warn("Failed to update last_read after check")
		}
	}

	resp := checkResponse{
		SessionID:   sessionID,
		NewMessages: messages,
		HasNew:      len(messages) > 0,
	}

	return toToolResult(resp)
}

// handleRead reads all messages in a session
func (c *CollabTool) handleRead(args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, err := c.requireSessionID(args)
	if err != nil {
		return nil, err
	}

	session, err := c.storage.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	messages, err := c.storage.GetAllMessages(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages: %w", err)
	}

	resp := readResponse{
		SessionID: sessionID,
		Topic:     session.Topic,
		Messages:  messages,
		Total:     len(messages),
	}

	return toToolResult(resp)
}

// handleListSessions lists all sessions
func (c *CollabTool) handleListSessions(args map[string]any) (*mcp.CallToolResult, error) {
	statusFilter, _ := args["status"].(string)

	sessions, err := c.storage.ListSessions(statusFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	summaries := make([]sessionSummary, 0, len(sessions))
	for _, s := range sessions {
		participants := make([]string, 0, len(s.Participants))
		for p := range s.Participants {
			participants = append(participants, p)
		}

		summaries = append(summaries, sessionSummary{
			SessionID:    s.SessionID,
			Topic:        s.Topic,
			Status:       s.Status,
			Participants: participants,
			MessageCount: s.MessageCount,
			CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	resp := listSessionsResponse{
		Sessions: summaries,
		Total:    len(summaries),
	}

	return toToolResult(resp)
}

// handleClose marks a session as closed
func (c *CollabTool) handleClose(args map[string]any) (*mcp.CallToolResult, error) {
	sessionID, err := c.requireSessionID(args)
	if err != nil {
		return nil, err
	}

	summary, _ := args["summary"].(string)
	if len(summary) > maxSummaryLength {
		return nil, fmt.Errorf("summary exceeds maximum length of %d characters", maxSummaryLength)
	}

	if err := c.storage.CloseSession(sessionID, summary); err != nil {
		return nil, fmt.Errorf("failed to close session: %w", err)
	}

	resp := closeResponse{
		SessionID: sessionID,
		Status:    "closed",
		Summary:   summary,
	}

	return toToolResult(resp)
}

// requireSessionID extracts and validates the session_id parameter
func (c *CollabTool) requireSessionID(args map[string]any) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("missing required parameter: session_id")
	}

	// Validate UUID format to prevent path traversal
	if _, err := uuid.Parse(sessionID); err != nil {
		return "", fmt.Errorf("invalid session_id: must be a valid UUID")
	}

	return sessionID, nil
}

// resolveParticipantName determines the participant name from args or MCP roots
func (c *CollabTool) resolveParticipantName(ctx context.Context, logger *logrus.Logger, args map[string]any) string {
	// Explicit name takes priority
	if name, ok := args["name"].(string); ok && name != "" {
		return name
	}

	// Try MCP roots for auto-detection
	name := detectNameFromRoots(ctx, logger)
	if name != "" {
		return name
	}

	return "agent"
}

// detectNameFromRoots attempts to detect a project name from MCP roots
func detectNameFromRoots(ctx context.Context, logger *logrus.Logger) string {
	srv := mcpserver.ServerFromContext(ctx)
	if srv == nil {
		return ""
	}

	roots, err := srv.RequestRoots(ctx, mcp.ListRootsRequest{})
	if err != nil {
		logger.WithError(err).Debug("Failed to request roots for participant name detection")
		return ""
	}

	if len(roots.Roots) == 0 {
		return ""
	}

	// Extract directory name from first root URI
	rootURI := roots.Roots[0].URI
	parsed, err := url.Parse(rootURI)
	if err != nil {
		return ""
	}

	dirName := filepath.Base(parsed.Path)
	if dirName == "" || dirName == "." || dirName == "/" {
		return ""
	}

	// Normalise for use as participant name
	dirName = strings.ToLower(dirName)
	dirName = strings.ReplaceAll(dirName, " ", "-")
	if participantNameRegexp.MatchString(dirName) {
		return dirName
	}

	return ""
}

// ProvideExtendedInfo provides detailed usage information
func (c *CollabTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Create a new collaboration session",
				Arguments: map[string]any{
					"action": "create_session",
					"topic":  "Add streaming support to library-b",
					"name":   "project-a-agent",
				},
				ExpectedResult: "Returns session UUID to share with the other agent",
			},
			{
				Description: "Join an existing session",
				Arguments: map[string]any{
					"action":     "join_session",
					"session_id": "<uuid>",
					"name":       "library-b-agent",
				},
				ExpectedResult: "Returns session info and all existing messages",
			},
			{
				Description: "Post a feature request",
				Arguments: map[string]any{
					"action":     "post",
					"session_id": "<uuid>",
					"content":    "We need streaming support for the data processing pipeline",
					"type":       "feature_request",
				},
				ExpectedResult: "Returns the new message ID",
			},
			{
				Description: "Check for new messages",
				Arguments: map[string]any{
					"action":     "check",
					"session_id": "<uuid>",
					"name":       "project-a-agent",
				},
				ExpectedResult: "Returns any messages posted since your last read",
			},
		},
		WhenToUse:    "Use when two AI agents need to coordinate across related projects - exchanging feature requests, implementation summaries, questions, or API change notifications.",
		WhenNotToUse: "Don't use for single-agent workflows, storing persistent project knowledge (use memory tool instead), or real-time communication requiring sub-second latency.",
	}
}

// toToolResult marshals a response to a tool result
func toToolResult(data any) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}
