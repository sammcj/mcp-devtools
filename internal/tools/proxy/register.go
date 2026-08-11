package proxy

import (
	"context"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// isProxyEnabled checks if the proxy tool is enabled via ENABLE_ADDITIONAL_TOOLS.
func isProxyEnabled() bool {
	return tools.IsToolEnabled("proxy")
}

// RegisterUpstreamTools attempts to connect to configured upstreams and register their tools.
// This function supports two modes controlled by the fastPath parameter:
//
//   - fastPath=true: Uses 5-second timeout, only succeeds if tokens are cached.
//     Intended for startup path to avoid blocking MCP server initialisation.
//
//   - fastPath=false: Uses 5-minute timeout, allows OAuth browser flow if needed.
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

// RegisterUpstreamToolsAsync discovers upstream tools in the background and
// registers them directly on the running MCP server, using newHandler to build
// each tool's handler so proxied tools behave like built-in ones.
//
// This avoids blocking server startup while still making upstream tools available as
// soon as the connection succeeds. Stateless HTTP clients pick the new tools up on
// their next tools/list; long-lived stdio clients are notified by the SDK.
func RegisterUpstreamToolsAsync(ctx context.Context, mcpSrv *mcp.Server, mainLogger *logrus.Logger, newHandler func(name string) mcp.ToolHandler) {
	// Quick pre-checks before spawning goroutine (avoid needless goroutines)
	if !isProxyEnabled() {
		mainLogger.Debug("Proxy: not enabled in ENABLE_ADDITIONAL_TOOLS, skipping async upstream registration")
		return
	}
	if os.Getenv("PROXY_UPSTREAMS") == "" && os.Getenv("PROXY_URL") == "" {
		mainLogger.Debug("Proxy: not configured, skipping async upstream registration")
		return
	}

	go func() {
		logger := mainLogger

		timeout := 5 * time.Minute // Allow time for OAuth browser flow
		logger.WithField("timeout", timeout).Info("Proxy: starting async upstream tool registration")

		initCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		manager := GetGlobalProxyManager()
		if err := manager.EnsureInitialised(initCtx, logger); err != nil {
			logger.WithError(err).Debug("Proxy: async initialisation failed, fallback proxy tool available")
			return
		}

		upstreamTools := manager.GetAggregator().GetTools()
		if len(upstreamTools) == 0 {
			logger.Error("Proxy: no upstream tools found after async aggregation")
			return
		}

		registered := 0
		for i := range upstreamTools {
			tool := &upstreamTools[i]
			dynamicTool := &DynamicProxyTool{
				toolName:         tool.Name,
				originalToolName: tool.OriginalName,
				upstreamName:     tool.UpstreamName,
				description:      tool.Description,
				inputSchema:      tool.InputSchema,
				manager:          manager,
			}

			// Register in our internal registry (for GetTool lookups)
			registry.RegisterProxiedTool(dynamicTool)

			// Register directly on the running MCP server so clients see it
			// immediately, reusing the shared handler so proxied tools get the
			// same metrics, telemetry and error semantics as built-in tools.
			definition := dynamicTool.Definition()
			mcpSrv.AddTool(&definition, newHandler(tool.Name))
			registered++

			logger.WithFields(logrus.Fields{
				"name":     tool.Name,
				"upstream": tool.UpstreamName,
			}).Info("Proxy: async-registered upstream tool on running server")
		}

		logger.WithFields(logrus.Fields{
			"count":      len(upstreamTools),
			"registered": registered,
		}).Info("Proxy: async upstream tool registration complete")
	}()
}
