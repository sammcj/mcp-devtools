package collab

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

const (
	defaultWaitTimeout  = 600 // 10 minutes
	maxWaitTimeout      = 3600
	defaultPollInterval = 60 // 1 minute
	minPollInterval     = 5
	maxPollInterval     = 300 // 5 minutes
)

// CollabWaitTool implements the collab_wait tool as a regular (non-task) tool.
// It blocks internally, polling the filesystem until new messages arrive or timeout.
//
// If MCP clients gain broad task augmentation support in future, this could be
// converted back to a task tool for true async behaviour.
type CollabWaitTool struct {
	storageOnce sync.Once
	storage     *Storage
	storageErr  error
}

func init() {
	registry.Register(&CollabWaitTool{})
}

// Definition returns the tool definition for collab_wait
func (w *CollabWaitTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"collab_wait",
		mcp.WithDescription(`Wait for new messages in a collaboration session. Returns when new messages arrive or timeout is reached.

Use when you want to wait for a response from another agent without polling manually. Blocks until new messages arrive, then returns. Call collab with action=check afterwards to retrieve the new messages.`),

		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("Session UUID to watch for new messages"),
		),

		mcp.WithNumber("timeout_seconds",
			mcp.Description("Maximum time to wait in seconds (default: 600, max: 3600)"),
		),

		mcp.WithNumber("poll_interval_seconds",
			mcp.Description("How often to check for new messages in seconds (default: 60, min: 5). Can also be set via COLLAB_POLL_INTERVAL env var"),
		),

		mcp.WithString("name",
			mcp.Description("Participant name. If provided, returns immediately when unread messages exist (based on last_read position) instead of waiting for new ones"),
		),

		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

// Execute blocks until new messages arrive in the session or timeout is reached.
func (w *CollabWaitTool) Execute(ctx context.Context, logger *logrus.Logger, _ *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Initialise storage lazily (thread-safe)
	w.storageOnce.Do(func() {
		w.storage, w.storageErr = NewStorage(logger)
	})
	if w.storageErr != nil {
		return nil, fmt.Errorf("failed to initialise collab storage: %w", w.storageErr)
	}

	// Parse and validate session_id
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("missing required parameter: session_id")
	}
	if _, err := uuid.Parse(sessionID); err != nil {
		return nil, fmt.Errorf("invalid session_id: must be a valid UUID")
	}

	// Parse timeout
	timeoutSec := intFromArgs(args, "timeout_seconds", defaultWaitTimeout)
	if timeoutSec <= 0 {
		timeoutSec = defaultWaitTimeout
	}
	if timeoutSec > maxWaitTimeout {
		timeoutSec = maxWaitTimeout
	}

	// Resolve poll interval (parameter > env var > default)
	pollSec := resolvePollInterval(args)
	if pollSec > timeoutSec {
		pollSec = timeoutSec
	}

	// Validate session exists and determine baseline
	currentCount, err := w.storage.GetMessageCount(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// If a participant name is provided, use their last_read as the baseline.
	// This means collab_wait returns immediately if there are already unread messages.
	baseline := currentCount
	if name, ok := args["name"].(string); ok && name != "" {
		if participant, valErr := validateParticipantName(name); valErr == nil {
			session, loadErr := w.storage.LoadSession(sessionID)
			if loadErr == nil {
				if p, exists := session.Participants[participant]; exists && p.LastRead < currentCount {
					// There are already unread messages -- return immediately
					unread := currentCount - p.LastRead
					logger.WithFields(logrus.Fields{
						"session_id": sessionID,
						"unread":     unread,
						"participant": participant,
					}).Debug("collab_wait found existing unread messages")

					resp := waitResponse{
						SessionID: sessionID,
						Status:    "new_messages",
						NewCount:  unread,
						Message:   fmt.Sprintf("%d unread message(s) found. Use collab check to read them.", unread),
					}
					return toToolResult(resp)
				}
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"session_id":    sessionID,
		"baseline":      baseline,
		"timeout":       timeoutSec,
		"poll_interval": pollSec,
	}).Debug("Starting collab_wait polling")

	// Poll for new messages
	timeout := time.Duration(timeoutSec) * time.Second
	poll := time.Duration(pollSec) * time.Second
	deadline := time.After(timeout)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-deadline:
			resp := waitResponse{
				SessionID: sessionID,
				Status:    "timeout",
				NewCount:  0,
				Message:   fmt.Sprintf("No new messages after %d seconds.", timeoutSec),
			}
			return toToolResult(resp)

		case <-ticker.C:
			currentCount, err := w.storage.GetMessageCount(sessionID)
			if err != nil {
				logger.WithError(err).Debug("Failed to poll session message count")
				continue
			}

			if currentCount > baseline {
				newCount := currentCount - baseline
				logger.WithFields(logrus.Fields{
					"session_id": sessionID,
					"new_count":  newCount,
				}).Debug("collab_wait detected new messages")

				resp := waitResponse{
					SessionID: sessionID,
					Status:    "new_messages",
					NewCount:  newCount,
					Message:   fmt.Sprintf("%d new message(s) detected. Use collab check to read them.", newCount),
				}
				return toToolResult(resp)
			}
		}
	}
}

// resolvePollInterval determines the poll interval from args, env var, or default.
// Priority: tool parameter > COLLAB_POLL_INTERVAL env var > defaultPollInterval.
func resolvePollInterval(args map[string]any) int {
	// 1. Check tool parameter
	if v, ok := args["poll_interval_seconds"]; ok {
		switch val := v.(type) {
		case float64:
			if int(val) >= minPollInterval {
				return min(int(val), maxPollInterval)
			}
		case int:
			if val >= minPollInterval {
				return min(val, maxPollInterval)
			}
		}
	}

	// 2. Check environment variable
	if envVal := os.Getenv("COLLAB_POLL_INTERVAL"); envVal != "" {
		if parsed, err := strconv.Atoi(envVal); err == nil && parsed >= minPollInterval {
			return min(parsed, maxPollInterval)
		}
	}

	// 3. Default
	return defaultPollInterval
}

// intFromArgs extracts an integer from tool arguments with a default fallback.
func intFromArgs(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	default:
		return defaultVal
	}
}
