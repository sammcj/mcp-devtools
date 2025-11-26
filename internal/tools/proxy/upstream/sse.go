package upstream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// SSETransport implements the Server-Sent Events transport for MCP.
// MCP SSE protocol flow:
// 1. Client opens GET request to establish SSE stream
// 2. Server sends "endpoint" event with URL for POSTing messages
// 3. Client sends messages via POST to the endpoint URL
// 4. Server sends responses via the SSE stream
type SSETransport struct {
	config    *Config
	client    *http.Client
	done      chan struct{}
	closeOnce sync.Once
	resp      *http.Response
	mu        sync.Mutex

	// endpoint is the URL to POST messages to (received from server)
	endpoint   *url.URL
	endpointMu sync.RWMutex

	// endpointReady signals when the endpoint has been received
	endpointReady chan struct{}

	// pending tracks requests waiting for responses (keyed by request ID)
	pending   map[any]chan *Message
	pendingMu sync.RWMutex

	// connCtx is the long-lived context for the SSE connection
	// This is separate from individual request contexts to keep the connection alive
	connCtx    context.Context
	connCancel context.CancelFunc
}

// NewSSETransport creates a new SSE transport.
func NewSSETransport(cfg *Config) *SSETransport {
	// Create long-lived context for SSE connection (separate from request contexts)
	connCtx, connCancel := context.WithCancel(context.Background())
	return &SSETransport{
		config:        cfg,
		client:        &http.Client{},
		done:          make(chan struct{}),
		endpointReady: make(chan struct{}),
		pending:       make(map[any]chan *Message),
		connCtx:       connCtx,
		connCancel:    connCancel,
	}
}

// Start establishes the SSE connection and waits for the endpoint event.
// The ctx parameter is used for initial setup timeout only.
// The SSE connection itself uses a long-lived context to stay alive across requests.
func (t *SSETransport) Start(ctx context.Context) error {
	// Use long-lived connection context for SSE GET request
	// This ensures the connection stays alive even when individual request contexts are cancelled
	req, err := http.NewRequestWithContext(t.connCtx, http.MethodGet, t.config.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Add custom headers
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
	}

	// Add authorisation header if auth provider is available
	if t.config.AuthProvider != nil {
		token, err := t.config.AuthProvider.GetAccessToken(ctx)
		if err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return ErrUnauthorised
	}

	if resp.StatusCode == http.StatusMethodNotAllowed {
		resp.Body.Close()
		return ErrMethodNotAllowed
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	t.mu.Lock()
	t.resp = resp
	t.mu.Unlock()

	// Start reading SSE events
	go t.readEvents(resp.Body)

	// Wait for endpoint event before returning
	select {
	case <-t.endpointReady:
		logrus.WithField("endpoint", t.endpoint.String()).Info("SSE transport ready")
		return nil
	case <-ctx.Done():
		t.Close()
		return ctx.Err()
	case <-t.done:
		return ErrClosed
	}
}

// readEvents reads SSE events from the response body.
func (t *SSETransport) readEvents(body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var event strings.Builder

	for scanner.Scan() {
		select {
		case <-t.done:
			return
		default:
		}

		line := scanner.Text()

		if line == "" {
			// Empty line indicates end of event
			if event.Len() > 0 {
				t.processEvent(event.String())
				event.Reset()
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			// Comment line, ignore
			continue
		}

		event.WriteString(line)
		event.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		logrus.WithError(err).Error("SSE scanner error")
	}
}

// processEvent processes a single SSE event.
func (t *SSETransport) processEvent(eventData string) {
	var eventType string
	var data string

	for line := range strings.SplitSeq(eventData, "\n") {
		if after, ok := strings.CutPrefix(line, "event:"); ok {
			eventType = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "data:"); ok {
			data = strings.TrimSpace(after)
		}
	}

	if data == "" {
		return
	}

	// Handle "endpoint" event - this contains the URL to POST messages to
	if eventType == "endpoint" {
		t.setEndpoint(data)
		return
	}

	// Check if this looks like an endpoint URL (starts with / or http)
	if eventType == "" && (strings.HasPrefix(data, "/") || strings.HasPrefix(data, "http")) && !strings.HasPrefix(data, "{") {
		t.setEndpoint(data)
		return
	}

	// Default: treat as JSON-RPC message
	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		// Ignore non-JSON messages
		return
	}

	// If this is a response (has ID), deliver it to the waiting request
	if msg.ID != nil {
		t.pendingMu.RLock()
		responseChan, exists := t.pending[msg.ID]
		t.pendingMu.RUnlock()

		if exists {
			select {
			case responseChan <- &msg:
				// Response delivered
			case <-t.done:
				// Transport closed
			}
		}
	}
}

// setEndpoint parses and stores the endpoint URL from the server.
func (t *SSETransport) setEndpoint(data string) {
	serverURL, err := url.Parse(t.config.ServerURL)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse server URL")
		return
	}

	// Parse the endpoint URL (may be relative or absolute)
	endpointURL, err := url.Parse(data)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse endpoint URL")
		return
	}

	// Resolve relative URLs against the server URL
	resolved := serverURL.ResolveReference(endpointURL)

	// Verify the endpoint has the same origin as the server
	if resolved.Scheme != serverURL.Scheme || resolved.Host != serverURL.Host {
		logrus.WithFields(logrus.Fields{
			"server_origin":   serverURL.Scheme + "://" + serverURL.Host,
			"endpoint_origin": resolved.Scheme + "://" + resolved.Host,
		}).Error("Endpoint origin mismatch (security check failed)")
		return
	}

	t.endpointMu.Lock()
	t.endpoint = resolved
	t.endpointMu.Unlock()

	// Signal that endpoint is ready (only once)
	select {
	case <-t.endpointReady:
		// Already signalled
	default:
		close(t.endpointReady)
	}
}

// SendReceive sends a JSON-RPC message via HTTP POST and waits for the response via SSE.
func (t *SSETransport) SendReceive(ctx context.Context, msg *Message) (*Message, error) {
	// Get the endpoint URL (set by the server via "endpoint" event)
	t.endpointMu.RLock()
	endpoint := t.endpoint
	t.endpointMu.RUnlock()

	if endpoint == nil {
		return nil, fmt.Errorf("not connected: endpoint not received from server")
	}

	// Create response channel for this request
	responseChan := make(chan *Message, 1)
	t.pendingMu.Lock()
	t.pending[msg.ID] = responseChan
	t.pendingMu.Unlock()

	// Ensure cleanup on return
	defer func() {
		t.pendingMu.Lock()
		delete(t.pending, msg.ID)
		t.pendingMu.Unlock()
		close(responseChan)
	}()

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for k, v := range t.config.Headers {
		req.Header.Set(k, v)
	}

	// Add authorisation header if auth provider is available
	if t.config.AuthProvider != nil {
		token, err := t.config.AuthProvider.GetAccessToken(ctx)
		if err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorised
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Streamable HTTP transport: Try to read response from POST body first
	// Some servers return the response immediately in the body (even with 202)
	// Others return empty body and send response via SSE stream
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// If there's a body, try to parse it as JSON-RPC
	if len(bodyBytes) > 0 {
		var response Message
		if err := json.Unmarshal(bodyBytes, &response); err == nil {
			return &response, nil
		}
	}

	// Empty or invalid body - response will come via SSE stream
	// Wait for response via SSE stream
	select {
	case response := <-responseChan:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.done:
		return nil, ErrClosed
	}
}

// Close closes the SSE transport.
func (t *SSETransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.done)
		// Cancel the connection context to stop the SSE reader
		t.connCancel()
		t.mu.Lock()
		if t.resp != nil {
			t.resp.Body.Close()
		}
		t.mu.Unlock()
	})
	return nil
}
