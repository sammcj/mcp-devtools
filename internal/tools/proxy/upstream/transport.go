package upstream

import (
	"context"
	"errors"
)

// Transport defines the interface for MCP transports.
type Transport interface {
	// Start initialises the transport connection.
	Start(ctx context.Context) error

	// SendReceive sends a JSON-RPC message and waits for the response.
	// This is a synchronous request/response operation.
	SendReceive(ctx context.Context, msg *Message) (*Message, error)

	// Close closes the transport connection.
	Close() error
}

// Strategy names an upstream transport. Streamable HTTP is the only one left;
// the sse values are kept so an existing config still parses, and the caller
// warns before falling back to HTTP.
type Strategy string

// Transport strategy constants.
const (
	StrategyHTTP Strategy = "http"
	StrategySSE  Strategy = "sse"
)

// ParseStrategy parses a strategy string.
func ParseStrategy(s string) Strategy {
	switch s {
	case "sse", "sse-first", "sse-only":
		return StrategySSE
	default:
		return StrategyHTTP
	}
}

// AuthProvider provides OAuth tokens for authenticated requests.
type AuthProvider interface {
	// GetAccessToken returns the current access token.
	GetAccessToken(ctx context.Context) (string, error)

	// RefreshToken refreshes the access token.
	RefreshToken(ctx context.Context) error
}

// Config holds transport configuration.
type Config struct {
	ServerURL    string
	Headers      map[string]string
	AuthProvider AuthProvider
}

// Transport errors.
var (
	ErrUnauthorised     = errors.New("unauthorised")
	ErrNotFound         = errors.New("not found (404)")
	ErrMethodNotAllowed = errors.New("method not allowed (405)")
)
