package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/sammcj/mcp-devtools/internal/oauth/metadata"
	"github.com/sammcj/mcp-devtools/internal/oauth/registration"
	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sammcj/mcp-devtools/internal/oauth/validation"
	"github.com/sirupsen/logrus"
)

// OAuth2Server implements an OAuth 2.1 resource server
type OAuth2Server struct {
	config           *types.OAuth2Config
	logger           *logrus.Logger
	tokenValidator   types.TokenValidator
	metadataProvider types.MetadataProvider
	clientRegistrar  types.ClientRegistrar
	wwwAuthBuilder   *validation.WWWAuthenticateBuilder
	baseURL          string
}

// NewOAuth2Server creates a new OAuth 2.1 server
func NewOAuth2Server(config *types.OAuth2Config, baseURL string, logger *logrus.Logger) (*OAuth2Server, error) {
	if config == nil {
		return nil, fmt.Errorf("OAuth config is required")
	}

	if !config.Enabled {
		return nil, fmt.Errorf("OAuth is not enabled")
	}

	// Create token validator
	tokenValidator, err := validation.NewJWTValidator(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create token validator: %w", err)
	}

	// Create metadata provider
	metadataProvider := metadata.NewProvider(config, baseURL, logger)

	// Create client registrar if dynamic registration is enabled
	var clientRegistrar types.ClientRegistrar
	if config.DynamicRegistration {
		clientRegistrar = registration.NewInMemoryRegistrar(logger)
	}

	// Create WWW-Authenticate builder
	resourceMetadataURL := baseURL + "/.well-known/oauth-protected-resource"
	wwwAuthBuilder := validation.NewWWWAuthenticateBuilder(resourceMetadataURL)

	return &OAuth2Server{
		config:           config,
		logger:           logger,
		tokenValidator:   tokenValidator,
		metadataProvider: metadataProvider,
		clientRegistrar:  clientRegistrar,
		wwwAuthBuilder:   wwwAuthBuilder,
		baseURL:          baseURL,
	}, nil
}

// AuthenticateRequest authenticates an HTTP request using OAuth 2.1
func (s *OAuth2Server) AuthenticateRequest(ctx context.Context, r *http.Request) *types.AuthenticationResult {
	// Validate HTTPS requirement
	if err := validation.ValidateHTTPSRequest(r, s.config.RequireHTTPS); err != nil {
		s.logger.WithError(err).Debug("HTTPS validation failed")
		return &types.AuthenticationResult{
			Authenticated:   false,
			Error:           err,
			WWWAuthenticate: s.wwwAuthBuilder.Build(s.baseURL, "invalid_request", "HTTPS is required"),
		}
	}

	// Extract Bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		s.logger.Debug("Missing Authorization header")
		return &types.AuthenticationResult{
			Authenticated:   false,
			Error:           fmt.Errorf("authorization header is required"),
			WWWAuthenticate: s.wwwAuthBuilder.Build(s.baseURL, "invalid_request", "Authorization header is required"),
		}
	}

	token, err := validation.ExtractBearerToken(authHeader)
	if err != nil {
		s.logger.WithError(err).Debug("Failed to extract Bearer token")
		return &types.AuthenticationResult{
			Authenticated:   false,
			Error:           err,
			WWWAuthenticate: s.wwwAuthBuilder.Build(s.baseURL, "invalid_request", "Invalid authorization format"),
		}
	}

	// Validate the token
	claims, err := s.tokenValidator.ValidateToken(ctx, token)
	if err != nil {
		s.logger.WithError(err).Debug("Token validation failed")
		errorCode := "invalid_token"
		errorDesc := "The access token is invalid"

		// More specific error messages for common cases
		if strings.Contains(err.Error(), "expired") {
			errorDesc = "The access token has expired"
		} else if strings.Contains(err.Error(), "audience") {
			errorDesc = "The access token audience is invalid"
		} else if strings.Contains(err.Error(), "issuer") {
			errorDesc = "The access token issuer is invalid"
		}

		return &types.AuthenticationResult{
			Authenticated:   false,
			Error:           err,
			WWWAuthenticate: s.wwwAuthBuilder.Build(s.baseURL, errorCode, errorDesc),
		}
	}

	s.logger.WithFields(logrus.Fields{
		"client_id": claims.ClientID,
		"subject":   claims.Subject,
		"scope":     claims.Scope,
	}).Debug("Request authenticated successfully")

	return &types.AuthenticationResult{
		Authenticated: true,
		Claims:        claims,
		Error:         nil,
	}
}

// CreateMiddleware creates an HTTP middleware for OAuth 2.1 authentication
func (s *OAuth2Server) CreateMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for OAuth metadata endpoints
			if s.isOAuthMetadataEndpoint(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Authenticate the request
			result := s.AuthenticateRequest(r.Context(), r)

			if !result.Authenticated {
				// Set WWW-Authenticate header and return 401
				if result.WWWAuthenticate != "" {
					w.Header().Set("WWW-Authenticate", result.WWWAuthenticate)
				}
				w.Header().Set("Content-Type", "application/json")

				// Determine appropriate error response
				oauth2Err := types.ErrInvalidToken
				if result.Error != nil {
					if strings.Contains(result.Error.Error(), "authorization header") {
						oauth2Err = types.ErrInvalidRequest
						oauth2Err.ErrorDescription = "Authorization header is required"
					} else {
						oauth2Err.ErrorDescription = result.Error.Error()
					}
				}

				oauth2Err.WriteHTTPResponse(w, http.StatusUnauthorized)
				return
			}

			// Add claims to request context for downstream handlers
			ctx := context.WithValue(r.Context(), types.OAuthClaimsKey, result.Claims)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// isOAuthMetadataEndpoint checks if a path is an OAuth metadata endpoint
func (s *OAuth2Server) isOAuthMetadataEndpoint(path string) bool {
	oauthPaths := []string{
		"/.well-known/oauth-authorization-server",
		"/.well-known/oauth-protected-resource",
		"/.well-known/jwks.json",
	}

	if s.config.DynamicRegistration {
		oauthPaths = append(oauthPaths, "/oauth/register")
	}

	for _, oauthPath := range oauthPaths {
		if path == oauthPath {
			return true
		}
	}

	return false
}

// RegisterHandlers registers OAuth 2.1 endpoints with an HTTP mux
func (s *OAuth2Server) RegisterHandlers(mux *http.ServeMux) {
	// Register metadata endpoints
	if provider, ok := s.metadataProvider.(*metadata.Provider); ok {
		provider.RegisterHandlers(mux)
	}

	// Register dynamic client registration endpoint if enabled
	if s.config.DynamicRegistration && s.clientRegistrar != nil {
		if registrar, ok := s.clientRegistrar.(*registration.InMemoryRegistrar); ok {
			mux.HandleFunc("/oauth/register", registrar.ServeRegistration)
		}
	}

	s.logger.Info("OAuth 2.1 endpoints registered")
}

// GetClaims extracts OAuth claims from a request context
func GetClaims(ctx context.Context) (*types.TokenClaims, bool) {
	claims, ok := ctx.Value(types.OAuthClaimsKey).(*types.TokenClaims)
	return claims, ok
}

// HasScope checks if the current request has a specific OAuth scope
func HasScope(ctx context.Context, requiredScope string) bool {
	claims, ok := GetClaims(ctx)
	if !ok {
		return false
	}

	// Parse scopes (space-separated)
	scopes := strings.Fields(claims.Scope)
	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}

	return false
}

// RequireScope creates a middleware that requires a specific OAuth scope
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasScope(r.Context(), scope) {
				oauth2Err := types.ErrInsufficientScope
				oauth2Err.ErrorDescription = fmt.Sprintf("Required scope: %s", scope)
				oauth2Err.WriteHTTPResponse(w, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
