package collab

import (
	"context"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/sirupsen/logrus"
)

// sendMessageNotification sends a best-effort MCP notification when a new message is posted.
// This is useful when both agents connect to the same MCP server instance (HTTP transport).
// For separate instances (stdio), the filesystem remains the cross-instance transport.
func sendMessageNotification(ctx context.Context, logger *logrus.Logger, sessionID, from string, _ int) {
	srv := mcpserver.ServerFromContext(ctx)
	if srv == nil {
		logger.Debug("No MCP server in context, skipping collab notification")
		return
	}

	err := srv.SendNotificationToClient(ctx, "notifications/message", map[string]any{
		"level":  "info",
		"logger": "collab",
		"data":   "New message in collaboration session " + sessionID + " from " + from,
	})
	if err != nil {
		logger.WithError(err).Debug("Failed to send collab message notification")
	}
}
