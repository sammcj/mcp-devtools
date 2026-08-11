package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ServerMetadata holds OAuth authorisation server metadata.
type ServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`

	// IssuerParameterSupported is RFC 9207. When true, an authorisation
	// response without `iss` must be rejected.
	IssuerParameterSupported bool `json:"authorization_response_iss_parameter_supported,omitempty"`
}

// FetchServerMetadata fetches OAuth authorisation server metadata.
// Follows RFC 8414 and MCP spec for discovery.
func FetchServerMetadata(ctx context.Context, serverURL string) (*ServerMetadata, error) {
	logrus.WithField("server_url", serverURL).Debug("auth: fetching server metadata")

	parsed, err := url.Parse(serverURL)
	if err != nil {
		logrus.WithError(err).WithField("url", serverURL).Error("auth: invalid server URL")
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// RFC 8414 section 3.1 builds the well-known URL by inserting the path into
	// the issuer's, so each candidate URL implies exactly one issuer that a
	// document served there is allowed to claim. Try the issuer-with-path form
	// first, then the bare one.
	origin := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	candidates := []struct{ url, issuer string }{
		{origin + "/.well-known/oauth-authorization-server" + parsed.Path, origin + parsed.Path},
		{origin + "/.well-known/oauth-authorization-server", origin},
	}

	var lastErr error
	for _, candidate := range candidates {
		logrus.WithField("url", candidate.url).Debug("auth: trying metadata URL")
		metadata, err := fetchMetadataFromURL(ctx, candidate.url, candidate.issuer)
		if err == nil {
			logrus.WithField("issuer", metadata.Issuer).Debug("auth: metadata fetched successfully")
			return metadata, nil
		}
		logrus.WithError(err).WithField("url", candidate.url).Debug("auth: metadata fetch failed")
		lastErr = err
	}

	logrus.WithError(lastErr).Error("auth: failed to fetch server metadata from all paths")
	return nil, fmt.Errorf("failed to fetch authorisation server metadata: %w", lastErr)
}

// maxMetadataBytes bounds the discovery response. A metadata document is a few
// kilobytes; anything larger is a server trying to exhaust this process.
const maxMetadataBytes = 256 << 10

// metadataClient refuses a redirect that leaves the origin. Discovery derives
// the URL from configuration, so the document has to come from the host that
// configuration named; a redirect elsewhere means the answer is being sourced
// from a party the user never nominated. Same-origin redirects still work, so
// a provider normalising a path is unaffected.
var metadataClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		from, to := via[0].URL, req.URL
		if from.Scheme != to.Scheme || from.Host != to.Host {
			return fmt.Errorf("metadata request redirected off %s://%s to %s://%s", from.Scheme, from.Host, to.Scheme, to.Host)
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects fetching metadata")
		}
		return nil
	},
}

func fetchMetadataFromURL(ctx context.Context, metadataURL, expectedIssuer string) (*ServerMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := metadataClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata endpoint returned %d", resp.StatusCode)
	}

	var metadata ServerMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetadataBytes)).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	if err := validateIssuer(metadata.Issuer, expectedIssuer); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// validateIssuer requires the RFC 8414 section 3.3 exact match against the
// issuer the well-known URL was built from. Comparing only scheme and host
// would let one tenant on a shared host answer for another.
//
// The single exception is the root: an issuer with an empty path and one with
// "/" produce the same well-known URL, so neither can be distinguished from the
// other and both are accepted.
func validateIssuer(issuer, expected string) error {
	if issuer == "" {
		return fmt.Errorf("metadata document has no issuer")
	}

	if issuer == expected {
		return nil
	}
	if strings.TrimSuffix(issuer, "/") == expected {
		return nil
	}

	return fmt.Errorf("metadata document claims issuer %q but was served from the well-known URL for %q", issuer, expected)
}

// ValidateScopes validates requested scopes against supported scopes.
func (m *ServerMetadata) ValidateScopes(requested []string) []string {
	if len(m.ScopesSupported) == 0 {
		return requested
	}

	supported := make(map[string]bool)
	for _, s := range m.ScopesSupported {
		supported[s] = true
	}

	var valid []string
	for _, s := range requested {
		if supported[s] {
			valid = append(valid, s)
		}
	}
	return valid
}

// SupportsPKCE returns true if the server supports PKCE with S256.
func (m *ServerMetadata) SupportsPKCE() bool {
	for _, method := range m.CodeChallengeMethodsSupported {
		if strings.ToUpper(method) == "S256" {
			return true
		}
	}
	// If not specified, assume PKCE is supported (required by MCP spec)
	return len(m.CodeChallengeMethodsSupported) == 0
}
