package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sammcj/mcp-devtools/internal/telemetry"
	"github.com/sirupsen/logrus"
)

// HTTPTransport implements the Streamable HTTP transport for MCP.
type HTTPTransport struct {
	config    *Config
	client    *http.Client
	closeOnce sync.Once

	// legacy records that this upstream rejected the 2026-07-28 framing, so
	// later requests skip straight to the older form rather than paying for a
	// failed attempt every call.
	legacy     atomic.Bool
	legacyOnce sync.Once
}

// NewHTTPTransport creates a new HTTP transport.
func NewHTTPTransport(cfg *Config) *HTTPTransport {
	logrus.WithField("url", cfg.ServerURL).Debug("creating HTTP transport")

	// Create HTTP client with OTEL instrumentation
	client := &http.Client{}
	telemetry.WrapHTTPClient(client)

	return &HTTPTransport{
		config: cfg,
		client: client,
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
//
// The 2026-07-28 framing (version header plus matching _meta, and the routing
// headers) is not understood by every upstream: servers on older revisions
// validate MCP-Protocol-Version against an allowlist and answer 400. When that
// happens the request is retried once without any of it, and the connection
// stays in that mode for its lifetime.
func (t *HTTPTransport) SendReceive(ctx context.Context, msg *Message) (*Message, error) {
	logrus.WithFields(logrus.Fields{
		"id":     msg.ID,
		"method": msg.Method,
	}).Debug("HTTP: SendReceive called")

	stateless := !t.legacy.Load()
	response, err := t.send(ctx, msg, stateless)
	if err != nil && stateless && isProtocolFramingRejection(err) {
		t.legacy.Store(true)
		t.legacyOnce.Do(func() {
			logrus.WithFields(logrus.Fields{
				"url":   t.config.ServerURL,
				"error": err.Error(),
			}).Warn("upstream rejected the 2026-07-28 request framing, falling back to the older form for this connection")
		})
		return t.send(ctx, msg, false)
	}
	return response, err
}

// isProtocolFramingRejection reports whether an upstream error looks like a
// complaint about the 2026-07-28 framing rather than about the request itself.
//
// A 400 body is unstructured, so this matches on wording. An empty body counts
// too: a marker cannot appear in nothing, and a gateway that strips error
// bodies would otherwise leave the upstream permanently broken. The retry costs
// one round trip and a genuine 400 simply reproduces in legacy mode.
func isProtocolFramingRejection(err error) bool {
	msg := strings.ToLower(err.Error())
	body, found := strings.CutPrefix(msg, "unexpected status 400: ")
	if !found {
		return false
	}
	if strings.TrimSpace(body) == "" {
		return true
	}
	for _, marker := range []string{"protocol version", "protocol-version", "_meta", "mcp-method", "mcp-name"} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	logrus.WithFields(logrus.Fields{
		"error": err.Error(),
	}).Warn("upstream returned 400 but did not name the request framing, so the 2026-07-28 form was kept")
	return false
}

func (t *HTTPTransport) send(ctx context.Context, msg *Message, stateless bool) (*Message, error) {
	data, err := msg.encode(stateless)
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

	// Add custom headers first so the routing headers below cannot be
	// overridden. A 2026-07-28 server validates Mcp-Method and Mcp-Name against
	// the body and answers -32020 CodeHeaderMismatch if they disagree.
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
	}

	// A header the user set in config is theirs to keep: legacy mode is meant
	// to reproduce the pre-2026-07-28 wire form, which passed custom headers
	// through untouched.
	deleteUnlessConfigured := func(key string) {
		if _, configured := t.config.Headers[key]; !configured {
			req.Header.Del(key)
		}
	}

	if stateless {
		req.Header.Set(headerProtocolVersion, ProtocolVersion)
		req.Header.Set(headerMethod, msg.Method)
		if msg.Name != "" {
			req.Header.Set(headerName, msg.Name)
		} else {
			deleteUnlessConfigured(headerName)
		}
	} else {
		deleteUnlessConfigured(headerProtocolVersion)
		deleteUnlessConfigured(headerMethod)
		deleteUnlessConfigured(headerName)
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
