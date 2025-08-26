package validation

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sirupsen/logrus"
)

// JWTValidator implements token validation for OAuth 2.1 JWT tokens
type JWTValidator struct {
	config *types.OAuth2Config
	logger *logrus.Logger
	jwks   *JWKSClient
}

// NewJWTValidator creates a new JWT token validator
func NewJWTValidator(config *types.OAuth2Config, logger *logrus.Logger) (*JWTValidator, error) {
	if config == nil {
		return nil, fmt.Errorf("OAuth config is required")
	}

	var jwks *JWKSClient
	if config.JWKSUrl != "" {
		var err error
		jwks, err = NewJWKSClient(config.JWKSUrl, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create JWKS client: %w", err)
		}
	}

	return &JWTValidator{
		config: config,
		logger: logger,
		jwks:   jwks,
	}, nil
}

// ValidateToken validates an OAuth 2.1 JWT token
func (v *JWTValidator) ValidateToken(ctx context.Context, tokenString string) (*types.TokenClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token is required")
	}

	// Parse the JWT token
	token, err := jwt.ParseWithClaims(tokenString, &types.TokenClaims{}, func(token *jwt.Token) (any, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key from JWKS
		if v.jwks != nil {
			return v.jwks.GetKey(ctx, token.Header["kid"].(string))
		}

		return nil, fmt.Errorf("no JWKS configured for token validation")
	})

	if err != nil {
		v.logger.WithError(err).Debug("Token parsing failed")
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*types.TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	if v.config.Issuer != "" && claims.Issuer != v.config.Issuer {
		v.logger.WithFields(logrus.Fields{
			"expected_issuer": v.config.Issuer,
			"token_issuer":    claims.Issuer,
		}).Debug("Token issuer validation failed")
		return nil, fmt.Errorf("invalid issuer")
	}

	// Validate audience (RFC8707 - Resource Indicators)
	if v.config.Audience != "" {
		if !v.validateAudience(claims.Audience, v.config.Audience) {
			v.logger.WithFields(logrus.Fields{
				"expected_audience": v.config.Audience,
				"token_audience":    claims.Audience,
			}).Debug("Token audience validation failed")
			return nil, fmt.Errorf("invalid audience")
		}
	}

	// Validate expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	// Validate not before
	if claims.NotBefore != nil && claims.NotBefore.After(time.Now()) {
		return nil, fmt.Errorf("token not yet valid")
	}

	v.logger.WithFields(logrus.Fields{
		"client_id": claims.ClientID,
		"scope":     claims.Scope,
		"sub":       claims.Subject,
	}).Debug("Token validation successful")

	return claims, nil
}

// validateAudience validates the audience claim according to RFC8707
func (v *JWTValidator) validateAudience(tokenAudience jwt.ClaimStrings, expectedAudience string) bool {
	return slices.Contains(tokenAudience, expectedAudience)
}

// GetJWKS returns the JWKS for this validator
func (v *JWTValidator) GetJWKS(ctx context.Context) (any, error) {
	if v.jwks == nil {
		return nil, fmt.Errorf("no JWKS configured")
	}
	return v.jwks.GetJWKS(ctx)
}

// PKCEValidator handles PKCE code challenge validation
type PKCEValidator struct {
	logger *logrus.Logger
}

// NewPKCEValidator creates a new PKCE validator
func NewPKCEValidator(logger *logrus.Logger) *PKCEValidator {
	return &PKCEValidator{
		logger: logger,
	}
}

// ValidateChallenge validates a PKCE code challenge against a verifier
func (p *PKCEValidator) ValidateChallenge(challenge, method, verifier string) error {
	if challenge == "" || verifier == "" {
		return fmt.Errorf("challenge and verifier are required")
	}

	switch method {
	case "S256":
		// SHA256 challenge method (recommended)
		hash := sha256.Sum256([]byte(verifier))
		encoded := base64.RawURLEncoding.EncodeToString(hash[:])

		if subtle.ConstantTimeCompare([]byte(challenge), []byte(encoded)) != 1 {
			p.logger.Debug("PKCE S256 challenge validation failed")
			return fmt.Errorf("invalid code challenge")
		}

	case "plain":
		// Plain text method (not recommended but supported)
		if subtle.ConstantTimeCompare([]byte(challenge), []byte(verifier)) != 1 {
			p.logger.Debug("PKCE plain challenge validation failed")
			return fmt.Errorf("invalid code challenge")
		}

	default:
		return fmt.Errorf("unsupported challenge method: %s", method)
	}

	p.logger.Debug("PKCE challenge validation successful")
	return nil
}

// GenerateChallenge generates a PKCE code challenge and verifier
func (p *PKCEValidator) GenerateChallenge(method string) (*types.PKCEChallenge, error) {
	// Generate a cryptographically secure random verifier (43-128 characters)
	verifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	var challenge string
	switch method {
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	case "plain":
		challenge = verifier
	default:
		return nil, fmt.Errorf("unsupported challenge method: %s", method)
	}

	return &types.PKCEChallenge{
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
		CodeVerifier:        verifier,
		CreatedAt:           time.Now(),
	}, nil
}

// WWWAuthenticateBuilder builds WWW-Authenticate headers for 401 responses
type WWWAuthenticateBuilder struct {
	resourceMetadataURL string
}

// NewWWWAuthenticateBuilder creates a new WWW-Authenticate header builder
func NewWWWAuthenticateBuilder(resourceMetadataURL string) *WWWAuthenticateBuilder {
	return &WWWAuthenticateBuilder{
		resourceMetadataURL: resourceMetadataURL,
	}
}

// Build builds a WWW-Authenticate header value
func (w *WWWAuthenticateBuilder) Build(realm, error, errorDescription string) string {
	parts := []string{"Bearer"}

	if realm != "" {
		parts = append(parts, fmt.Sprintf(`realm="%s"`, realm))
	}

	if error != "" {
		parts = append(parts, fmt.Sprintf(`error="%s"`, error))
	}

	if errorDescription != "" {
		parts = append(parts, fmt.Sprintf(`error_description="%s"`, errorDescription))
	}

	if w.resourceMetadataURL != "" {
		parts = append(parts, fmt.Sprintf(`resource_metadata_url="%s"`, w.resourceMetadataURL))
	}

	return strings.Join(parts, " ")
}

// ExtractBearerToken extracts a Bearer token from an Authorisation header
func ExtractBearerToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is required")
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", fmt.Errorf("invalid authorization format, expected Bearer token")
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	if token == "" {
		return "", fmt.Errorf("bearer token is empty")
	}

	return token, nil
}

// ValidateHTTPSRequest validates that a request uses HTTPS when required
func ValidateHTTPSRequest(r *http.Request, requireHTTPS bool) error {
	if !requireHTTPS {
		return nil
	}

	// Check if request is HTTPS or from localhost (for development)
	if r.TLS == nil && !isLocalhostRequest(r) {
		return fmt.Errorf("HTTPS is required for OAuth endpoints")
	}

	return nil
}

// isLocalhostRequest checks if a request is from localhost
func isLocalhostRequest(r *http.Request) bool {
	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
