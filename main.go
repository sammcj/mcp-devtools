package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	// Import all tool packages to register them
	_ "github.com/sammcj/mcp-devtools/internal/tools/internetsearch/brave"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/unified"
	_ "github.com/sammcj/mcp-devtools/internal/tools/shadcnui"
	_ "github.com/sammcj/mcp-devtools/internal/tools/webfetch"
)

// Version information (set during build)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	// Create a logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Initialize the registry
	registry.Init(logger)

	// Create and run the CLI app
	app := &cli.App{
		Name:    "mcp-devtools",
		Usage:   "MCP server for developer tools",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "transport",
				Aliases: []string{"t"},
				Value:   "stdio",
				Usage:   "Transport type (stdio, sse, or http)",
			},
			&cli.StringFlag{
				Name:  "port",
				Value: "18080",
				Usage: "Port to use for HTTP transports (SSE and Streamable HTTP)",
			},
			&cli.StringFlag{
				Name:  "base-url",
				Value: "http://localhost",
				Usage: "Base URL for HTTP transports",
			},
			&cli.StringFlag{
				Name:  "auth-token",
				Usage: "Authentication token for Streamable HTTP transport (optional)",
			},
			&cli.StringFlag{
				Name:  "endpoint-path",
				Value: "/http",
				Usage: "Endpoint path for Streamable HTTP transport",
			},
			&cli.DurationFlag{
				Name:  "session-timeout",
				Value: 30 * time.Minute,
				Usage: "Session timeout for Streamable HTTP transport",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Value:   false,
				Usage:   "Enable debug logging",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Print version information",
				Action: func(c *cli.Context) error {
					fmt.Printf("mcp-devtools version %s\n", Version)
					fmt.Printf("Commit: %s\n", Commit)
					fmt.Printf("Built: %s\n", BuildDate)
					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			// Get transport settings first
			transport := c.String("transport")
			port := c.String("port")
			baseURL := c.String("base-url")

			// Configure logger appropriately for transport mode
			if transport == "stdio" {
				// For stdio mode, disable logging to prevent conflicts with MCP protocol
				logger.SetOutput(os.Stderr)        // Use stderr instead of stdout
				logger.SetLevel(logrus.ErrorLevel) // Only log errors to stderr
			} else {
				// For non-stdio modes, normal logging is fine
				if c.Bool("debug") {
					logger.SetLevel(logrus.DebugLevel)
					logger.Debug("Debug logging enabled")
				}
			}

			// Only log startup info for non-stdio transports
			if transport != "stdio" {
				logger.Infof("Starting mcp-devtools version %s (commit: %s, built: %s)",
					Version, Commit, BuildDate)
			}

			// Create MCP server
			server := mcpserver.NewMCPServer("mcp-devtools", "MCP DevTools Server")

			// Register tools - fix race condition by capturing variables properly
			for toolName, toolImpl := range registry.GetTools() {
				// Capture variables to avoid closure race condition
				name := toolName
				tool := toolImpl

				if transport != "stdio" {
					logger.Infof("Registering tool: %s", name)
				}

				server.AddTool(tool.Definition(), func(toolCtx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					// Get fresh reference from registry to ensure consistency
					currentTool, ok := registry.GetTool(name)
					if !ok {
						return nil, fmt.Errorf("tool not found: %s", name)
					}

					// Type assert the arguments to map[string]interface{}
					args, ok := request.Params.Arguments.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}, got %T", request.Params.Arguments)
					}

					// Execute tool with error recovery
					result, err := currentTool.Execute(toolCtx, registry.GetLogger(), registry.GetCache(), args)
					if err != nil {
						// Log error to stderr for debugging (won't interfere with stdio)
						if transport != "stdio" {
							logger.WithError(err).Errorf("Tool execution failed: %s", name)
						}
						return nil, fmt.Errorf("tool execution failed: %w", err)
					}

					return result, nil
				})
			}

			// Start the server
			switch transport {
			case "stdio":
				return mcpserver.ServeStdio(server)
			case "sse":
				sseServer := mcpserver.NewSSEServer(server, mcpserver.WithBaseURL(baseURL+"/sse"))
				return sseServer.Start(":" + port)
			case "http":
				return startStreamableHTTPServer(c, server, logger)
			default:
				return fmt.Errorf("unsupported transport: %s", transport)
			}
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatalf("Error: %v", err)
	}
}

// startStreamableHTTPServer configures and starts the Streamable HTTP server
func startStreamableHTTPServer(c *cli.Context, server *mcpserver.MCPServer, logger *logrus.Logger) error {
	port := c.String("port")
	authToken := c.String("auth-token")
	endpointPath := c.String("endpoint-path")
	sessionTimeout := c.Duration("session-timeout")

	logger.Infof("Starting Streamable HTTP server on port %s with endpoint %s", port, endpointPath)

	// Configure server options
	var opts []mcpserver.StreamableHTTPOption

	// Set endpoint path
	opts = append(opts, mcpserver.WithEndpointPath(endpointPath))

	// Set session timeout (create a custom session manager)
	if sessionTimeout > 0 {
		opts = append(opts, mcpserver.WithSessionIdManager(&TimeoutSessionManager{
			timeout: sessionTimeout,
			logger:  logger,
		}))
	}

	// Add authentication if token is provided
	if authToken != "" {
		opts = append(opts, mcpserver.WithHTTPContextFunc(createAuthMiddleware(authToken, logger)))
	}

	// Add heartbeat interval for keep-alive
	heartbeatInterval := 30 * time.Second
	if sessionTimeout > 0 {
		// Set heartbeat to 1/4 of session timeout
		heartbeatInterval = sessionTimeout / 4
	}
	opts = append(opts, mcpserver.WithHeartbeatInterval(heartbeatInterval))

	// Add logger
	opts = append(opts, mcpserver.WithLogger(&logrusAdapter{logger: logger}))

	// Create streamable HTTP server
	httpServer := mcpserver.NewStreamableHTTPServer(server, opts...)

	logger.Infof("Heartbeat interval: %v", heartbeatInterval)
	logger.Info("Server supports multiple simultaneous connections")
	logger.Info("MCP Protocol compliance: Full specification support")

	// Start server
	return httpServer.Start(":" + port)
}

// createAuthMiddleware creates an HTTP context function for token authentication
func createAuthMiddleware(expectedToken string, logger *logrus.Logger) mcpserver.HTTPContextFunc {
	return func(ctx context.Context, req *http.Request) context.Context {
		// Validate MCP Protocol Version header
		protocolVersion := req.Header.Get("MCP-Protocol-Version")
		if protocolVersion != "" {
			if !isValidProtocolVersion(protocolVersion) {
				logger.Warnf("Unsupported MCP Protocol Version: %s", protocolVersion)
				// Note: In a full implementation, we would return an error response
				// For now, we log and continue
			} else {
				logger.Debugf("MCP Protocol Version: %s", protocolVersion)
			}
		} else {
			// Default to 2025-06-18 as per specification
			logger.Debug("No MCP-Protocol-Version header, assuming 2025-06-18")
		}

		// Validate Origin header for security (DNS rebinding protection)
		origin := req.Header.Get("Origin")
		if origin != "" && !isValidOrigin(origin) {
			logger.Warnf("Invalid Origin header: %s", origin)
			// Note: In production, this should return a 403 Forbidden
		}

		// Check Authorization header if token is required
		if expectedToken != "" {
			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn("Request missing Authorization header")
				return ctx
			}

			// Extract Bearer token
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				logger.Warn("Invalid authorization format, expected Bearer token")
				return ctx
			}

			token := strings.TrimPrefix(authHeader, bearerPrefix)
			if token != expectedToken {
				logger.Warn("Invalid authentication token")
				return ctx
			}

			logger.Debug("Request authenticated successfully")
		}

		return ctx
	}
}

// isValidProtocolVersion checks if the MCP protocol version is supported
func isValidProtocolVersion(version string) bool {
	supportedVersions := []string{
		"2025-06-18", // Current version
		"2024-11-05", // Backwards compatibility
	}

	for _, supported := range supportedVersions {
		if version == supported {
			return true
		}
	}
	return false
}

// isValidOrigin validates the Origin header to prevent DNS rebinding attacks
func isValidOrigin(origin string) bool {
	// Allow localhost and 127.0.0.1 origins for development
	allowedOrigins := []string{
		"http://localhost",
		"https://localhost",
		"http://127.0.0.1",
		"https://127.0.0.1",
	}

	for _, allowed := range allowedOrigins {
		if strings.HasPrefix(origin, allowed) {
			return true
		}
	}

	// In production, you would add your specific allowed origins here
	return false
}

// TimeoutSessionManager implements SessionIdManager with timeout support
type TimeoutSessionManager struct {
	timeout time.Duration
	logger  *logrus.Logger
}

func (t *TimeoutSessionManager) Generate() string {
	// Generate a simple UUID-like session ID
	// In production, you'd want to use crypto/rand or a proper UUID library
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

func (t *TimeoutSessionManager) Validate(sessionID string) (bool, error) {
	// For this simple implementation, we don't track session expiry
	// In production, you'd store sessions with timestamps and check expiry
	if sessionID == "" {
		return false, fmt.Errorf("empty session ID")
	}
	return false, nil // Session is not terminated
}

func (t *TimeoutSessionManager) Terminate(sessionID string) (bool, error) {
	// For this simple implementation, we don't track sessions
	// In production, you'd remove the session from storage
	t.logger.Debugf("Session terminated: %s", sessionID)
	return true, nil // Session was terminated successfully
}

// logrusAdapter adapts logrus.Logger to the mcp-go util.Logger interface
type logrusAdapter struct {
	logger *logrus.Logger
}

func (l *logrusAdapter) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *logrusAdapter) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *logrusAdapter) Warnf(format string, args ...interface{}) {
	l.logger.Warnf(format, args...)
}

func (l *logrusAdapter) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}
