package types

import (
	"context"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OAuth2Config represents OAuth 2.0/2.1 configuration
type OAuth2Config struct {
	Enabled               bool   `json:"enabled"`
	Issuer                string `json:"issuer"`
	Audience              string `json:"audience"`
	JWKSUrl               string `json:"jwks_url"`
	DynamicRegistration   bool   `json:"dynamic_registration"`
	AuthorizationServer   string `json:"authorization_server,omitempty"`
	RequireHTTPS          bool   `json:"require_https"`
	TokenIntrospectionUrl string `json:"token_introspection_url,omitempty"`
}

// TokenClaims represents the claims in an OAuth 2.1 JWT token
type TokenClaims struct {
	jwt.RegisteredClaims
	Scope       string   `json:"scope,omitempty"`
	ClientID    string   `json:"client_id,omitempty"`
	Username    string   `json:"username,omitempty"`
	Authorities []string `json:"authorities,omitempty"`
}

// TokenValidator interface for token validation
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
	GetJWKS(ctx context.Context) (interface{}, error)
}

// AuthorizationServerMetadata represents OAuth 2.0 Authorization Server Metadata (RFC8414)
type AuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	JWKSUri                           string   `json:"jwks_uri"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
}

// ProtectedResourceMetadata represents OAuth 2.0 Protected Resource Metadata (RFC9728)
type ProtectedResourceMetadata struct {
	Resource                          string   `json:"resource"`
	AuthorizationServers              []string `json:"authorization_servers"`
	JWKSUri                           string   `json:"jwks_uri,omitempty"`
	BearerMethodsSupported            []string `json:"bearer_methods_supported,omitempty"`
	ResourceDocumentation             string   `json:"resource_documentation,omitempty"`
	ResourceSigningAlgValuesSupported []string `json:"resource_signing_alg_values_supported,omitempty"`
}

// DynamicClientRegistrationRequest represents a client registration request (RFC7591)
type DynamicClientRegistrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	LogoURI                 string   `json:"logo_uri,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
	Contacts                []string `json:"contacts,omitempty"`
	TosURI                  string   `json:"tos_uri,omitempty"`
	PolicyURI               string   `json:"policy_uri,omitempty"`
	JWKSUri                 string   `json:"jwks_uri,omitempty"`
	SoftwareID              string   `json:"software_id,omitempty"`
	SoftwareVersion         string   `json:"software_version,omitempty"`
}

// DynamicClientRegistrationResponse represents a client registration response (RFC7591)
type DynamicClientRegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	LogoURI                 string   `json:"logo_uri,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
	Contacts                []string `json:"contacts,omitempty"`
	TosURI                  string   `json:"tos_uri,omitempty"`
	PolicyURI               string   `json:"policy_uri,omitempty"`
	JWKSUri                 string   `json:"jwks_uri,omitempty"`
	SoftwareID              string   `json:"software_id,omitempty"`
	SoftwareVersion         string   `json:"software_version,omitempty"`
}

// PKCEChallenge represents a PKCE code challenge
type PKCEChallenge struct {
	CodeChallenge       string    `json:"code_challenge"`
	CodeChallengeMethod string    `json:"code_challenge_method"`
	CodeVerifier        string    `json:"code_verifier"`
	CreatedAt           time.Time `json:"created_at"`
}

// OAuth2Error represents an OAuth 2.0 error response
type OAuth2Error struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
	State            string `json:"state,omitempty"`
}

func (e OAuth2Error) WriteHTTPResponse(w http.ResponseWriter, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(statusCode)

	// Write JSON error response
	response := `{"error":"` + e.Error + `"`
	if e.ErrorDescription != "" {
		response += `,"error_description":"` + e.ErrorDescription + `"`
	}
	if e.ErrorURI != "" {
		response += `,"error_uri":"` + e.ErrorURI + `"`
	}
	response += `}`

	_, _ = w.Write([]byte(response))
}

// Standard OAuth 2.0 error types
var (
	ErrInvalidRequest       = OAuth2Error{Error: "invalid_request"}
	ErrInvalidClient        = OAuth2Error{Error: "invalid_client"}
	ErrInvalidGrant         = OAuth2Error{Error: "invalid_grant"}
	ErrUnauthorizedClient   = OAuth2Error{Error: "unauthorized_client"}
	ErrUnsupportedGrantType = OAuth2Error{Error: "unsupported_grant_type"}
	ErrInvalidScope         = OAuth2Error{Error: "invalid_scope"}
	ErrInvalidToken         = OAuth2Error{Error: "invalid_token"}
	ErrInsufficientScope    = OAuth2Error{Error: "insufficient_scope"}
)

// AuthenticationResult represents the result of token authentication
type AuthenticationResult struct {
	Authenticated   bool
	Claims          *TokenClaims
	Error           error
	WWWAuthenticate string
}

// ClientRegistrar interface for dynamic client registration
type ClientRegistrar interface {
	RegisterClient(ctx context.Context, req *DynamicClientRegistrationRequest) (*DynamicClientRegistrationResponse, error)
	GetClient(ctx context.Context, clientID string) (*DynamicClientRegistrationResponse, error)
	UpdateClient(ctx context.Context, clientID string, req *DynamicClientRegistrationRequest) (*DynamicClientRegistrationResponse, error)
	DeleteClient(ctx context.Context, clientID string) error
}

// MetadataProvider interface for OAuth metadata endpoints
type MetadataProvider interface {
	GetAuthorizationServerMetadata(ctx context.Context) (*AuthorizationServerMetadata, error)
	GetProtectedResourceMetadata(ctx context.Context) (*ProtectedResourceMetadata, error)
}

// Context key types to avoid collisions
type contextKey string

const (
	// OAuthClaimsKey is the context key for OAuth claims
	OAuthClaimsKey contextKey = "oauth_claims"
	// OAuthAuthFailedKey is the context key for OAuth auth failure
	OAuthAuthFailedKey contextKey = "oauth_auth_failed"
)
