package proxy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// RegisterUpstreamTools attempts to connect to configured upstreams and register their tools.
// This function supports two modes controlled by the fastPath parameter:
//
//   - fastPath=true: Uses 5-second timeout, only succeeds if tokens are cached.
//     Intended for startup path to avoid blocking MCP server initialisation.
//
//   - fastPath=false: Uses 30-second timeout, allows OAuth flow if needed.
//     Intended for background goroutine registration after server starts.
//
// Returns true if tools were successfully registered, false otherwise.
// The fallback ProxyTool is registered via init() and is always available.
func RegisterUpstreamTools(ctx context.Context, fastPath bool) bool {
	logger := logrus.StandardLogger()

	// Check if proxy is configured
	if os.Getenv("PROXY_UPSTREAMS") == "" && os.Getenv("PROXY_URL") == "" {
		logger.Debug("Proxy: not configured, skipping upstream tool registration")
		return false
	}

	// Determine timeout based on mode
	timeout := 5 * time.Minute // Allow time for OAuth browser flow
	mode := "background"
	if fastPath {
		timeout = 5 * time.Second
		mode = "fast-path"
	}

	logger.WithFields(logrus.Fields{
		"mode":    mode,
		"timeout": timeout,
	}).Info("Proxy: attempting to register upstream tools")

	// Create a context with timeout for connection attempt
	initCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get the global proxy manager
	manager := GetGlobalProxyManager()

	// Try to initialise (this will connect to upstreams)
	if err := manager.EnsureInitialised(initCtx, logger); err != nil {
		logger.WithFields(logrus.Fields{
			"mode":  mode,
			"error": err,
		}).Debug("Proxy: failed to initialise during registration, fallback tool available")
		return false
	}

	// Get all upstream tools
	tools := manager.GetAggregator().GetTools()
	logger.WithFields(logrus.Fields{
		"count": len(tools),
		"mode":  mode,
	}).Info("Proxy: fetched upstream tools from aggregator")

	// Write detailed info to debug file
	if homeDir, err := os.UserHomeDir(); err == nil {
		logPath := filepath.Join(homeDir, ".mcp-devtools", "proxy-registration.log")
		if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); err == nil {
			fmt.Fprintf(logFile, "\n[%s] Registration attempt - fetched %d tools from aggregator\n",
				time.Now().Format("2006-01-02 15:04:05"), len(tools))
			for i := range tools {
				fmt.Fprintf(logFile, "  - %s (upstream: %s, original: %s)\n",
					tools[i].Name, tools[i].UpstreamName, tools[i].OriginalName)
			}
			logFile.Close()
		}
	}

	if len(tools) == 0 {
		logger.WithField("mode", mode).Error("Proxy: no upstream tools found after aggregation")
		return false
	}

	// Register each upstream tool as a separate tool
	// Use ForceRegister to bypass enablement checks since proxy itself is already enabled
	registered := 0
	for i := range tools {
		tool := &tools[i]
		dynamicTool := &DynamicProxyTool{
			toolName:         tool.Name,
			originalToolName: tool.OriginalName,
			upstreamName:     tool.UpstreamName,
			description:      tool.Description,
			inputSchema:      tool.InputSchema,
			manager:          manager,
		}
		registry.RegisterProxiedTool(dynamicTool)
		registered++

		logger.WithFields(logrus.Fields{
			"name":     tool.Name,
			"upstream": tool.UpstreamName,
			"mode":     mode,
		}).Info("Proxy: force-registered upstream tool")
	}

	logger.WithFields(logrus.Fields{
		"count":      len(tools),
		"registered": registered,
		"mode":       mode,
	}).Info("Proxy: successfully registered upstream tools")

	// Write success to debug file
	if homeDir, err := os.UserHomeDir(); err == nil {
		logPath := filepath.Join(homeDir, ".mcp-devtools", "proxy-registration.log")
		if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); err == nil {
			fmt.Fprintf(logFile, "[%s] Successfully registered %d tools\n",
				time.Now().Format("2006-01-02 15:04:05"), registered)
			logFile.Close()
		}
	}

	return true
}
