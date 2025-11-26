package upstream

import (
	"context"
	"encoding/json"
	"fmt"
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

	// Create transport based on strategy
	strategy := ParseStrategy(c.config.Transport)
	if strategy == "" {
		strategy = StrategyHTTPFirst
	}

	transportConfig := &Config{
		ServerURL:    c.config.URL,
		Headers:      c.config.Headers,
		AuthProvider: c.authProvider,
		Strategy:     strategy,
	}

	// Try to connect with configured strategy
	var transport Transport
	var err error

	useSSE := strategy == StrategySSEOnly || strategy == StrategySSEFirst

	if useSSE {
		transport = NewSSETransport(transportConfig)
	} else {
		transport = NewHTTPTransport(transportConfig)
	}

	logrus.WithFields(logrus.Fields{
		"name":      c.config.Name,
		"transport": transportName(useSSE),
	}).Debug("attempting connection")

	err = transport.Start(ctx)
	if err == nil {
		c.transport = transport
		c.connected = true
		logrus.WithFields(logrus.Fields{
			"name":      c.config.Name,
			"transport": transportName(useSSE),
		}).Info("connected to upstream server")
		return nil
	}

	logrus.WithError(err).WithFields(logrus.Fields{
		"name":      c.config.Name,
		"transport": transportName(useSSE),
	}).Debug("connection failed")

	// Handle transport fallback
	if (strategy == StrategyHTTPFirst && err == ErrNotFound) ||
		(strategy == StrategySSEFirst && err == ErrMethodNotAllowed) {
		// Switch transport type
		useSSE = !useSSE
		logrus.WithFields(logrus.Fields{
			"name":      c.config.Name,
			"transport": transportName(useSSE),
		}).Info("falling back to alternate transport")

		if useSSE {
			transport = NewSSETransport(transportConfig)
		} else {
			transport = NewHTTPTransport(transportConfig)
		}

		err = transport.Start(ctx)
		if err == nil {
			c.transport = transport
			c.connected = true
			logrus.WithFields(logrus.Fields{
				"name":      c.config.Name,
				"transport": transportName(useSSE),
			}).Info("connected to upstream server after fallback")
			return nil
		}
	}

	// Handle authentication
	if err == ErrUnauthorised {
		logrus.WithField("name", c.config.Name).Info("authentication required")

		if err := c.authenticateAndConnect(ctx, useSSE, transportConfig); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		c.connected = true
		logrus.WithField("name", c.config.Name).Info("connected after authentication")
		return nil
	}

	return fmt.Errorf("failed to connect: %w", err)
}

// authenticateAndConnect performs OAuth authentication and connects.
func (c *Connection) authenticateAndConnect(ctx context.Context, useSSE bool, transportConfig *Config) error {
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
	var transport Transport
	if useSSE {
		transport = NewSSETransport(transportConfig)
	} else {
		transport = NewHTTPTransport(transportConfig)
	}

	if err := transport.Start(ctx); err != nil {
		return fmt.Errorf("failed to connect after auth: %w", err)
	}

	c.transport = transport
	return nil
}

// Port returns the OAuth callback port (needed for auth provider access).
func (c *Connection) Port() int {
	// This is a helper method to access the callback port through the auth provider
	// In practice, we'd track this separately, but for simplicity using a default
	return 3334
}

// FetchTools fetches the list of tools from the upstream server.
func (c *Connection) FetchTools(ctx context.Context) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	logrus.WithField("name", c.config.Name).Debug("fetching tools from upstream")

	// Create tools/list request
	req := &Message{
		JSONRPC: "2.0",
		ID:      "fetch-tools",
		Method:  "tools/list",
		Params:  json.RawMessage("{}"),
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

	// Create tools/call request
	params := map[string]any{
		"name":      toolName,
		"arguments": args,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := &Message{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("tool-call-%d", time.Now().UnixNano()),
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	logrus.WithFields(logrus.Fields{
		"name": c.config.Name,
		"tool": toolName,
		"id":   req.ID,
	}).Debug("Proxy: executing tool with request ID")

	// Add timeout to context (60 seconds for tool execution)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Send request and wait for response
	transportType := "unknown"
	if _, ok := c.transport.(*HTTPTransport); ok {
		transportType = "HTTP"
	} else if _, ok := c.transport.(*SSETransport); ok {
		transportType = "SSE"
	}

	logrus.WithFields(logrus.Fields{
		"name":      c.config.Name,
		"tool":      toolName,
		"transport": transportType,
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

func transportName(useSSE bool) string {
	if useSSE {
		return "SSE"
	}
	return "HTTP"
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
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}
