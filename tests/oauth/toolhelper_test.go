package oauth

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sammcj/mcp-devtools/internal/oauth/toolhelper"
	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthHelper_GetUserClaims(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests
	helper := toolhelper.NewOAuthHelper(logger)

	t.Run("ValidClaims", func(t *testing.T) {
		claims := &types.TokenClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:  "user123",
				Issuer:   "https://auth.example.com",
				Audience: jwt.ClaimStrings{"https://mcp.example.com"},
			},
			ClientID: "test-client",
			Scope:    "openid profile mcp:tools",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		result, err := helper.GetUserClaims(ctx)
		require.NoError(t, err)
		assert.Equal(t, claims.Subject, result.Subject)
		assert.Equal(t, claims.ClientID, result.ClientID)
		assert.Equal(t, claims.Scope, result.Scope)
	})

	t.Run("NoClaims", func(t *testing.T) {
		ctx := t.Context()

		_, err := helper.GetUserClaims(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no OAuth claims found in context")
	})

	t.Run("WrongType", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, "not-claims")

		_, err := helper.GetUserClaims(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no OAuth claims found in context")
	})
}

func TestOAuthHelper_HasScope(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	t.Run("HasRequiredScope", func(t *testing.T) {
		claims := &types.TokenClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "user123",
			},
			Scope: "openid profile mcp:documents:read mcp:tools",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		assert.True(t, helper.HasScope(ctx, "mcp:documents:read"))
		assert.True(t, helper.HasScope(ctx, "openid"))
		assert.True(t, helper.HasScope(ctx, "profile"))
		assert.True(t, helper.HasScope(ctx, "mcp:tools"))
	})

	t.Run("DoesNotHaveScope", func(t *testing.T) {
		claims := &types.TokenClaims{
			Scope: "openid profile",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		assert.False(t, helper.HasScope(ctx, "mcp:documents:read"))
		assert.False(t, helper.HasScope(ctx, "mcp:admin"))
	})

	t.Run("EmptyScope", func(t *testing.T) {
		claims := &types.TokenClaims{
			Scope: "",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		assert.False(t, helper.HasScope(ctx, "openid"))
	})

	t.Run("NoClaims", func(t *testing.T) {
		ctx := t.Context()

		assert.False(t, helper.HasScope(ctx, "openid"))
	})

	t.Run("ScopeWithExtraSpaces", func(t *testing.T) {
		claims := &types.TokenClaims{
			Scope: "  openid   profile   mcp:tools  ",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		assert.True(t, helper.HasScope(ctx, "openid"))
		assert.True(t, helper.HasScope(ctx, "profile"))
		assert.True(t, helper.HasScope(ctx, "mcp:tools"))
	})
}

func TestOAuthHelper_RequireScope(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	t.Run("HasRequiredScope", func(t *testing.T) {
		claims := &types.TokenClaims{
			Scope: "openid profile mcp:documents:read",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		err := helper.RequireScope(ctx, "mcp:documents:read")
		assert.NoError(t, err)
	})

	t.Run("MissingRequiredScope", func(t *testing.T) {
		claims := &types.TokenClaims{
			Scope: "openid profile",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		err := helper.RequireScope(ctx, "mcp:documents:write")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient permissions")
		assert.Contains(t, err.Error(), "mcp:documents:write")
	})

	t.Run("NoClaims", func(t *testing.T) {
		ctx := t.Context()

		err := helper.RequireScope(ctx, "openid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient permissions")
	})
}

func TestOAuthHelper_GetUserToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	t.Run("NotYetImplemented", func(t *testing.T) {
		claims := &types.TokenClaims{
			Scope: "openid profile",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		_, err := helper.GetUserToken(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "future enhancement")
	})

	t.Run("NoClaims", func(t *testing.T) {
		ctx := t.Context()

		_, err := helper.GetUserToken(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no OAuth claims found in context")
	})
}

func TestOAuthHelper_CreateServiceClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	t.Run("ValidConfig", func(t *testing.T) {
		config := &toolhelper.ServiceOAuthConfig{
			ClientID:     "confluence-client",
			ClientSecret: "secret",
			IssuerURL:    "https://auth.atlassian.com",
			Scope:        "read:confluence-content.all",
			RequireHTTPS: true,
		}

		client, err := helper.CreateServiceClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("NilConfig", func(t *testing.T) {
		_, err := helper.CreateServiceClient(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OAuth configuration is required")
	})

	t.Run("EmptyClientID", func(t *testing.T) {
		config := &toolhelper.ServiceOAuthConfig{
			ClientID:     "", // Empty client ID should cause validation error
			ClientSecret: "secret",
			IssuerURL:    "https://auth.atlassian.com",
		}

		_, err := helper.CreateServiceClient(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create OAuth client")
	})
}

func TestServiceOAuthClient_GetAuthenticatedHTTPClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	config := &toolhelper.ServiceOAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "secret",
		IssuerURL:    "https://auth.example.com",
		Scope:        "read:api",
		RequireHTTPS: true,
	}

	client, err := helper.CreateServiceClient(config)
	require.NoError(t, err)

	t.Run("NotYetImplemented", func(t *testing.T) {
		ctx := t.Context()

		_, err := client.GetAuthenticatedHTTPClient(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not yet fully implemented")
		assert.Contains(t, err.Error(), "future enhancement")
	})
}

func TestServiceOAuthClient_Authenticate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	config := &toolhelper.ServiceOAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "secret",
		IssuerURL:    "https://auth.example.com",
		Scope:        "read:api",
		RequireHTTPS: true,
	}

	client, err := helper.CreateServiceClient(config)
	require.NoError(t, err)

	t.Run("NotYetImplemented", func(t *testing.T) {
		ctx := t.Context()

		err := client.Authenticate(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "future enhancement")
	})
}

// Test helper functions for scope parsing
func TestScopeParsing(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	testCases := []struct {
		name     string
		scope    string
		required string
		expected bool
	}{
		{
			name:     "SingleScope",
			scope:    "openid",
			required: "openid",
			expected: true,
		},
		{
			name:     "MultipleScopes",
			scope:    "openid profile email",
			required: "profile",
			expected: true,
		},
		{
			name:     "ScopeNotPresent",
			scope:    "openid profile",
			required: "admin",
			expected: false,
		},
		{
			name:     "EmptyScope",
			scope:    "",
			required: "openid",
			expected: false,
		},
		{
			name:     "ExtraWhitespace",
			scope:    "  openid   profile   email  ",
			required: "profile",
			expected: true,
		},
		{
			name:     "TabsAndNewlines",
			scope:    "openid\tprofile\nemail",
			required: "email",
			expected: true,
		},
		{
			name:     "ColonInScope",
			scope:    "openid mcp:documents:read mcp:tools",
			required: "mcp:documents:read",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			claims := &types.TokenClaims{
				Scope: tc.scope,
			}

			ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

			result := helper.HasScope(ctx, tc.required)
			assert.Equal(t, tc.expected, result, "Scope: '%s', Required: '%s'", tc.scope, tc.required)
		})
	}
}

// Integration test showing how tools would use the OAuth helper
func TestOAuthHelper_ToolIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	helper := toolhelper.NewOAuthHelper(logger)

	t.Run("DocumentToolScenario", func(t *testing.T) {
		// Simulate a document tool that requires specific permissions
		claims := &types.TokenClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:  "user123",
				Issuer:   "https://auth.example.com",
				Audience: jwt.ClaimStrings{"https://mcp.example.com"},
			},
			ClientID: "mcp-client",
			Scope:    "openid profile mcp:documents:read mcp:documents:write",
		}

		ctx := context.WithValue(t.Context(), types.OAuthClaimsKey, claims)

		// Tool checks authentication
		userClaims, err := helper.GetUserClaims(ctx)
		require.NoError(t, err)
		assert.Equal(t, "user123", userClaims.Subject)

		// Tool checks read permission
		err = helper.RequireScope(ctx, "mcp:documents:read")
		assert.NoError(t, err)

		// Tool checks write permission
		err = helper.RequireScope(ctx, "mcp:documents:write")
		assert.NoError(t, err)

		// Tool checks admin permission (should fail)
		err = helper.RequireScope(ctx, "mcp:documents:admin")
		assert.Error(t, err)

		// Tool checks optional admin features
		hasAdmin := helper.HasScope(ctx, "mcp:admin")
		assert.False(t, hasAdmin)
	})

	t.Run("UnauthenticatedUser", func(t *testing.T) {
		// Simulate unauthenticated request
		ctx := t.Context()

		// Tool should fail to get claims
		_, err := helper.GetUserClaims(ctx)
		assert.Error(t, err)

		// Tool should fail scope checks
		err = helper.RequireScope(ctx, "mcp:documents:read")
		assert.Error(t, err)

		hasScope := helper.HasScope(ctx, "openid")
		assert.False(t, hasScope)
	})
}
