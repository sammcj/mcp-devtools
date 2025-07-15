package toolhelper

import (
	"context"
	"fmt"
	"net/http"

	oauthclient "github.com/sammcj/mcp-devtools/internal/oauth/client"
	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sirupsen/logrus"
)

// OAuthHelper provides OAuth functionality for MCP tools
type OAuthHelper struct {
	logger *logrus.Logger
}

// NewOAuthHelper creates a new OAuth helper for tools
func NewOAuthHelper(logger *logrus.Logger) *OAuthHelper {
	return &OAuthHelper{
		logger: logger,
	}
}

// GetUserClaims extracts OAuth claims from the current request context
// This is for Scenario 2: Tool uses user's OAuth identity
func (h *OAuthHelper) GetUserClaims(ctx context.Context) (*types.TokenClaims, error) {
	claims, ok := ctx.Value(types.OAuthClaimsKey).(*types.TokenClaims)
	if !ok {
		return nil, fmt.Errorf("no OAuth claims found in context - user may not be authenticated")
	}
	return claims, nil
}

// GetUserToken extracts the user's access token from the current request context
// This is for Scenario 2: Tool needs to make API calls as the authenticated user
func (h *OAuthHelper) GetUserToken(ctx context.Context) (string, error) {
	_, err := h.GetUserClaims(ctx)
	if err != nil {
		return "", err
	}

	// Note: In the current implementation, the raw token isn't stored in claims
	// This would need to be enhanced to store the original access token
	// For now, we return an error indicating this feature needs implementation
	return "", fmt.Errorf("user token extraction not yet implemented - this is a future enhancement")
}

// HasScope checks if the current user has a specific OAuth scope
// This is for Scenario 2: Tool-level authorisation based on user permissions
func (h *OAuthHelper) HasScope(ctx context.Context, requiredScope string) bool {
	claims, err := h.GetUserClaims(ctx)
	if err != nil {
		h.logger.WithError(err).Debug("Failed to get user claims for scope check")
		return false
	}

	// Parse scopes (space-separated)
	scopes := parseScopes(claims.Scope)
	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}

	h.logger.WithFields(logrus.Fields{
		"required_scope": requiredScope,
		"user_scopes":    claims.Scope,
	}).Debug("User does not have required scope")

	return false
}

// RequireScope returns an error if the user doesn't have the required scope
// This is for Scenario 2: Tool-level authorisation
func (h *OAuthHelper) RequireScope(ctx context.Context, requiredScope string) error {
	if !h.HasScope(ctx, requiredScope) {
		return fmt.Errorf("insufficient permissions: required scope '%s' not granted", requiredScope)
	}
	return nil
}

// CreateServiceClient creates an OAuth client for service-to-service authentication
// This is for Scenario 3: Tool authenticates to external services
func (h *OAuthHelper) CreateServiceClient(config *ServiceOAuthConfig) (*ServiceOAuthClient, error) {
	if config == nil {
		return nil, fmt.Errorf("OAuth configuration is required")
	}

	clientConfig := &oauthclient.OAuth2ClientConfig{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		IssuerURL:    config.IssuerURL,
		Scope:        config.Scope,
		RequireHTTPS: config.RequireHTTPS,
	}

	oauthClient, err := oauthclient.NewOAuth2Client(clientConfig, h.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth client: %w", err)
	}

	return &ServiceOAuthClient{
		client: oauthClient,
		config: config,
		logger: h.logger,
	}, nil
}

// ServiceOAuthConfig represents OAuth configuration for external service authentication
type ServiceOAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	IssuerURL    string `json:"issuer_url"`
	Scope        string `json:"scope,omitempty"`
	RequireHTTPS bool   `json:"require_https"`
}

// ServiceOAuthClient handles OAuth authentication to external services
type ServiceOAuthClient struct {
	client oauthclient.OAuth2Client
	config *ServiceOAuthConfig
	logger *logrus.Logger
	// Note: token caching will be added in future implementation
}

// GetAuthenticatedHTTPClient returns an HTTP client with OAuth authentication
// This is for Scenario 3: Making authenticated requests to external services
func (c *ServiceOAuthClient) GetAuthenticatedHTTPClient(ctx context.Context) (*http.Client, error) {
	// For now, return a basic HTTP client
	// In a full implementation, this would:
	// 1. Check if we have a valid cached token
	// 2. If not, perform OAuth flow to get token
	// 3. Return HTTP client with Authorisation header set

	return &http.Client{}, fmt.Errorf("service OAuth authentication not yet fully implemented - this is a future enhancement")
}

// Authenticate performs OAuth authentication to the external service
// This is for Scenario 3: Service-to-service authentication
func (c *ServiceOAuthClient) Authenticate(ctx context.Context) error {
	// This would implement the OAuth flow for service-to-service authentication
	// For now, return an error indicating this is a future enhancement
	return fmt.Errorf("service OAuth authentication not yet implemented - this is a future enhancement")
}

// parseScopes parses a space-separated scope string into a slice
func parseScopes(scopeString string) []string {
	if scopeString == "" {
		return []string{}
	}

	scopes := []string{}
	for _, scope := range splitSpaces(scopeString) {
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

// splitSpaces splits a string on spaces and returns non-empty parts
func splitSpaces(s string) []string {
	var result []string
	current := ""

	for _, char := range s {
		if char == ' ' || char == '\t' || char == '\n' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
