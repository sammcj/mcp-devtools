package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

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
	logrus.WithFields(logrus.Fields{
		"id":     msg.ID,
		"method": msg.Method,
	}).Debug("HTTP: SendReceive called")

	data, err := json.Marshal(msg)
	if err != nil {
		logrus.WithError(err).Debug("HTTP: marshal failed")
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"url":   t.config.ServerURL,
		"bytes": len(data),
	}).Debug("HTTP: creating POST request")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.config.ServerURL, bytes.NewReader(data))
	if err != nil {
		logrus.WithError(err).Debug("HTTP: create request failed")
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
			logrus.Debug("HTTP: added authorisation header")
		}
	}

	logrus.WithField("bytes", len(data)).Debug("HTTP: sending POST request")
	resp, err := t.client.Do(req)
	if err != nil {
		logrus.WithError(err).Debug("HTTP: POST request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	logrus.WithField("status", resp.StatusCode).Debug("HTTP: received response")

	if resp.StatusCode == http.StatusUnauthorized {
		logrus.Debug("HTTP: unauthorised response")
		return nil, ErrUnauthorised
	}

	if resp.StatusCode == http.StatusNotFound {
		logrus.Debug("HTTP: not found response")
		return nil, ErrNotFound
	}

	if resp.StatusCode == http.StatusMethodNotAllowed {
		logrus.Debug("HTTP: method not allowed response")
		return nil, ErrMethodNotAllowed
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		logrus.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(body),
		}).Debug("HTTP: unexpected status")
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	logrus.Debug("HTTP: decoding response JSON")
	// Parse response
	var response Message
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logrus.WithError(err).Debug("HTTP: decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logrus.WithField("id", response.ID).Debug("HTTP: successfully received response")
	return &response, nil
}

// Close closes the HTTP transport.
func (t *HTTPTransport) Close() error {
	t.closeOnce.Do(func() {
		logrus.Debug("HTTP transport closed")
	})
	return nil
}
