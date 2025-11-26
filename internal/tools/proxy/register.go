package proxy

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// isProxyEnabled checks if the proxy tool is enabled via ENABLE_ADDITIONAL_TOOLS.
func isProxyEnabled() bool {
	enabledTools := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	if enabledTools == "" {
		return false
	}

	// Check if "all" is specified to enable all tools
	if strings.TrimSpace(strings.ToLower(enabledTools)) == "all" {
		return true
	}

	// Split by comma and check each tool
	for tool := range strings.SplitSeq(enabledTools, ",") {
		// Normalise: trim spaces, lowercase, replace underscores with hyphens
		normalisedTool := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tool), "_", "-"))
		if normalisedTool == "proxy" {
			return true
		}
	}

	return false
}

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

	// Check if proxy tool is enabled via ENABLE_ADDITIONAL_TOOLS
	if !isProxyEnabled() {
		logger.Debug("Proxy: not enabled in ENABLE_ADDITIONAL_TOOLS, skipping upstream tool registration")
		return false
	}

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

	// Log detailed tool information
	for i := range tools {
		logger.WithFields(logrus.Fields{
			"name":     tools[i].Name,
			"upstream": tools[i].UpstreamName,
			"original": tools[i].OriginalName,
			"mode":     mode,
		}).Debug("Proxy: upstream tool fetched")
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

	return true
}
