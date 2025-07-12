package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/oauth/metadata"
	"github.com/sammcj/mcp-devtools/internal/oauth/registration"
	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sammcj/mcp-devtools/internal/oauth/validation"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataProvider(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests

	config := &types.OAuth2Config{
		Enabled:             true,
		Issuer:              "https://auth.example.com",
		Audience:            "https://mcp.example.com",
		JWKSUrl:             "https://auth.example.com/.well-known/jwks.json",
		DynamicRegistration: true,
		RequireHTTPS:        true,
	}

	baseURL := "https://mcp.example.com"
	provider := metadata.NewProvider(config, baseURL, logger)

	t.Run("GetAuthorizationServerMetadata", func(t *testing.T) {
		metadata, err := provider.GetAuthorizationServerMetadata(context.Background())
		require.NoError(t, err)
		
		assert.Equal(t, config.Issuer, metadata.Issuer)
		assert.Equal(t, baseURL+"/oauth/authorize", metadata.AuthorizationEndpoint)
		assert.Equal(t, baseURL+"/oauth/token", metadata.TokenEndpoint)
		assert.Equal(t, baseURL+"/.well-known/jwks.json", metadata.JWKSUri)
		assert.Equal(t, baseURL+"/oauth/register", metadata.RegistrationEndpoint)
		assert.Contains(t, metadata.ResponseTypesSupported, "code")
		assert.Contains(t, metadata.CodeChallengeMethodsSupported, "S256")
	})

	t.Run("GetProtectedResourceMetadata", func(t *testing.T) {
		metadata, err := provider.GetProtectedResourceMetadata(context.Background())
		require.NoError(t, err)
		
		assert.Equal(t, config.Audience, metadata.Resource)
		assert.Contains(t, metadata.AuthorizationServers, config.Issuer)
		assert.Contains(t, metadata.BearerMethodsSupported, "header")
		assert.Contains(t, metadata.ResourceSigningAlgValuesSupported, "RS256")
	})

	t.Run("ServeAuthorizationServerMetadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		
		provider.ServeAuthorizationServerMetadata(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		
		var metadata types.AuthorizationServerMetadata
		err := json.NewDecoder(w.Body).Decode(&metadata)
		require.NoError(t, err)
		assert.Equal(t, config.Issuer, metadata.Issuer)
	})

	t.Run("ServeProtectedResourceMetadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
		w := httptest.NewRecorder()
		
		provider.ServeProtectedResourceMetadata(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		
		var metadata types.ProtectedResourceMetadata
		err := json.NewDecoder(w.Body).Decode(&metadata)
		require.NoError(t, err)
		assert.Equal(t, config.Audience, metadata.Resource)
	})
}

func TestPKCEValidator(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	validator := validation.NewPKCEValidator(logger)

	t.Run("GenerateChallenge_S256", func(t *testing.T) {
		challenge, err := validator.GenerateChallenge("S256")
		require.NoError(t, err)
		
		assert.Equal(t, "S256", challenge.CodeChallengeMethod)
		assert.NotEmpty(t, challenge.CodeChallenge)
		assert.NotEmpty(t, challenge.CodeVerifier)
		assert.True(t, len(challenge.CodeVerifier) >= 43)
		assert.True(t, len(challenge.CodeVerifier) <= 128)
	})

	t.Run("GenerateChallenge_Plain", func(t *testing.T) {
		challenge, err := validator.GenerateChallenge("plain")
		require.NoError(t, err)
		
		assert.Equal(t, "plain", challenge.CodeChallengeMethod)
		assert.Equal(t, challenge.CodeVerifier, challenge.CodeChallenge)
	})

	t.Run("ValidateChallenge_S256", func(t *testing.T) {
		challenge, err := validator.GenerateChallenge("S256")
		require.NoError(t, err)
		
		err = validator.ValidateChallenge(challenge.CodeChallenge, "S256", challenge.CodeVerifier)
		assert.NoError(t, err)
		
		// Test with wrong verifier
		err = validator.ValidateChallenge(challenge.CodeChallenge, "S256", "wrong-verifier")
		assert.Error(t, err)
	})

	t.Run("ValidateChallenge_Plain", func(t *testing.T) {
		challenge, err := validator.GenerateChallenge("plain")
		require.NoError(t, err)
		
		err = validator.ValidateChallenge(challenge.CodeChallenge, "plain", challenge.CodeVerifier)
		assert.NoError(t, err)
		
		// Test with wrong verifier
		err = validator.ValidateChallenge(challenge.CodeChallenge, "plain", "wrong-verifier")
		assert.Error(t, err)
	})
}

func TestDynamicClientRegistration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	registrar := registration.NewInMemoryRegistrar(logger)

	t.Run("RegisterClient", func(t *testing.T) {
		req := &types.DynamicClientRegistrationRequest{
			RedirectURIs:     []string{"https://client.example.com/callback"},
			ClientName:       "Test Client",
			GrantTypes:       []string{"authorization_code"},
			ResponseTypes:    []string{"code"},
			Scope:            "openid profile",
			TokenEndpointAuthMethod: "client_secret_basic",
		}

		response, err := registrar.RegisterClient(context.Background(), req)
		require.NoError(t, err)
		
		assert.NotEmpty(t, response.ClientID)
		assert.NotEmpty(t, response.ClientSecret)
		assert.Equal(t, req.RedirectURIs, response.RedirectURIs)
		assert.Equal(t, req.ClientName, response.ClientName)
		assert.Equal(t, req.GrantTypes, response.GrantTypes)
		assert.Equal(t, req.ResponseTypes, response.ResponseTypes)
		assert.Equal(t, req.Scope, response.Scope)
		assert.Equal(t, req.TokenEndpointAuthMethod, response.TokenEndpointAuthMethod)
		assert.True(t, response.ClientIDIssuedAt > 0)
		assert.True(t, response.ClientSecretExpiresAt > 0)
	})

	t.Run("RegisterClient_InvalidRedirectURI", func(t *testing.T) {
		req := &types.DynamicClientRegistrationRequest{
			RedirectURIs: []string{"http://evil.example.com/callback"}, // HTTP not allowed for non-localhost
			ClientName:   "Evil Client",
		}

		_, err := registrar.RegisterClient(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP redirect URIs only allowed for localhost")
	})

	t.Run("RegisterClient_LocalhostHTTP", func(t *testing.T) {
		req := &types.DynamicClientRegistrationRequest{
			RedirectURIs: []string{"http://localhost:8080/callback"}, // Localhost HTTP is allowed
			ClientName:   "Localhost Client",
		}

		response, err := registrar.RegisterClient(context.Background(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, response.ClientID)
	})

	t.Run("GetClient", func(t *testing.T) {
		// First register a client
		req := &types.DynamicClientRegistrationRequest{
			RedirectURIs: []string{"https://client.example.com/callback"},
			ClientName:   "Test Client for Get",
		}

		registered, err := registrar.RegisterClient(context.Background(), req)
		require.NoError(t, err)

		// Then retrieve it
		retrieved, err := registrar.GetClient(context.Background(), registered.ClientID)
		require.NoError(t, err)
		
		assert.Equal(t, registered.ClientID, retrieved.ClientID)
		assert.Equal(t, registered.ClientName, retrieved.ClientName)
	})

	t.Run("GetClient_NotFound", func(t *testing.T) {
		_, err := registrar.GetClient(context.Background(), "non-existent-client")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client not found")
	})
}

func TestWWWAuthenticateBuilder(t *testing.T) {
	builder := validation.NewWWWAuthenticateBuilder("https://mcp.example.com/.well-known/oauth-protected-resource")

	t.Run("BuildBasic", func(t *testing.T) {
		header := builder.Build("https://mcp.example.com", "", "")
		expected := `Bearer realm="https://mcp.example.com" resource_metadata_url="https://mcp.example.com/.well-known/oauth-protected-resource"`
		assert.Equal(t, expected, header)
	})

	t.Run("BuildWithError", func(t *testing.T) {
		header := builder.Build("https://mcp.example.com", "invalid_token", "The access token is invalid")
		assert.Contains(t, header, `error="invalid_token"`)
		assert.Contains(t, header, `error_description="The access token is invalid"`)
		assert.Contains(t, header, `realm="https://mcp.example.com"`)
	})
}

func TestExtractBearerToken(t *testing.T) {
	t.Run("ValidToken", func(t *testing.T) {
		token, err := validation.ExtractBearerToken("Bearer eyJhbGciOiJIUzI1NiIs")
		require.NoError(t, err)
		assert.Equal(t, "eyJhbGciOiJIUzI1NiIs", token)
	})

	t.Run("NoAuthHeader", func(t *testing.T) {
		_, err := validation.ExtractBearerToken("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authorization header is required")
	})

	t.Run("NotBearer", func(t *testing.T) {
		_, err := validation.ExtractBearerToken("Basic dXNlcjpwYXNz")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected Bearer token")
	})

	t.Run("EmptyToken", func(t *testing.T) {
		_, err := validation.ExtractBearerToken("Bearer ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bearer token is empty")
	})
}

func TestValidateHTTPSRequest(t *testing.T) {
	t.Run("HTTPS_Required_WithTLS", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://example.com/test", nil)
		req.TLS = &http.Request{}.TLS // Simulate TLS
		
		err := validation.ValidateHTTPSRequest(req, true)
		assert.NoError(t, err)
	})

	t.Run("HTTPS_Required_Localhost", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
		
		err := validation.ValidateHTTPSRequest(req, true)
		assert.NoError(t, err) // Localhost should be allowed even with HTTP
	})

	t.Run("HTTPS_Required_NonLocalhost_HTTP", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		
		err := validation.ValidateHTTPSRequest(req, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTPS is required")
	})

	t.Run("HTTPS_NotRequired", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		
		err := validation.ValidateHTTPSRequest(req, false)
		assert.NoError(t, err)
	})
}

func TestOAuth2Error(t *testing.T) {
	t.Run("WriteHTTPResponse", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := types.OAuth2Error{
			Error:            "invalid_token",
			ErrorDescription: "The access token is invalid",
			ErrorURI:         "https://example.com/error",
		}
		
		err.WriteHTTPResponse(w, http.StatusUnauthorized)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
		
		body := w.Body.String()
		assert.Contains(t, body, `"error":"invalid_token"`)
		assert.Contains(t, body, `"error_description":"The access token is invalid"`)
		assert.Contains(t, body, `"error_uri":"https://example.com/error"`)
	})
}