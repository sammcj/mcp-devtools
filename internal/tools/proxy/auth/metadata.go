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

	// Try MCP-specific path first: /.well-known/oauth-authorization-server/{path}
	// Then fall back to standard: /.well-known/oauth-authorization-server
	paths := []string{
		fmt.Sprintf("/.well-known/oauth-authorization-server%s", parsed.Path),
		"/.well-known/oauth-authorization-server",
	}

	var lastErr error
	for _, path := range paths {
		metadataURL := fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, path)
		logrus.WithField("url", metadataURL).Debug("auth: trying metadata URL")
		metadata, err := fetchMetadataFromURL(ctx, metadataURL)
		if err == nil {
			logrus.WithField("issuer", metadata.Issuer).Debug("auth: metadata fetched successfully")
			return metadata, nil
		}
		logrus.WithError(err).WithField("url", metadataURL).Debug("auth: metadata fetch failed")
		lastErr = err
	}

	logrus.WithError(lastErr).Error("auth: failed to fetch server metadata from all paths")
	return nil, fmt.Errorf("failed to fetch authorisation server metadata: %w", lastErr)
}

// maxMetadataBytes bounds the discovery response. A metadata document is a few
// kilobytes; anything larger is a server trying to exhaust this process.
const maxMetadataBytes = 256 << 10

var metadataClient = &http.Client{Timeout: 30 * time.Second}

func fetchMetadataFromURL(ctx context.Context, metadataURL string) (*ServerMetadata, error) {
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

	// Check against where the document actually came from, not where it was
	// requested: the client follows redirects, and validating the pre-redirect
	// URL would let an off-host redirect deliver an attacker-authored document
	// that still passes.
	servedFrom := metadataURL
	if resp.Request != nil && resp.Request.URL != nil {
		servedFrom = resp.Request.URL.String()
	}
	if err := validateIssuerMatchesURL(metadata.Issuer, servedFrom); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// validateIssuerMatchesURL checks scheme and host only, not the full RFC 8414
// §3.3 equality. That blocks the cross-host mix-up, which is the attack that
// matters here, while tolerating providers that issue from a path (Authentik
// "/application/o/<slug>/", Okta "/oauth2/<id>") where full-path equality
// against the two constructed well-known URLs would reject a valid document.
func validateIssuerMatchesURL(issuer, metadataURL string) error {
	if issuer == "" {
		return fmt.Errorf("metadata document has no issuer")
	}

	fetched, err := url.Parse(metadataURL)
	if err != nil {
		return fmt.Errorf("invalid metadata URL: %w", err)
	}

	claimed, err := url.Parse(issuer)
	if err != nil {
		return fmt.Errorf("invalid issuer %q: %w", issuer, err)
	}

	if claimed.Scheme != fetched.Scheme || claimed.Host != fetched.Host {
		return fmt.Errorf("metadata issuer %q was served from %s://%s", issuer, fetched.Scheme, fetched.Host)
	}

	return nil
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
