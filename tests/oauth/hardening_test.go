package oauth

import (
	"context"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/oauth/metadata"
	"github.com/sammcj/mcp-devtools/internal/oauth/registration"
	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sammcj/mcp-devtools/internal/oauth/validation"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func quietLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.FatalLevel)
	return l
}

// OAuth 2.1 forbids the plain PKCE method, and dynamic client registration is
// deprecated, so neither may be advertised by default.
func TestMetadataAdvertisesHardenedFeatures(t *testing.T) {
	provider := metadata.NewProvider(
		&types.OAuth2Config{Enabled: true, Issuer: "https://issuer.example"},
		"https://mcp.example",
		quietLogger(),
	)

	got, err := provider.GetAuthorizationServerMetadata(context.Background())
	require.NoError(t, err)

	// OAuth 2.1 forbids the plain PKCE method, so it must not be advertised.
	assert.NotContains(t, got.CodeChallengeMethodsSupported, "plain")
	assert.Contains(t, got.CodeChallengeMethodsSupported, "S256")

	// Dynamic registration is deprecated, so it is only advertised on request.
	assert.Empty(t, got.RegistrationEndpoint)
}

func TestRegistrarApplicationType(t *testing.T) {
	registrar := registration.NewInMemoryRegistrar(quietLogger())

	t.Run("native is preserved", func(t *testing.T) {
		got, err := registrar.RegisterClient(context.Background(), &types.DynamicClientRegistrationRequest{
			RedirectURIs:    []string{"http://127.0.0.1:8080/callback"},
			ApplicationType: "native",
		})
		require.NoError(t, err)
		assert.Equal(t, "native", got.ApplicationType)
	})

	t.Run("absent defaults to web", func(t *testing.T) {
		got, err := registrar.RegisterClient(context.Background(), &types.DynamicClientRegistrationRequest{
			RedirectURIs: []string{"https://app.example/callback"},
		})
		require.NoError(t, err)
		assert.Equal(t, "web", got.ApplicationType)
	})

	t.Run("unknown value is rejected", func(t *testing.T) {
		_, err := registrar.RegisterClient(context.Background(), &types.DynamicClientRegistrationRequest{
			RedirectURIs:    []string{"https://app.example/callback"},
			ApplicationType: "spaceship",
		})
		assert.Error(t, err)
	})
}

// The plain PKCE method lets anyone who captured the authorisation request
// replay it, so it must be rejected outright rather than merely unadvertised.
func TestPKCERejectsPlainMethod(t *testing.T) {
	v := validation.NewPKCEValidator(quietLogger())

	// Generating a plain challenge is refused too, so nothing internal can
	// produce a pair that would then be validated.
	_, err := v.GenerateChallenge("plain")
	assert.Error(t, err)

	pkce, err := v.GenerateChallenge("S256")
	require.NoError(t, err)
	assert.NoError(t, v.ValidateChallenge(pkce.CodeChallenge, "S256", pkce.CodeVerifier))
	assert.Error(t, v.ValidateChallenge(pkce.CodeVerifier, "plain", pkce.CodeVerifier))
}
