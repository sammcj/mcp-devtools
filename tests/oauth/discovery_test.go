package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/oauth/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// discoveryServer serves whatever metadata document the test supplies at both
// the OpenID Connect and RFC 8414 well-known paths.
func discoveryServer(t *testing.T, doc func(issuer string) map[string]any) *httptest.Server {
	t.Helper()

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(doc(srv.URL)))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func discoveryClient(t *testing.T, issuerURL string) (client.OAuth2Client, *client.OAuth2ClientConfig) {
	t.Helper()

	config := &client.OAuth2ClientConfig{
		ClientID:  "test-client",
		IssuerURL: issuerURL,
	}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	oauthClient, err := client.NewOAuth2Client(config, logger)
	require.NoError(t, err)

	return oauthClient, config
}

// RFC 8414 section 3.2 and OpenID Connect Discovery both require the issuer. A
// document without one has to be rejected: accepting it applies the endpoints
// it declares with no binding at all, which is what the binding exists to stop.
func TestDiscoveryRejectsMetadataWithNoIssuer(t *testing.T) {
	srv := discoveryServer(t, func(string) map[string]any {
		return map[string]any{
			"authorization_endpoint": "https://elsewhere.example.com/authorize",
			"token_endpoint":         "https://elsewhere.example.com/token",
		}
	})

	oauthClient, config := discoveryClient(t, srv.URL)

	require.Error(t, oauthClient.DiscoverEndpoints(t.Context()))
	assert.Empty(t, config.AuthorizationEndpoint, "endpoints from an unbound document were applied")
	assert.Empty(t, config.TokenEndpoint)
}

func TestDiscoveryRejectsNonStringIssuer(t *testing.T) {
	srv := discoveryServer(t, func(string) map[string]any {
		return map[string]any{
			"issuer":                 42,
			"authorization_endpoint": "https://elsewhere.example.com/authorize",
			"token_endpoint":         "https://elsewhere.example.com/token",
		}
	})

	oauthClient, config := discoveryClient(t, srv.URL)

	require.Error(t, oauthClient.DiscoverEndpoints(t.Context()))
	assert.Empty(t, config.AuthorizationEndpoint)
}

func TestDiscoveryAcceptsAMatchingIssuer(t *testing.T) {
	srv := discoveryServer(t, func(issuer string) map[string]any {
		return map[string]any{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/authorize",
			"token_endpoint":         issuer + "/token",
		}
	})

	oauthClient, config := discoveryClient(t, srv.URL)

	require.NoError(t, oauthClient.DiscoverEndpoints(t.Context()))
	assert.Equal(t, srv.URL+"/authorize", config.AuthorizationEndpoint)
	assert.Equal(t, srv.URL, config.IssuerIdentifier)
}

func TestDiscoveryRejectsAnIssuerOtherThanTheConfiguredOne(t *testing.T) {
	srv := discoveryServer(t, func(issuer string) map[string]any {
		return map[string]any{
			"issuer":                 "https://an-unrelated-issuer.example.com",
			"authorization_endpoint": issuer + "/authorize",
			"token_endpoint":         issuer + "/token",
		}
	})

	oauthClient, config := discoveryClient(t, srv.URL)

	require.Error(t, oauthClient.DiscoverEndpoints(t.Context()))
	assert.Empty(t, config.AuthorizationEndpoint)
}

// Discovery builds its URL from configuration, so a redirect off that origin
// would source the endpoints from a party the operator never nominated.
//
// The document served after the redirect claims the *configured* issuer, so it
// satisfies every check other than where it came from. Without the redirect
// policy this succeeds and the off-origin endpoints go into live config.
func TestDiscoveryRefusesOffOriginRedirect(t *testing.T) {
	var configured string

	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 configured,
			"authorization_endpoint": "https://attacker.example.com/authorize",
			"token_endpoint":         "https://attacker.example.com/token",
		}))
	}))
	t.Cleanup(elsewhere.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/.well-known/openid-configuration", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	configured = srv.URL

	oauthClient, config := discoveryClient(t, srv.URL)

	require.Error(t, oauthClient.DiscoverEndpoints(t.Context()))
	assert.Empty(t, config.AuthorizationEndpoint, "followed a redirect off the configured origin")
	assert.Empty(t, config.TokenEndpoint)
}
