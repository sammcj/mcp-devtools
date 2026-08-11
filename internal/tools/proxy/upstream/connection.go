package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/auth"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy/types"
	"github.com/sirupsen/logrus"
)

// Connection represents a connection to a single upstream MCP server.
type Connection struct {
	config       *types.UpstreamConfig
	cacheDir     string
	transport    Transport
	authProvider *auth.Provider
	tools        []ToolInfo
	toolsMu      sync.RWMutex
	connected    bool
	connMu       sync.RWMutex
}

// ToolInfo holds information about a tool from an upstream server.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// NewConnection creates a new upstream connection.
func NewConnection(config *types.UpstreamConfig, cacheDir string, callbackPort int) (*Connection, error) {
	// Create OAuth provider
	serverHash := types.UpstreamHash(config)

	var staticClientInfo *auth.ClientInfo
	if config.OAuth != nil && config.OAuth.ClientID != "" {
		staticClientInfo = &auth.ClientInfo{
			ClientID:     config.OAuth.ClientID,
			ClientSecret: config.OAuth.ClientSecret,
		}
	}

	authProvider := auth.NewProvider(&auth.ProviderConfig{
		ServerURL:        config.URL,
		ServerHash:       serverHash,
		CallbackPort:     callbackPort,
		CallbackHost:     "localhost",
		ClientName:       "MCP DevTools Proxy",
		CacheDir:         cacheDir,
		StaticClientInfo: staticClientInfo,
	})

	return &Connection{
		config:       config,
		cacheDir:     cacheDir,
		authProvider: authProvider,
	}, nil
}

// Connect establishes the connection to the upstream server.
func (c *Connection) Connect(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.connected {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"name": c.config.Name,
		"url":  c.config.URL,
	}).Info("connecting to upstream server")

	// Initialise OAuth provider
	if err := c.authProvider.Initialise(ctx); err != nil {
		logrus.WithError(err).WithField("name", c.config.Name).Warn("OAuth initialisation failed (may require auth)")
		// Don't fail here - auth might be handled during transport Start
	}

	// Streamable HTTP is the only upstream transport. HTTP+SSE was deprecated
	// in MCP 2026-07-28 and removed here in v2.
	if strategy := ParseStrategy(c.config.Transport); strategy != StrategyHTTP {
		logrus.WithFields(logrus.Fields{
			"name":      c.config.Name,
			"requested": c.config.Transport,
		}).Warn("upstream SSE transport was removed; using Streamable HTTP")
	}

	transportConfig := &Config{
		ServerURL:    c.config.URL,
		Headers:      c.config.Headers,
		AuthProvider: c.authProvider,
	}

	transport := NewHTTPTransport(transportConfig)

	logrus.WithField("name", c.config.Name).Debug("attempting connection")

	err := transport.Start(ctx)
	if err == nil {
		c.transport = transport
		c.connected = true
		logrus.WithField("name", c.config.Name).Info("connected to upstream server")
		return nil
	}

	logrus.WithError(err).WithField("name", c.config.Name).Debug("connection failed")

	// Handle authentication
	if err == ErrUnauthorised {
		logrus.WithField("name", c.config.Name).Info("authentication required")

		if err := c.authenticateAndConnect(ctx, transportConfig); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		c.connected = true
		logrus.WithField("name", c.config.Name).Info("connected after authentication")
		return nil
	}

	return fmt.Errorf("failed to connect: %w", err)
}

// authenticateAndConnect performs OAuth authentication and connects.
func (c *Connection) authenticateAndConnect(ctx context.Context, transportConfig *Config) error {
	// Start callback server
	callbackServer, err := auth.NewCallbackServer(c.authProvider.Port())
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer callbackServer.Close()
	callbackServer.Start()

	// Get authorisation URL
	authURL, err := c.authProvider.GetAuthorizationURL("")
	if err != nil {
		return fmt.Errorf("failed to get authorisation URL: %w", err)
	}

	// Tell the callback server what a valid response looks like before any
	// browser redirect can reach it.
	callbackServer.Expect(c.authProvider.AuthorizationExpectations())

	logrus.WithFields(logrus.Fields{
		"name": c.config.Name,
		"url":  authURL,
	}).Info("opening browser for authorisation")

	// Open browser for OAuth flow
	if err := openBrowser(authURL); err != nil {
		logrus.WithError(err).Warn("failed to open browser automatically")
		logrus.WithField("url", authURL).Warn("Please open this URL in your browser to authorise")
	}

	// Wait for callback
	code, err := callbackServer.WaitForCode(ctx, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to receive authorisation code: %w", err)
	}

	// Exchange code for tokens
	if err := c.authProvider.ExchangeCode(ctx, code); err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	logrus.WithField("name", c.config.Name).Info("authorisation successful")

	// Retry connection with new token
	transport := NewHTTPTransport(transportConfig)

	if err := transport.Start(ctx); err != nil {
		return fmt.Errorf("failed to connect after auth: %w", err)
	}

	c.transport = transport
	return nil
}

// Port returns the OAuth callback port (needed for auth provider access).
func (c *Connection) Port() int {
	return c.authProvider.Port()
}

// FetchTools fetches the list of tools from the upstream server.
func (c *Connection) FetchTools(ctx context.Context) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	logrus.WithField("name", c.config.Name).Debug("fetching tools from upstream")

	req, err := newRequest("fetch-tools", "tools/list", "", nil)
	if err != nil {
		return err
	}

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send request and wait for response
	msg, err := c.transport.SendReceive(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send/receive tools/list: %w", err)
	}

	if msg.Error != nil {
		return fmt.Errorf("tools/list error: %s", msg.Error.Message)
	}

	var result struct {
		Tools []ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		return fmt.Errorf("failed to parse tools/list response: %w", err)
	}

	c.toolsMu.Lock()
	c.tools = result.Tools
	c.toolsMu.Unlock()

	logrus.WithFields(logrus.Fields{
		"name":  c.config.Name,
		"count": len(result.Tools),
	}).Info("fetched tools from upstream")

	return nil
}

// GetTools returns the list of tools from this upstream.
func (c *Connection) GetTools() []ToolInfo {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()
	return c.tools
}

// ExecuteTool executes a tool on the upstream server.
func (c *Connection) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (*Message, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	logrus.WithFields(logrus.Fields{
		"name": c.config.Name,
		"tool": toolName,
	}).Debug("executing tool on upstream")

	req, err := newRequest(
		fmt.Sprintf("tool-call-%d", time.Now().UnixNano()),
		"tools/call",
		toolName,
		map[string]any{"name": toolName, "arguments": args},
	)
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"name": c.config.Name,
		"tool": toolName,
		"id":   req.ID,
	}).Debug("Proxy: executing tool with request ID")

	// Add timeout to context (60 seconds for tool execution)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	logrus.WithFields(logrus.Fields{
		"name": c.config.Name,
		"tool": toolName,
	}).Debug("Proxy: calling SendReceive")

	msg, err := c.transport.SendReceive(ctx, req)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"name": c.config.Name,
			"tool": toolName,
		}).Debug("Proxy: SendReceive failed")
		return nil, fmt.Errorf("failed to execute tool on upstream: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"name": c.config.Name,
		"tool": toolName,
		"id":   msg.ID,
	}).Debug("Proxy: received response")
	return msg, nil
}

// Close closes the connection.
func (c *Connection) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.transport != nil {
		if err := c.transport.Close(); err != nil {
			return err
		}
	}

	c.connected = false
	logrus.WithField("name", c.config.Name).Info("closed connection to upstream")
	return nil
}

// openBrowser opens a URL in the default system browser
func openBrowser(url string) error {
	var cmdName string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmdName = "open"
		args = []string{url}
	case "linux":
		cmdName = "xdg-open"
		args = []string{url}
	case "windows":
		// Windows requires cmd /c start with an empty title parameter
		cmdName = "cmd"
		args = []string{"/c", "start", "", url}
	default:
		return fmt.Errorf("unsupported operating system for browser opening: %s", runtime.GOOS)
	}

	// Create command and redirect stdout/stderr to prevent stdio pollution
	// This is critical when running in stdio mode - any output would corrupt the MCP protocol
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}
