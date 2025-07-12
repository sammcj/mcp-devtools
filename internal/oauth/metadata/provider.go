package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sirupsen/logrus"
)

// Provider implements OAuth 2.0 metadata endpoints
type Provider struct {
	config  *types.OAuth2Config
	logger  *logrus.Logger
	baseURL string
}

// NewProvider creates a new metadata provider
func NewProvider(config *types.OAuth2Config, baseURL string, logger *logrus.Logger) *Provider {
	return &Provider{
		config:  config,
		logger:  logger,
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// GetAuthorizationServerMetadata returns OAuth 2.0 Authorization Server Metadata (RFC8414)
func (p *Provider) GetAuthorizationServerMetadata(ctx context.Context) (*types.AuthorizationServerMetadata, error) {
	metadata := &types.AuthorizationServerMetadata{
		Issuer:                            p.config.Issuer,
		AuthorizationEndpoint:             p.baseURL + "/oauth/authorize",
		TokenEndpoint:                     p.baseURL + "/oauth/token",
		JWKSUri:                           p.baseURL + "/.well-known/jwks.json",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post", "none"},
		CodeChallengeMethodsSupported:     []string{"S256", "plain"},
		ScopesSupported:                   []string{"openid", "profile", "email"},
	}

	// Add optional endpoints if enabled
	if p.config.DynamicRegistration {
		metadata.RegistrationEndpoint = p.baseURL + "/oauth/register"
	}

	if p.config.TokenIntrospectionUrl != "" {
		metadata.IntrospectionEndpoint = p.config.TokenIntrospectionUrl
	}

	return metadata, nil
}

// GetProtectedResourceMetadata returns OAuth 2.0 Protected Resource Metadata (RFC9728)
func (p *Provider) GetProtectedResourceMetadata(ctx context.Context) (*types.ProtectedResourceMetadata, error) {
	// Determine the resource identifier (canonical URI)
	resourceURI := p.baseURL
	if p.config.Audience != "" {
		resourceURI = p.config.Audience
	}

	// Determine authorization servers
	authServers := []string{}
	if p.config.Issuer != "" {
		authServers = append(authServers, p.config.Issuer)
	}
	if p.config.AuthorizationServer != "" && p.config.AuthorizationServer != p.config.Issuer {
		authServers = append(authServers, p.config.AuthorizationServer)
	}

	metadata := &types.ProtectedResourceMetadata{
		Resource:                          resourceURI,
		AuthorizationServers:              authServers,
		BearerMethodsSupported:            []string{"header"},
		ResourceDocumentation:             p.baseURL + "/docs",
		ResourceSigningAlgValuesSupported: []string{"RS256", "RS384", "RS512"},
	}

	// Add JWKS URI if available
	if p.config.JWKSUrl != "" {
		metadata.JWKSUri = p.config.JWKSUrl
	} else if p.config.Issuer != "" {
		// Try to construct JWKS URL from issuer
		if jwksURL, err := p.constructJWKSURL(p.config.Issuer); err == nil {
			metadata.JWKSUri = jwksURL
		}
	}

	return metadata, nil
}

// constructJWKSURL attempts to construct a JWKS URL from an issuer URL
func (p *Provider) constructJWKSURL(issuer string) (string, error) {
	issuerURL, err := url.Parse(issuer)
	if err != nil {
		return "", fmt.Errorf("invalid issuer URL: %w", err)
	}

	// Standard JWKS path
	issuerURL.Path = strings.TrimSuffix(issuerURL.Path, "/") + "/.well-known/jwks.json"
	return issuerURL.String(), nil
}

// ServeAuthorizationServerMetadata serves the authorization server metadata endpoint
func (p *Provider) ServeAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metadata, err := p.GetAuthorizationServerMetadata(r.Context())
	if err != nil {
		p.logger.WithError(err).Error("Failed to get authorization server metadata")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		p.logger.WithError(err).Error("Failed to encode authorization server metadata")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	p.logger.Debug("Served authorization server metadata")
}

// ServeProtectedResourceMetadata serves the protected resource metadata endpoint
func (p *Provider) ServeProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metadata, err := p.GetProtectedResourceMetadata(r.Context())
	if err != nil {
		p.logger.WithError(err).Error("Failed to get protected resource metadata")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		p.logger.WithError(err).Error("Failed to encode protected resource metadata")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	p.logger.Debug("Served protected resource metadata")
}

// RegisterHandlers registers the metadata endpoints with an HTTP mux
func (p *Provider) RegisterHandlers(mux *http.ServeMux) {
	// Authorization Server Metadata (RFC8414)
	mux.HandleFunc("/.well-known/oauth-authorization-server", p.ServeAuthorizationServerMetadata)

	// Protected Resource Metadata (RFC9728)
	mux.HandleFunc("/.well-known/oauth-protected-resource", p.ServeProtectedResourceMetadata)
}
