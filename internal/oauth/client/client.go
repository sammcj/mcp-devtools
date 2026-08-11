package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sirupsen/logrus"
)

// DefaultOAuth2Client implements OAuth2Client for browser-based authentication
type DefaultOAuth2Client struct {
	config          *OAuth2ClientConfig
	logger          *logrus.Logger
	browserLauncher BrowserLauncher
	httpClient      *http.Client
}

// NewOAuth2Client creates a new OAuth 2.0 client for browser authentication
func NewOAuth2Client(config *OAuth2ClientConfig, logger *logrus.Logger) (OAuth2Client, error) {
	if err := validateClientConfig(config); err != nil {
		return nil, fmt.Errorf("invalid client configuration: %w", err)
	}

	return &DefaultOAuth2Client{
		config:          config,
		logger:          logger,
		browserLauncher: NewBrowserLauncher(),
		httpClient: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: refuseOffOriginRedirect,
		},
	}, nil
}

// maxMetadataBytes bounds a discovery response. A metadata document is a few
// kilobytes; anything larger is a server trying to exhaust this process.
const maxMetadataBytes = 256 << 10

// refuseOffOriginRedirect keeps discovery on the host the configured issuer
// named. The URL is built from configuration, so a redirect elsewhere means the
// authorisation and token endpoints would be sourced from a party the operator
// never nominated. Same-origin redirects are ordinary path normalisation.
func refuseOffOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	from, to := via[0].URL, req.URL
	if from.Scheme != to.Scheme || from.Host != to.Host {
		return fmt.Errorf("discovery request redirected off %s://%s to %s://%s", from.Scheme, from.Host, to.Scheme, to.Host)
	}
	if len(via) >= 10 {
		return fmt.Errorf("too many redirects during discovery")
	}
	return nil
}

// ValidateConfiguration validates the OAuth client configuration
func (c *DefaultOAuth2Client) ValidateConfiguration() error {
	return validateClientConfig(c.config)
}

// DiscoverEndpoints discovers OAuth endpoints from the issuer URL using RFC8414 or OpenID Connect Discovery
func (c *DefaultOAuth2Client) DiscoverEndpoints(ctx context.Context) error {
	if c.config.IssuerURL == "" {
		return fmt.Errorf("issuer URL is required for endpoint discovery")
	}

	baseURL := strings.TrimSuffix(c.config.IssuerURL, "/")

	// Try OpenID Connect Discovery first (most common)
	oidcURL := baseURL + "/.well-known/openid-configuration"
	c.logger.Debugf("Trying OpenID Connect Discovery from: %s", oidcURL)

	if err := c.tryDiscoverFromURL(ctx, oidcURL, true); err == nil {
		return nil
	}

	// Fallback to OAuth 2.0 Authorization Server Metadata (RFC8414)
	oauthURL := baseURL + "/.well-known/oauth-authorization-server"
	c.logger.Debugf("Trying OAuth 2.0 Authorization Server Metadata from: %s", oauthURL)

	if err := c.tryDiscoverFromURL(ctx, oauthURL, false); err == nil {
		return nil
	}

	return fmt.Errorf("failed to discover endpoints from both OpenID Connect Discovery (%s) and OAuth 2.0 Authorization Server Metadata (%s)", oidcURL, oauthURL)
}

// tryDiscoverFromURL attempts to discover endpoints from a specific URL
func (c *DefaultOAuth2Client) tryDiscoverFromURL(ctx context.Context, discoveryURL string, isOpenIDConnect bool) error {
	// Make HTTP request to discovery endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create discovery request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mcp-devtools/oauth-client")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metadata request failed with status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the metadata response - both OpenID Connect and OAuth 2.0 use similar structures
	var metadata map[string]any
	if err := json.Unmarshal(body, &metadata); err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Read everything into locals first. A partially applied discovery leaves
	// an attacker-supplied endpoint in live config even though this returns an
	// error, and StartAuthentication skips re-discovery once the endpoints are
	// set.
	authEndpoint, ok := metadata["authorization_endpoint"].(string)
	if !ok || authEndpoint == "" {
		return fmt.Errorf("authorization_endpoint not found in metadata")
	}

	tokenEndpoint, ok := metadata["token_endpoint"].(string)
	if !ok || tokenEndpoint == "" {
		return fmt.Errorf("token_endpoint not found in metadata")
	}

	// Record the issuer as the server states it. RFC 9207 compares the `iss`
	// parameter against this, not against the URL we happened to fetch from.
	// A server claiming an issuer other than the one configured is a mix-up
	// attempt or a misconfiguration; either way, do not proceed with it.
	//
	// The trailing slash is normalised on both sides here, and only here: this
	// weighs the server's issuer against the operator's own configuration,
	// where a stray slash is a typo rather than a different party. The verbatim
	// declared value is what gets stored and later compared against `iss`, so
	// that check stays exact.
	// RFC 8414 section 3.2 and OpenID Connect Discovery both require the issuer,
	// so a document without one is malformed. Treating it as "nothing to check"
	// would apply its endpoints with no binding at all, which is the one case
	// the binding exists to catch.
	issuerIdentifier, ok := metadata["issuer"].(string)
	if !ok || issuerIdentifier == "" {
		return fmt.Errorf("metadata document has no issuer")
	}
	if configured := strings.TrimSuffix(c.config.IssuerURL, "/"); configured != "" &&
		strings.TrimSuffix(issuerIdentifier, "/") != configured {
		return fmt.Errorf("metadata issuer %q does not match the configured issuer %q", issuerIdentifier, c.config.IssuerURL)
	}

	issuerParameterSupported, _ := metadata["authorization_response_iss_parameter_supported"].(bool)

	c.config.AuthorizationEndpoint = authEndpoint
	c.config.TokenEndpoint = tokenEndpoint
	c.config.IssuerIdentifier = issuerIdentifier
	c.config.IssuerParameterSupported = issuerParameterSupported
	c.logger.Debugf("Discovered authorization endpoint: %s", authEndpoint)
	c.logger.Debugf("Discovered token endpoint: %s", tokenEndpoint)

	discoveryType := "OAuth 2.0 Authorization Server Metadata"
	if isOpenIDConnect {
		discoveryType = "OpenID Connect Discovery"
	}
	c.logger.Infof("Successfully discovered endpoints using %s", discoveryType)

	return nil
}

// expectedIssuer returns the identifier an RFC 9207 `iss` parameter must match.
//
// Neither value is normalised. RFC 8414 compares issuer identifiers exactly,
// and discovery stores what the server declared verbatim, so trimming the
// configured fallback would both reject a server whose issuer genuinely ends in
// "/" and accept the neighbouring identifier that does not.
func (c *DefaultOAuth2Client) expectedIssuer() string {
	if c.config.IssuerIdentifier != "" {
		return c.config.IssuerIdentifier
	}
	return c.config.IssuerURL
}

// StartAuthentication initiates the OAuth 2.0 authorization code flow with PKCE
func (c *DefaultOAuth2Client) StartAuthentication(ctx context.Context) (*AuthenticationSession, error) {
	// Discover endpoints if needed
	if c.config.AuthorizationEndpoint == "" && c.config.IssuerURL != "" {
		if err := c.DiscoverEndpoints(ctx); err != nil {
			return nil, fmt.Errorf("failed to discover endpoints: %w", err)
		}
	}

	// Validate we have required endpoints
	if c.config.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("authorization endpoint is required")
	}

	// Generate PKCE challenge
	pkceChallenge, err := GeneratePKCEChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE challenge: %w", err)
	}

	// Generate state parameter
	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state parameter: %w", err)
	}

	// Create callback server
	callbackServer := NewCallbackServer(c.logger)

	// Start callback server
	if err := callbackServer.Start(ctx, c.config.ServerPort); err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}

	redirectURI := callbackServer.GetRedirectURI()
	c.logger.Debugf("OAuth callback server started at: %s", redirectURI)

	// Register what the authorisation response must carry before any code from
	// it is redeemed.
	callbackServer.Expect(state, c.expectedIssuer(), c.config.IssuerParameterSupported)

	// Build authorization URL
	authURL, err := c.buildAuthorizationURL(redirectURI, state, pkceChallenge)
	if err != nil {
		_ = callbackServer.Stop() // Error already logged in Stop()
		return nil, fmt.Errorf("failed to build authorization URL: %w", err)
	}

	// Create session context with timeout
	sessionCtx, cancel := context.WithTimeout(ctx, c.getAuthTimeout())

	session := &AuthenticationSession{
		PKCEChallenge:  pkceChallenge,
		State:          state,
		RedirectURI:    redirectURI,
		AuthURL:        authURL,
		CallbackServer: callbackServer,
		ResultCh:       make(chan *AuthenticationResult, 1),
		ErrorCh:        make(chan error, 1),
		Context:        sessionCtx,
		Cancel:         cancel,
	}

	// Start the authentication flow in a goroutine
	go c.handleAuthenticationFlow(session)

	// Open browser to authorization URL
	c.logger.Infof("Opening browser for OAuth authentication: %s", authURL)
	if err := c.browserLauncher.OpenURL(authURL); err != nil {
		cancel()
		_ = callbackServer.Stop() // Error already logged in Stop()
		return nil, fmt.Errorf("failed to open browser: %w", err)
	}

	return session, nil
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (c *DefaultOAuth2Client) ExchangeCodeForToken(ctx context.Context, code string, pkce *types.PKCEChallenge, redirectURI string) (*TokenResponse, error) {
	if c.config.TokenEndpoint == "" {
		return nil, fmt.Errorf("token endpoint is required")
	}

	// Prepare token request
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {c.config.ClientID},
		"code_verifier": {pkce.CodeVerifier},
	}

	// Add client secret if present (for confidential clients)
	if c.config.ClientSecret != "" {
		data.Set("client_secret", c.config.ClientSecret)
	}

	// Add resource parameter if specified (RFC8707)
	if c.config.Resource != "" {
		data.Set("resource", c.config.Resource)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.TokenEndpoint,
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mcp-devtools/oauth-client")

	c.logger.Debug("Exchanging authorization code for access token")

	// Make the token request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var oauthErr types.OAuth2Error
		if err := json.Unmarshal(body, &oauthErr); err == nil && oauthErr.Error != "" {
			return nil, fmt.Errorf("oauth error: %s - %s", oauthErr.Error, oauthErr.ErrorDescription)
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful token response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Validate required fields
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}

	if tokenResp.TokenType == "" {
		tokenResp.TokenType = "Bearer" // Default to Bearer if not specified
	}

	c.logger.Info("Successfully exchanged authorization code for access token")
	return &tokenResp, nil
}

// buildAuthorizationURL builds the OAuth authorization URL with PKCE
func (c *DefaultOAuth2Client) buildAuthorizationURL(redirectURI, state string, pkce *types.PKCEChallenge) (string, error) {
	baseURL, err := url.Parse(c.config.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid authorization endpoint: %w", err)
	}

	// Build query parameters
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.config.ClientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {pkce.CodeChallenge},
		"code_challenge_method": {pkce.CodeChallengeMethod},
	}

	// Add scope if specified
	if c.config.Scope != "" {
		params.Set("scope", c.config.Scope)
	}

	// Add resource parameter if specified (RFC8707)
	if c.config.Resource != "" {
		params.Set("resource", c.config.Resource)
	}

	baseURL.RawQuery = params.Encode()
	return baseURL.String(), nil
}

// handleAuthenticationFlow handles the complete authentication flow
func (c *DefaultOAuth2Client) handleAuthenticationFlow(session *AuthenticationSession) {
	defer func() {
		session.Cancel()
		_ = session.CallbackServer.Stop() // Error already logged in Stop()
	}()

	select {
	case code := <-session.CallbackServer.GetAuthorizationCode():
		c.logger.Debug("Received authorization code from callback")

		// Exchange code for token
		tokenResp, err := c.ExchangeCodeForToken(session.Context, code, session.PKCEChallenge, session.RedirectURI)
		if err != nil {
			session.ErrorCh <- fmt.Errorf("failed to exchange code for token: %w", err)
			return
		}

		// Send successful result
		result := &AuthenticationResult{
			Success:       true,
			TokenResponse: tokenResp,
			State:         session.State,
			ExchangedAt:   time.Now(),
		}

		select {
		case session.ResultCh <- result:
		case <-session.Context.Done():
		}

	case err := <-session.CallbackServer.GetError():
		c.logger.WithError(err).Error("OAuth callback error")
		session.ErrorCh <- err

	case <-session.Context.Done():
		c.logger.Debug("OAuth authentication session timed out")
		session.ErrorCh <- fmt.Errorf("authentication timed out")
	}
}

// getAuthTimeout returns the authentication timeout duration
func (c *DefaultOAuth2Client) getAuthTimeout() time.Duration {
	if c.config.AuthTimeout > 0 {
		return c.config.AuthTimeout
	}
	return 5 * time.Minute // Default timeout
}

// validateClientConfig validates the OAuth client configuration
func validateClientConfig(config *OAuth2ClientConfig) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	if config.ClientID == "" {
		return fmt.Errorf("client ID is required")
	}

	// Either endpoints must be provided directly, or issuer URL for discovery
	if config.AuthorizationEndpoint == "" && config.IssuerURL == "" {
		return fmt.Errorf("either authorization endpoint or issuer URL must be provided")
	}

	if config.TokenEndpoint == "" && config.IssuerURL == "" {
		return fmt.Errorf("either token endpoint or issuer URL must be provided")
	}

	return nil
}
