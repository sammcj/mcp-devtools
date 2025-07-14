package client

import (
	"context"
	"time"

	"github.com/sammcj/mcp-devtools/internal/oauth/types"
)

// OAuth2ClientConfig represents OAuth 2.0 client configuration for browser authentication
type OAuth2ClientConfig struct {
	// Client credentials
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"` // Optional for public clients

	// Authorization server endpoints
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`

	// Optional discovery
	IssuerURL string `json:"issuer_url,omitempty"` // For .well-known/oauth-authorization-server discovery

	// Redirect configuration
	RedirectURI string `json:"redirect_uri"` // Usually http://localhost:PORT/callback

	// OAuth parameters
	Scope    string `json:"scope,omitempty"`    // Requested scopes
	Resource string `json:"resource,omitempty"` // RFC8707 resource parameter

	// Security settings
	RequireHTTPS bool `json:"require_https"` // Default true, false only for localhost

	// Timeouts
	AuthTimeout time.Duration `json:"auth_timeout"` // How long to wait for user authentication
	ServerPort  int           `json:"server_port"`  // Port for callback server (0 = random)
}

// TokenResponse represents an OAuth 2.0 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// AuthenticationSession represents an ongoing OAuth authentication session
type AuthenticationSession struct {
	// PKCE parameters
	PKCEChallenge *types.PKCEChallenge

	// Session state
	State       string
	RedirectURI string

	// Authorization URL
	AuthURL string

	// Callback server
	CallbackServer CallbackServer

	// Result channels
	ResultCh chan *AuthenticationResult
	ErrorCh  chan error

	// Context for cancellation
	Context context.Context
	Cancel  context.CancelFunc
}

// AuthenticationResult represents the result of browser-based authentication
type AuthenticationResult struct {
	Success       bool
	TokenResponse *TokenResponse
	Error         error

	// Additional metadata
	State       string
	ExchangedAt time.Time
}

// CallbackServer interface for handling OAuth callbacks
type CallbackServer interface {
	Start(ctx context.Context, port int) error
	Stop() error
	GetRedirectURI() string
	GetAuthorizationCode() <-chan string
	GetError() <-chan error
}

// OAuth2Client interface for browser-based OAuth flows
type OAuth2Client interface {
	// StartAuthentication initiates the OAuth 2.0 authorization code flow
	StartAuthentication(ctx context.Context) (*AuthenticationSession, error)

	// ExchangeCodeForToken exchanges authorization code for access token
	ExchangeCodeForToken(ctx context.Context, code string, pkce *types.PKCEChallenge) (*TokenResponse, error)

	// DiscoverEndpoints discovers OAuth endpoints from issuer URL
	DiscoverEndpoints(ctx context.Context) error

	// ValidateConfiguration validates the client configuration
	ValidateConfiguration() error
}

// BrowserLauncher interface for opening browsers
type BrowserLauncher interface {
	OpenURL(url string) error
}
