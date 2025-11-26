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

// Strategy defines the transport selection strategy.
type Strategy string

// Transport strategy constants.
const (
	StrategyHTTPFirst Strategy = "http-first"
	StrategySSEFirst  Strategy = "sse-first"
	StrategyHTTPOnly  Strategy = "http-only"
	StrategySSEOnly   Strategy = "sse-only"
)

// ParseStrategy parses a strategy string.
func ParseStrategy(s string) Strategy {
	switch s {
	case "http-first":
		return StrategyHTTPFirst
	case "sse-first":
		return StrategySSEFirst
	case "http-only":
		return StrategyHTTPOnly
	case "sse-only":
		return StrategySSEOnly
	default:
		return StrategyHTTPFirst
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
	Strategy     Strategy
}

// Transport errors.
var (
	ErrUnauthorised     = errors.New("unauthorised")
	ErrNotFound         = errors.New("not found (404)")
	ErrMethodNotAllowed = errors.New("method not allowed (405)")
	ErrClosed           = errors.New("transport closed")
)
