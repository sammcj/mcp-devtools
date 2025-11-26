package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	oauthclient "github.com/sammcj/mcp-devtools/internal/oauth/client"
	oauthserver "github.com/sammcj/mcp-devtools/internal/oauth/server"
	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	// Import all tool packages to register them
	_ "github.com/sammcj/mcp-devtools/internal/imports"
	coderename "github.com/sammcj/mcp-devtools/internal/tools/code_rename"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy"
)

// Version information (set during build)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Global resources that need cleanup
// Using atomic operations to prevent race conditions between signal handlers and cleanup
var (
	debugLogFile atomic.Pointer[os.File]
	isStdioMode  atomic.Bool
)

const (
	// DefaultMemoryLimit is the default memory limit for the Go application (5GB)
	DefaultMemoryLimit = 5 * 1024 * 1024 * 1024
)

// parseLogLevel parses the LOG_LEVEL environment variable and returns the appropriate logrus level.
// Defaults to WarnLevel if not set or invalid.
func parseLogLevel() logrus.Level {
	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr == "" {
		return logrus.WarnLevel // Default to warn
	}

	// Normalise to lowercase for comparison
	logLevelStr = strings.ToLower(strings.TrimSpace(logLevelStr))

	switch logLevelStr {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	case "panic":
		return logrus.PanicLevel
	default:
		// Invalid value, default to warn
		return logrus.WarnLevel
	}
}

// setMemoryLimit configures the Go runtime memory limit
func setMemoryLimit() {
	// Check for environment variable override
	memLimitStr := os.Getenv("MCP_DEVTOOLS_MEMORY_LIMIT")
	var memLimit int64 = DefaultMemoryLimit

	if memLimitStr != "" {
		if parsed, err := strconv.ParseInt(memLimitStr, 10, 64); err == nil && parsed > 0 {
			memLimit = parsed
		}
	}

	// Set the GOMEMLIMIT for the Go runtime
	// This is a soft limit - Go will try to keep memory usage under this value
	// The Go runtime will automatically adjust GC behaviour to stay under this limit
	debug.SetMemoryLimit(memLimit)
}

func main() {
	// Set memory limit for the Go application
	setMemoryLimit()

	// Create context with signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create a logger with default configuration
	// Initially discard output - will be reconfigured in Action based on transport mode
	logger := logrus.New()
	logger.SetOutput(io.Discard)     // Prevent any early logging before we know the transport mode
	logger.SetLevel(parseLogLevel()) // Use LOG_LEVEL env var (default: WarnLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Initialise the registry
	registry.Init(logger)

	// Ensure cleanup runs on normal exit OR signal
	defer performCleanup(logger)

	// Register upstream proxy tools BEFORE creating MCP server
	// This can block for OAuth authentication - that's fine, we haven't started the server yet
	// MCP clients will wait during connection, which is normal behaviour
	if err := registerUpstreamProxyTools(ctx); err != nil {
		logger.WithError(err).Error("Proxy: upstream registration failed (fallback proxy tool available)")
	} else {
		// Check if proxy was configured
		if os.Getenv("PROXY_UPSTREAMS") != "" || os.Getenv("PROXY_URL") != "" {
			logger.Info("Proxy: upstream tools registered successfully")
		}
	}

	// Create and run the CLI app
	app := &cli.Command{
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
			// OAuth 2.0/2.1 flags
			&cli.BoolFlag{
				Name:    "oauth-enabled",
				Usage:   "Enable OAuth 2.0/2.1 authorisation (HTTP transport only)",
				Sources: cli.EnvVars("OAUTH_ENABLED", "MCP_OAUTH_ENABLED"),
			},
			&cli.StringFlag{
				Name:    "oauth-issuer",
				Usage:   "OAuth issuer URL (required if oauth-enabled)",
				Sources: cli.EnvVars("OAUTH_ISSUER", "MCP_OAUTH_ISSUER"),
			},
			&cli.StringFlag{
				Name:    "oauth-audience",
				Usage:   "OAuth audience for this resource server",
				Sources: cli.EnvVars("OAUTH_AUDIENCE", "MCP_OAUTH_AUDIENCE"),
			},
			&cli.StringFlag{
				Name:    "oauth-jwks-url",
				Usage:   "JWKS URL for token validation",
				Sources: cli.EnvVars("OAUTH_JWKS_URL", "MCP_OAUTH_JWKS_URL"),
			},
			&cli.BoolFlag{
				Name:    "oauth-dynamic-registration",
				Usage:   "Enable RFC7591 dynamic client registration",
				Sources: cli.EnvVars("OAUTH_DYNAMIC_REGISTRATION", "MCP_OAUTH_DYNAMIC_REGISTRATION"),
			},
			&cli.StringFlag{
				Name:    "oauth-authorization-server",
				Usage:   "Authorisation server URL (if different from issuer)",
				Sources: cli.EnvVars("OAUTH_AUTHORIZATION_SERVER", "MCP_OAUTH_AUTHORIZATION_SERVER"),
			},
			&cli.BoolFlag{
				Name:    "oauth-require-https",
				Value:   true,
				Usage:   "Require HTTPS for OAuth endpoints (disable only for development)",
				Sources: cli.EnvVars("OAUTH_REQUIRE_HTTPS", "MCP_OAUTH_REQUIRE_HTTPS"),
			},
			// OAuth Client Browser Authentication flags
			&cli.BoolFlag{
				Name:    "oauth-browser-auth",
				Usage:   "Enable browser-based OAuth authentication flow at startup",
				Sources: cli.EnvVars("OAUTH_BROWSER_AUTH", "MCP_OAUTH_BROWSER_AUTH"),
			},
			&cli.StringFlag{
				Name:    "oauth-client-id",
				Usage:   "OAuth client ID for browser authentication",
				Sources: cli.EnvVars("OAUTH_CLIENT_ID", "MCP_OAUTH_CLIENT_ID"),
			},
			&cli.StringFlag{
				Name:    "oauth-client-secret",
				Usage:   "OAuth client secret for browser authentication (optional for public clients)",
				Sources: cli.EnvVars("OAUTH_CLIENT_SECRET", "MCP_OAUTH_CLIENT_SECRET"),
			},
			&cli.StringFlag{
				Name:    "oauth-scope",
				Usage:   "OAuth scopes to request during browser authentication",
				Sources: cli.EnvVars("OAUTH_SCOPE", "MCP_OAUTH_SCOPE"),
			},
			&cli.IntFlag{
				Name:    "oauth-callback-port",
				Value:   0,
				Usage:   "Port for OAuth callback server (0 for random port)",
				Sources: cli.EnvVars("OAUTH_CALLBACK_PORT", "MCP_OAUTH_CALLBACK_PORT"),
			},
			&cli.DurationFlag{
				Name:    "oauth-auth-timeout",
				Value:   5 * time.Minute,
				Usage:   "Timeout for browser authentication flow",
				Sources: cli.EnvVars("OAUTH_AUTH_TIMEOUT", "MCP_OAUTH_AUTH_TIMEOUT"),
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Print version information",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Printf("mcp-devtools version %s\n", Version)
					fmt.Printf("Commit: %s\n", Commit)
					fmt.Printf("Built: %s\n", BuildDate)
					return nil
				},
			},
			{
				Name:  "security-config-diff",
				Usage: "Show differences between user security config and default config",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "update",
						Usage: "Update user config with new default rules (preserves user customizations)",
					},
					&cli.StringFlag{
						Name:  "config-path",
						Usage: "Path to security configuration file (default: ~/.mcp-devtools/security.yaml)",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return handleSecurityConfigDiff(cmd)
				},
			},
			{
				Name:  "security-config-validate",
				Usage: "Validate security configuration file for errors",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config-path",
						Usage: "Path to security configuration file (default: ~/.mcp-devtools/security.yaml)",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return handleSecurityConfigValidate(cmd)
				},
			},
		},
		Action: func(cliCtx context.Context, cmd *cli.Command) error {
			// Get transport settings
			transport := cmd.String("transport")
			port := cmd.String("port")
			baseURL := cmd.String("base-url")

			// Track stdio mode for error handling (atomic to prevent races with signal handlers)
			isStdioMode.Store(transport == "stdio")

			// Configure logger - ALWAYS use file logging to avoid breaking stdio protocol
			homeDir, err := os.UserHomeDir()
			if err == nil {
				logDir := filepath.Join(homeDir, ".mcp-devtools", "logs")
				if err := os.MkdirAll(logDir, 0700); err == nil {
					logFile := filepath.Join(logDir, "mcp-devtools.log")
					if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); err == nil {
						// Store file handle for cleanup
						debugLogFile.Store(file)
						// Configure loggers for file output
						logger.SetOutput(file)
						logrus.SetOutput(file)
						// Apply LOG_LEVEL setting (stdio mode uses warn level minimum)
						logLevel := parseLogLevel()
						if isStdioMode.Load() && logLevel < logrus.WarnLevel {
							logLevel = logrus.WarnLevel // Minimum warn level for stdio mode
						}
						logger.SetLevel(logLevel)
						logrus.SetLevel(logLevel)
						logger.WithField("level", logLevel.String()).Debug("Logging configured")
					} else {
						// Critical: Cannot create log file - use io.Discard in stdio mode to prevent protocol breakage
						if isStdioMode.Load() {
							logger.SetOutput(io.Discard)
							logrus.SetOutput(io.Discard)
						} else {
							// Non-stdio mode can fallback to stderr
							logger.SetOutput(os.Stderr)
							logrus.SetOutput(os.Stderr)
						}
						logLevel := parseLogLevel()
						logger.SetLevel(logLevel)
						logrus.SetLevel(logLevel)
					}
				} else {
					// Critical: Cannot create log directory
					if isStdioMode.Load() {
						logger.SetOutput(io.Discard)
						logrus.SetOutput(io.Discard)
					} else {
						logger.SetOutput(os.Stderr)
						logrus.SetOutput(os.Stderr)
					}
					logLevel := parseLogLevel()
					logger.SetLevel(logLevel)
					logrus.SetLevel(logLevel)
				}
			} else {
				// Critical: Cannot get home directory
				if isStdioMode.Load() {
					logger.SetOutput(io.Discard)
					logrus.SetOutput(io.Discard)
				} else {
					logger.SetOutput(os.Stderr)
					logrus.SetOutput(os.Stderr)
				}
				logLevel := parseLogLevel()
				logger.SetLevel(logLevel)
				logrus.SetLevel(logLevel)
			}

			// Initialise tool error logger after logging is configured
			if err := tools.InitGlobalErrorLogger(logger); err != nil {
				logger.WithError(err).Debug("Failed to initialise tool error logger")
				if transport != "stdio" {
					logger.WithError(err).Warn("Failed to initialise tool error logger")
				}
			}

			// Initialise security system (if enabled) - after logging is configured
			logger.Debug("Initializing security system")
			if err := security.InitGlobalSecurityManager(); err != nil {
				logger.WithError(err).Debug("Security initialisation failed")
				if transport != "stdio" {
					logger.WithError(err).Warn("Failed to initialise security system")
				}
			} else {
				logger.Debug("Security system initialised successfully")
			}

			// Only log startup info for non-stdio transports
			if transport != "stdio" {
				logger.Infof("Starting mcp-devtools version %s (commit: %s, built: %s)",
					Version, Commit, BuildDate)
			}

			// Create MCP server
			// Note: Upstream proxy tools are already registered in main() before CLI runs
			logger.Debug("Creating MCP server")
			mcpSrv := mcpserver.NewMCPServer("mcp-devtools", "MCP DevTools Server")

			enabledTools := registry.GetEnabledTools()
			logger.WithField("tool_count", len(enabledTools)).Debug("MCP server created, registering tools")

			// Register tools - fix race condition by capturing variables properly
			for toolName, toolImpl := range enabledTools {
				// Capture variables to avoid closure race condition
				name := toolName
				tool := toolImpl

				if transport != "stdio" {
					logger.Infof("Registering tool: %s", name)
				}

				mcpSrv.AddTool(tool.Definition(), func(toolCtx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					// Get fresh reference from registry to ensure consistency
					currentTool, ok := registry.GetTool(name)
					if !ok {
						return nil, fmt.Errorf("tool not found: %s", name)
					}

					// Type assert the arguments to map[string]interface{}
					args, ok := request.Params.Arguments.(map[string]any)
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

						// Log tool error to file if enabled
						if errorLogger := tools.GetGlobalErrorLogger(); errorLogger != nil && errorLogger.IsEnabled() {
							errorLogger.LogToolError(name, args, err, transport)
						}

						return nil, fmt.Errorf("tool execution failed: %w", err)
					}

					return result, nil
				})
			}

			// Handle browser-based OAuth authentication if enabled
			if cmd.Bool("oauth-browser-auth") {
				if err := handleBrowserAuthentication(cmd, transport, logger); err != nil {
					return fmt.Errorf("browser authentication failed: %w", err)
				}
			}

			// Start the server
			logger.WithField("transport", transport).Debug("Starting server")
			switch transport {
			case "stdio":
				logger.Debug("Starting stdio server")
				return mcpserver.ServeStdio(mcpSrv)
			case "sse":
				logger.WithField("port", port).Debug("Starting SSE server")
				sseServer := mcpserver.NewSSEServer(mcpSrv, mcpserver.WithBaseURL(baseURL+"/sse"))
				return sseServer.Start(":" + port)
			case "http":
				logger.WithField("port", port).Debug("Starting HTTP server")
				return startStreamableHTTPServer(cliCtx, cmd, mcpSrv, logger)
			default:
				return fmt.Errorf("unsupported transport: %s", transport)
			}
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		// CRITICAL: In stdio mode, we must NOT log to stdout or stderr as it breaks the MCP protocol
		// Even though this occurs after ServeStdio() returns, initialisation errors could occur
		// before the protocol starts, so we avoid all logging in stdio mode
		if !isStdioMode.Load() {
			logger.Fatalf("Error: %v", err)
		}
		os.Exit(1)
	}
}

// registerUpstreamProxyTools attempts to register upstream tools before MCP server starts.
// This can block for OAuth authentication, which is acceptable before server creation.
func registerUpstreamProxyTools(ctx context.Context) error {
	// Call proxy registration with no fast-path mode (allow full OAuth)
	if proxy.RegisterUpstreamTools(ctx, false) {
		return nil
	}
	return fmt.Errorf("upstream tools not registered")
}

// performCleanup handles cleanup of resources on shutdown
func performCleanup(logger *logrus.Logger) {
	// Close the debug log file if it was opened (atomic load to prevent races)
	if file := debugLogFile.Load(); file != nil {
		// Silently close - we're in cleanup and can't safely log errors
		// (stdio mode: no output allowed; non-stdio: logger might write to this file)
		_ = file.Close()
	}

	// Close the tool error logger if it was initialised
	if errorLogger := tools.GetGlobalErrorLogger(); errorLogger != nil {
		// Use Warn level - in stdio mode this won't output (ErrorLevel only)
		if err := errorLogger.Close(); err != nil {
			logger.WithError(err).Warn("Failed to close tool error logger")
		}
	}

	// Stop LSP client cleanup routine and close all cached LSP clients
	// Uses Debug level logging internally - won't output in stdio mode
	coderename.StopCleanupRoutine(registry.GetCache(), logger)
}

// startStreamableHTTPServer configures and starts the Streamable HTTP server with graceful shutdown
func startStreamableHTTPServer(ctx context.Context, cmd *cli.Command, mcpServer *mcpserver.MCPServer, logger *logrus.Logger) error {
	port := cmd.String("port")
	authToken := cmd.String("auth-token")
	endpointPath := cmd.String("endpoint-path")
	sessionTimeout := cmd.Duration("session-timeout")
	baseURL := cmd.String("base-url")

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

	// Check if OAuth is enabled
	oauthEnabled := cmd.Bool("oauth-enabled")
	if oauthEnabled {
		// Configure OAuth 2.1
		oauthConfig := &types.OAuth2Config{
			Enabled:             true,
			Issuer:              cmd.String("oauth-issuer"),
			Audience:            cmd.String("oauth-audience"),
			JWKSUrl:             cmd.String("oauth-jwks-url"),
			DynamicRegistration: cmd.Bool("oauth-dynamic-registration"),
			AuthorizationServer: cmd.String("oauth-authorization-server"),
			RequireHTTPS:        cmd.Bool("oauth-require-https"),
		}

		// Validate OAuth configuration
		if err := validateOAuthConfig(oauthConfig); err != nil {
			return fmt.Errorf("invalid OAuth configuration: %w", err)
		}

		// Create OAuth server
		fullBaseURL := fmt.Sprintf("%s:%s", baseURL, port)
		oauthServer, err := oauthserver.NewOAuth2Server(oauthConfig, fullBaseURL, logger)
		if err != nil {
			return fmt.Errorf("failed to create OAuth server: %w", err)
		}

		// Use OAuth middleware
		opts = append(opts, mcpserver.WithHTTPContextFunc(createOAuthMiddleware(oauthServer, logger)))

		logger.Info("OAuth 2.1 authentication enabled")
		logger.Infof("OAuth issuer: %s", oauthConfig.Issuer)
		logger.Infof("OAuth audience: %s", oauthConfig.Audience)
		logger.Infof("Dynamic client registration: %t", oauthConfig.DynamicRegistration)

		// Register OAuth endpoints
		httpServer := mcpserver.NewStreamableHTTPServer(mcpServer, opts...)

		// Get the underlying HTTP mux to register OAuth endpoints
		// Note: This is a simplified approach - in production you might need a more sophisticated setup
		mux := http.NewServeMux()

		// Register OAuth metadata endpoints
		oauthServer.RegisterHandlers(mux)

		// Register the main MCP endpoint
		mux.Handle(endpointPath, httpServer)

		// Start the server with custom mux and security timeouts
		logger.Infof("OAuth endpoints available at %s/.well-known/", fullBaseURL)
		server := &http.Server{
			Addr:           ":" + port,
			Handler:        mux,
			ReadTimeout:    30 * time.Second,  // Prevent slow loris attacks
			WriteTimeout:   30 * time.Second,  // Prevent slow writes
			IdleTimeout:    120 * time.Second, // Close idle connections
			MaxHeaderBytes: 1 << 20,           // 1MB max header size
		}

		// Start server in goroutine to allow graceful shutdown
		serverErr := make(chan error, 1)
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				// Use select to prevent blocking if context is cancelled
				select {
				case serverErr <- err:
				case <-ctx.Done():
					// Context cancelled, error no longer relevant
				}
			}
		}()

		// Wait for context cancellation or server error
		select {
		case err := <-serverErr:
			return fmt.Errorf("HTTP server failed: %w", err)
		case <-ctx.Done():
			logger.Info("Shutdown signal received, stopping HTTP server")
		}

		// Graceful shutdown with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.WithError(err).Error("HTTP server shutdown failed")
			return err
		}

		logger.Info("HTTP server stopped gracefully")
		return nil

	} else if authToken != "" {
		// Use legacy token authentication
		opts = append(opts, mcpserver.WithHTTPContextFunc(createAuthMiddleware(authToken, logger)))
		logger.Info("Legacy token authentication enabled")
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
	httpServer := mcpserver.NewStreamableHTTPServer(mcpServer, opts...)

	logger.Infof("Heartbeat interval: %v", heartbeatInterval)
	logger.Info("Server supports multiple simultaneous connections")
	logger.Info("MCP Protocol compliance: Full specification support")

	// Start server
	// Note: The mcp-go StreamableHTTPServer.Start() method doesn't currently support
	// context-based graceful shutdown. Consider using OAuth mode (which creates its own
	// http.Server) for production deployments requiring graceful shutdown.
	// TODO: Update when mcp-go library adds graceful shutdown support
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

	return slices.Contains(supportedVersions, version)
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

func (l *logrusAdapter) Debugf(format string, args ...any) {
	l.logger.Debugf(format, args...)
}

func (l *logrusAdapter) Infof(format string, args ...any) {
	l.logger.Infof(format, args...)
}

func (l *logrusAdapter) Warnf(format string, args ...any) {
	l.logger.Warnf(format, args...)
}

func (l *logrusAdapter) Errorf(format string, args ...any) {
	l.logger.Errorf(format, args...)
}

// validateOAuthConfig validates OAuth configuration
func validateOAuthConfig(config *types.OAuth2Config) error {
	if !config.Enabled {
		return fmt.Errorf("OAuth is not enabled")
	}

	if config.Issuer == "" {
		return fmt.Errorf("oauth-issuer is required when OAuth is enabled")
	}

	if config.Audience == "" {
		return fmt.Errorf("oauth-audience is required when OAuth is enabled")
	}

	if config.JWKSUrl == "" {
		return fmt.Errorf("oauth-jwks-url is required when OAuth is enabled")
	}

	return nil
}

// createOAuthMiddleware creates OAuth 2.1 authentication middleware
func createOAuthMiddleware(oauthServer *oauthserver.OAuth2Server, logger *logrus.Logger) func(context.Context, *http.Request) context.Context {
	return func(ctx context.Context, req *http.Request) context.Context {
		// Skip OAuth for metadata endpoints
		if strings.HasPrefix(req.URL.Path, "/.well-known/") || req.URL.Path == "/oauth/register" {
			logger.Debug("Skipping OAuth authentication for metadata endpoint")
			return ctx
		}

		// Authenticate the request
		result := oauthServer.AuthenticateRequest(ctx, req)

		if !result.Authenticated {
			logger.WithError(result.Error).Debug("OAuth authentication failed")
			// The authentication result will be handled by the OAuth middleware
			// We add a marker to the context to indicate authentication failure
			return context.WithValue(ctx, types.OAuthAuthFailedKey, result)
		}

		// Add claims to context for downstream handlers
		return context.WithValue(ctx, types.OAuthClaimsKey, result.Claims)
	}
}

// handleBrowserAuthentication handles the browser-based OAuth authentication flow
func handleBrowserAuthentication(cmd *cli.Command, transport string, logger *logrus.Logger) error {
	// Browser authentication is not compatible with stdio mode
	if transport == "stdio" {
		logger.Debug("Browser authentication disabled for stdio transport")
		return nil
	}

	// Validate required configuration
	clientID := cmd.String("oauth-client-id")
	if clientID == "" {
		return fmt.Errorf("oauth-client-id is required for browser authentication")
	}

	issuerURL := cmd.String("oauth-issuer")
	if issuerURL == "" {
		return fmt.Errorf("oauth-issuer is required for browser authentication")
	}

	// Build OAuth client configuration
	clientConfig := &oauthclient.OAuth2ClientConfig{
		ClientID:     clientID,
		ClientSecret: cmd.String("oauth-client-secret"),
		IssuerURL:    issuerURL,
		Scope:        cmd.String("oauth-scope"),
		ServerPort:   cmd.Int("oauth-callback-port"),
		AuthTimeout:  cmd.Duration("oauth-auth-timeout"),
		RequireHTTPS: cmd.Bool("oauth-require-https"),
	}

	// Set resource parameter for audience binding (RFC8707)
	audience := cmd.String("oauth-audience")
	if audience != "" {
		clientConfig.Resource = audience
	}

	// Create and validate browser authentication flow
	browserAuth, err := oauthclient.NewBrowserAuthFlow(clientConfig, logger)
	if err != nil {
		return fmt.Errorf("failed to create browser authentication flow: %w", err)
	}

	if err := browserAuth.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid browser authentication configuration: %w", err)
	}

	// Log authentication details
	logger.Info("Browser-based OAuth authentication enabled")
	logger.Infof("OAuth client ID: %s", clientConfig.ClientID)
	logger.Infof("OAuth issuer: %s", clientConfig.IssuerURL)
	if clientConfig.Scope != "" {
		logger.Infof("OAuth scope: %s", clientConfig.Scope)
	}
	if clientConfig.Resource != "" {
		logger.Infof("OAuth resource: %s", clientConfig.Resource)
	}

	// Perform the authentication
	logger.Info("Starting browser authentication flow...")
	logger.Info("Please complete the authentication in your browser")

	tokenResponse, err := browserAuth.AuthenticateWithTimeout(clientConfig.AuthTimeout)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if tokenResponse == nil {
		// This shouldn't happen, but let's be defensive
		return fmt.Errorf("authentication completed but no token received")
	}

	// Log successful authentication (without sensitive token data)
	logger.Info("Browser authentication completed successfully")
	logger.Infof("Token type: %s", tokenResponse.TokenType)
	if tokenResponse.ExpiresIn > 0 {
		logger.Infof("Token expires in: %d seconds", tokenResponse.ExpiresIn)
	}
	if tokenResponse.Scope != "" {
		logger.Infof("Granted scope: %s", tokenResponse.Scope)
	}

	// Store the access token for use by the MCP server
	// For now, we'll store it in an environment variable that the OAuth middleware can use
	// In a production implementation, you might want to use a more secure storage mechanism
	if err := os.Setenv("MCP_ACCESS_TOKEN", tokenResponse.AccessToken); err != nil {
		logger.WithError(err).Warn("Failed to store access token in environment")
	}

	logger.Info("MCP DevTools is now authenticated and ready to start")
	return nil
}

// handleSecurityConfigDiff compares user config against default config and optionally updates it
func handleSecurityConfigDiff(cmd *cli.Command) error {
	// Get config path
	configPath := cmd.String("config-path")
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = fmt.Sprintf("%s/.mcp-devtools/security.yaml", homeDir)
	}

	// Check if user config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("User config file does not exist at: %s\n", configPath)
		fmt.Println("A default configuration will be created when the security system is first used.")
		return nil
	}

	// Generate default config
	defaultConfig := security.GenerateDefaultConfig()

	// Read user config
	userConfigData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read user config: %w", err)
	}

	// Compare configs
	userConfigStr := string(userConfigData)
	if userConfigStr == defaultConfig {
		fmt.Println("✅ User configuration matches the current default configuration")
		return nil
	}

	fmt.Println("📋 Configuration Differences Found")
	fmt.Println("==================================")
	fmt.Printf("User config: %s\n", configPath)
	fmt.Println("Default config: (generated)")
	fmt.Println()

	// Show basic comparison
	fmt.Println("User config size:", len(userConfigStr), "bytes")
	fmt.Println("Default config size:", len(defaultConfig), "bytes")
	fmt.Println()

	// Parse both configs to show structural differences
	var userRules security.SecurityRules
	var defaultRules security.SecurityRules

	if err := yaml.Unmarshal(userConfigData, &userRules); err != nil {
		fmt.Printf("⚠️  Warning: User config has parsing errors: %v\n", err)
		fmt.Println("Run 'security-config-validate' command for detailed error information")
	} else {
		if err := yaml.Unmarshal([]byte(defaultConfig), &defaultRules); err != nil {
			return fmt.Errorf("failed to parse default config: %w", err)
		}

		// Compare versions
		if userRules.Version != defaultRules.Version {
			fmt.Printf("📄 Version difference: user=%s, default=%s\n", userRules.Version, defaultRules.Version)
		}

		// Compare rule counts
		fmt.Printf("📊 Rules: user=%d, default=%d\n", len(userRules.Rules), len(defaultRules.Rules))

		// Show new rules available in default
		newRules := []string{}
		for ruleName := range defaultRules.Rules {
			if _, exists := userRules.Rules[ruleName]; !exists {
				newRules = append(newRules, ruleName)
			}
		}

		if len(newRules) > 0 {
			fmt.Printf("🆕 New rules available in default config: %v\n", newRules)
		}

		// Check for new settings
		if userRules.Settings.SizeExceededBehaviour == "" && defaultRules.Settings.SizeExceededBehaviour != "" {
			fmt.Printf("🆕 New setting available: size_exceeded_behaviour (default: %s)\n", defaultRules.Settings.SizeExceededBehaviour)
		}
	}

	// Offer to update if requested
	if cmd.Bool("update") {

		fmt.Println("\n🔄 Updating user configuration...")

		// Create backup
		backupPath := configPath + ".backup." + fmt.Sprintf("%d", time.Now().Unix())
		if err := os.WriteFile(backupPath, userConfigData, 0600); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		fmt.Printf("📦 Backup created: %s\n", backupPath)

		// Write the default config (user will need to manually merge customizations)
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0600); err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}

		fmt.Printf("✅ Configuration updated: %s\n", configPath)
		fmt.Println("⚠️  Note: Any custom rules have been backed up. Please review and re-add them manually if needed.")
	} else {
		fmt.Println("\n💡 To update your configuration with new defaults, run:")
		fmt.Printf("   mcp-devtools security-config-diff --update\n")
		fmt.Println("   (This will create a backup of your current config)")
	}

	return nil
}

// handleSecurityConfigValidate validates the security configuration file
func handleSecurityConfigValidate(cmd *cli.Command) error {
	// Get config path
	configPath := cmd.String("config-path")
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = fmt.Sprintf("%s/.mcp-devtools/security.yaml", homeDir)
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("❌ Configuration file not found: %s\n", configPath)
		fmt.Println("💡 The file will be created automatically when the security system is first used.")
		return nil
	}

	fmt.Printf("🔍 Validating security configuration: %s\n", configPath)
	fmt.Println("=" + strings.Repeat("=", len(configPath)+35))

	// Read config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Try to parse and validate the config
	rules, yamlErr := security.ValidateSecurityConfig(configData)
	if yamlErr != nil {
		fmt.Printf("❌ YAML parsing failed: %v\n", yamlErr)

		// Try to provide more detailed error information
		lines := strings.Split(string(configData), "\n")
		fmt.Printf("\n📄 File has %d lines, %d bytes\n", len(lines), len(configData))

		// Check for common YAML issues
		for i, line := range lines {
			lineNum := i + 1
			trimmed := strings.TrimSpace(line)

			// Check for tabs (common YAML issue)
			if strings.Contains(line, "\t") {
				fmt.Printf("⚠️  Line %d contains tabs (use spaces instead): %s\n", lineNum, trimmed)
			}

			// Check for basic syntax issues
			if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "#") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) == "" && !strings.HasSuffix(trimmed, ":") {
					fmt.Printf("⚠️  Line %d may have missing value: %s\n", lineNum, trimmed)
				}
			}
		}

		return fmt.Errorf("configuration file has YAML syntax errors")
	}

	fmt.Println("✅ Configuration is valid")

	// Show configuration summary
	fmt.Println("\n📊 Configuration Summary")
	fmt.Println("========================")
	fmt.Printf("Version: %s\n", rules.Version)
	fmt.Printf("Security enabled: %t\n", rules.Settings.Enabled)
	fmt.Printf("Default action: %s\n", rules.Settings.DefaultAction)
	fmt.Printf("Auto reload: %t\n", rules.Settings.AutoReload)
	fmt.Printf("Max content size: %d KB\n", rules.Settings.MaxContentSize)
	fmt.Printf("Max scan size: %d KB\n", rules.Settings.MaxScanSize)
	fmt.Printf("Size exceeded behaviour: %s\n", rules.Settings.SizeExceededBehaviour)
	fmt.Printf("Rules defined: %d\n", len(rules.Rules))
	fmt.Printf("Trusted domains: %d\n", len(rules.TrustedDomains))
	fmt.Printf("Denied files: %d\n", len(rules.AccessControl.DenyFiles))
	fmt.Printf("Denied domains: %d\n", len(rules.AccessControl.DenyDomains))

	fmt.Println("\n✅ Configuration is valid and ready for use")
	return nil
}
