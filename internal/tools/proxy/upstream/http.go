package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPTransport implements the Streamable HTTP transport for MCP.
type HTTPTransport struct {
	config    *Config
	client    *http.Client
	closeOnce sync.Once
}

// NewHTTPTransport creates a new HTTP transport.
func NewHTTPTransport(cfg *Config) *HTTPTransport {
	logrus.WithField("url", cfg.ServerURL).Debug("creating HTTP transport")
	return &HTTPTransport{
		config: cfg,
		client: &http.Client{},
	}
}

// Start initialises the HTTP transport by verifying connectivity.
func (t *HTTPTransport) Start(ctx context.Context) error {
	logrus.WithField("url", t.config.ServerURL).Debug("HTTP transport starting")

	// Make a test request to verify connectivity and auth status
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.config.ServerURL, nil)
	if err != nil {
		logrus.WithError(err).Debug("HTTP failed to create request")
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Add custom headers
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
		logrus.WithField("key", k).Debug("HTTP adding custom header")
	}

	// Add authorisation header if auth provider is available
	if t.config.AuthProvider != nil {
		token, err := t.config.AuthProvider.GetAccessToken(ctx)
		if err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
			logrus.Debug("HTTP added authorisation header")
		}
	}

	logrus.WithField("url", t.config.ServerURL).Debug("HTTP sending connectivity check")
	resp, err := t.client.Do(req)
	if err != nil {
		logrus.WithError(err).Debug("HTTP connectivity check failed")
		return fmt.Errorf("connectivity check failed: %w", err)
	}
	defer resp.Body.Close()

	logrus.WithField("status", resp.StatusCode).Debug("HTTP received response")

	if resp.StatusCode == http.StatusUnauthorized {
		logrus.Debug("HTTP unauthorised response")
		return ErrUnauthorised
	}

	if resp.StatusCode == http.StatusNotFound {
		logrus.Debug("HTTP not found response")
		return ErrNotFound
	}

	if resp.StatusCode == http.StatusMethodNotAllowed {
		logrus.Debug("HTTP method not allowed response")
		return ErrMethodNotAllowed
	}

	// Server is reachable - any other status is fine
	logrus.WithField("url", t.config.ServerURL).Info("HTTP transport ready")
	return nil
}

// SendReceive sends a JSON-RPC message via HTTP POST and returns the response.
func (t *HTTPTransport) SendReceive(ctx context.Context, msg *Message) (*Message, error) {
	t.logToFile(fmt.Sprintf("SendReceive called for ID %v method %s", msg.ID, msg.Method))

	data, err := json.Marshal(msg)
	if err != nil {
		t.logToFile(fmt.Sprintf("Marshal failed: %v", err))
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	t.logToFile(fmt.Sprintf("Creating POST request to %s", t.config.ServerURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.config.ServerURL, bytes.NewReader(data))
	if err != nil {
		t.logToFile(fmt.Sprintf("Create request failed: %v", err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Add custom headers
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
	}

	// Add authorisation header if auth provider is available
	if t.config.AuthProvider != nil {
		token, err := t.config.AuthProvider.GetAccessToken(ctx)
		if err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
			t.logToFile("Added auth header")
		}
	}

	t.logToFile(fmt.Sprintf("Sending POST request (%d bytes)", len(data)))
	resp, err := t.client.Do(req)
	if err != nil {
		t.logToFile(fmt.Sprintf("POST request failed: %v", err))
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	t.logToFile(fmt.Sprintf("Received response status %d", resp.StatusCode))

	if resp.StatusCode == http.StatusUnauthorized {
		t.logToFile("Response: unauthorised")
		return nil, ErrUnauthorised
	}

	if resp.StatusCode == http.StatusNotFound {
		t.logToFile("Response: not found")
		return nil, ErrNotFound
	}

	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.logToFile("Response: method not allowed")
		return nil, ErrMethodNotAllowed
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.logToFile(fmt.Sprintf("Unexpected status %d: %s", resp.StatusCode, string(body)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	t.logToFile("Decoding response JSON")
	// Parse response
	var response Message
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.logToFile(fmt.Sprintf("Decode failed: %v", err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	t.logToFile(fmt.Sprintf("Successfully received response for ID %v", response.ID))
	return &response, nil
}

// logToFile writes a debug message to the proxy execution log file
func (t *HTTPTransport) logToFile(message string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	logDir := filepath.Join(homeDir, ".mcp-devtools")
	logPath := filepath.Join(logDir, "proxy-execution.log")

	// Ensure directory exists
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer logFile.Close()

	fmt.Fprintf(logFile, "[%s] [HTTP] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), message)
}

// Close closes the HTTP transport.
func (t *HTTPTransport) Close() error {
	t.closeOnce.Do(func() {
		logrus.Debug("HTTP transport closed")
	})
	return nil
}
