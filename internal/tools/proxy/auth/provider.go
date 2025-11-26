package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Provider implements OAuth authentication for MCP.
type Provider struct {
	serverURL    string
	serverHash   string
	callbackPort int
	callbackHost string
	clientName   string
	cacheDir     string

	// Static configuration (optional)
	staticClientInfo     *ClientInfo
	staticClientMetadata *ClientMetadata

	// Runtime state
	metadata   *ServerMetadata
	clientInfo *ClientInfo
	tokens     *Tokens
	pkce       *PKCE

	mu sync.RWMutex
}

// ProviderConfig holds configuration for the auth provider.
type ProviderConfig struct {
	ServerURL            string
	ServerHash           string
	CallbackPort         int
	CallbackHost         string
	ClientName           string
	CacheDir             string
	StaticClientInfo     *ClientInfo
	StaticClientMetadata *ClientMetadata
}

// NewProvider creates a new OAuth provider.
func NewProvider(cfg *ProviderConfig) *Provider {
	hashPreview := cfg.ServerHash
	if len(hashPreview) > 8 {
		hashPreview = hashPreview[:8] + "..."
	}
	logrus.WithFields(logrus.Fields{
		"server_url":                 cfg.ServerURL,
		"server_hash":                hashPreview,
		"callback_port":              cfg.CallbackPort,
		"has_static_client_info":     cfg.StaticClientInfo != nil,
		"has_static_client_metadata": cfg.StaticClientMetadata != nil,
	}).Debug("creating OAuth provider")

	return &Provider{
		serverURL:            cfg.ServerURL,
		serverHash:           cfg.ServerHash,
		callbackPort:         cfg.CallbackPort,
		callbackHost:         cfg.CallbackHost,
		clientName:           cfg.ClientName,
		cacheDir:             cfg.CacheDir,
		staticClientInfo:     cfg.StaticClientInfo,
		staticClientMetadata: cfg.StaticClientMetadata,
	}
}

// GetAccessToken returns the current access token.
func (p *Provider) GetAccessToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	tokens := p.tokens
	p.mu.RUnlock()

	if tokens == nil {
		logrus.Debug("auth: no tokens in memory, attempting to load from disk")
		// Try to load from disk
		loaded, err := LoadTokens(p.cacheDir, p.serverHash)
		if err != nil {
			logrus.WithError(err).Debug("auth: no tokens available on disk")
			return "", fmt.Errorf("no tokens available")
		}
		logrus.Debug("auth: loaded tokens from disk")
		p.mu.Lock()
		p.tokens = loaded
		tokens = loaded
		p.mu.Unlock()
	}

	if tokens.IsExpired() && tokens.RefreshToken != "" {
		logrus.Debug("auth: access token expired, attempting refresh")
		if err := p.RefreshToken(ctx); err != nil {
			logrus.WithError(err).Warn("auth: token refresh failed")
			return "", err
		}
		p.mu.RLock()
		tokens = p.tokens
		p.mu.RUnlock()
	}

	logrus.WithField("expires_at", tokens.ExpiresAt).Debug("auth: returning access token")
	return tokens.AccessToken, nil
}

// RefreshToken refreshes the access token.
func (p *Provider) RefreshToken(ctx context.Context) error {
	logrus.Debug("auth: starting token refresh")

	p.mu.RLock()
	tokens := p.tokens
	clientInfo := p.clientInfo
	metadata := p.metadata
	p.mu.RUnlock()

	if tokens == nil || tokens.RefreshToken == "" {
		logrus.Debug("auth: no refresh token available")
		return fmt.Errorf("no refresh token available")
	}

	if metadata == nil {
		logrus.Debug("auth: fetching server metadata for refresh")
		var err error
		metadata, err = FetchServerMetadata(ctx, p.serverURL)
		if err != nil {
			logrus.WithError(err).Error("auth: failed to fetch metadata for refresh")
			return err
		}
		p.mu.Lock()
		p.metadata = metadata
		p.mu.Unlock()
	}

	if clientInfo == nil {
		logrus.Debug("auth: loading client info for refresh")
		loaded, err := LoadClientInfo(p.cacheDir, p.serverHash)
		if err != nil {
			logrus.WithError(err).Error("auth: no client info available for refresh")
			return fmt.Errorf("no client info available: %w", err)
		}
		clientInfo = loaded
		p.mu.Lock()
		p.clientInfo = loaded
		p.mu.Unlock()
	}

	// Exchange refresh token
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", tokens.RefreshToken)
	data.Set("client_id", clientInfo.ClientID)
	if clientInfo.ClientSecret != "" {
		data.Set("client_secret", clientInfo.ClientSecret)
	}

	logrus.WithField("endpoint", metadata.TokenEndpoint).Debug("auth: sending refresh token request")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.TokenEndpoint,
		strings.NewReader(data.Encode()))
	if err != nil {
		logrus.WithError(err).Error("auth: failed to create refresh request")
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.WithError(err).Error("auth: refresh request failed")
		return err
	}
	defer resp.Body.Close()

	logrus.WithField("status", resp.StatusCode).Debug("auth: refresh response")
	if resp.StatusCode != http.StatusOK {
		logrus.WithField("status", resp.StatusCode).Error("auth: token refresh failed")
		return fmt.Errorf("token refresh failed: %d", resp.StatusCode)
	}

	var newTokens Tokens
	if err := json.NewDecoder(resp.Body).Decode(&newTokens); err != nil {
		logrus.WithError(err).Error("auth: failed to decode refresh response")
		return err
	}

	// Calculate expiry time
	if newTokens.ExpiresIn > 0 {
		newTokens.ExpiresAt = time.Now().Add(time.Duration(newTokens.ExpiresIn) * time.Second)
	}

	p.mu.Lock()
	p.tokens = &newTokens
	p.mu.Unlock()

	logrus.WithField("expires_at", newTokens.ExpiresAt).Info("auth: token refreshed successfully")
	return SaveTokens(p.cacheDir, p.serverHash, &newTokens)
}

// Initialise prepares the OAuth provider for authentication.
func (p *Provider) Initialise(ctx context.Context) error {
	logrus.Debug("auth: initialising provider")

	// Try to load existing tokens
	if tokens, err := LoadTokens(p.cacheDir, p.serverHash); err == nil {
		logrus.Debug("auth: found existing tokens on disk")
		p.mu.Lock()
		p.tokens = tokens
		p.mu.Unlock()

		if !tokens.IsExpired() {
			logrus.WithField("expires_at", tokens.ExpiresAt).Info("auth: using existing valid tokens")
			return nil // Already authenticated
		}

		logrus.Debug("auth: existing tokens expired, attempting refresh")
		if tokens.RefreshToken != "" {
			if err := p.RefreshToken(ctx); err == nil {
				logrus.Info("auth: refreshed existing tokens")
				return nil
			}
			logrus.Debug("auth: refresh failed, will need new authentication")
		}
	} else {
		logrus.WithError(err).Debug("auth: no existing tokens found")
	}

	// Fetch server metadata
	logrus.WithField("url", p.serverURL).Debug("auth: fetching server metadata")
	metadata, err := FetchServerMetadata(ctx, p.serverURL)
	if err != nil {
		logrus.WithError(err).Error("auth: failed to fetch server metadata")
		return fmt.Errorf("failed to fetch server metadata: %w", err)
	}
	logrus.WithFields(logrus.Fields{
		"issuer":           metadata.Issuer,
		"has_registration": metadata.RegistrationEndpoint != "",
	}).Debug("auth: server metadata retrieved")
	p.mu.Lock()
	p.metadata = metadata
	p.mu.Unlock()

	// Use static client info or load/register
	if p.staticClientInfo != nil {
		logrus.WithField("client_id", p.staticClientInfo.ClientID).Debug("auth: using static client info")
		p.mu.Lock()
		p.clientInfo = p.staticClientInfo
		p.mu.Unlock()
	} else {
		clientInfo, err := LoadClientInfo(p.cacheDir, p.serverHash)
		if err != nil {
			logrus.Debug("auth: no stored client info, attempting registration")
			// Need to register
			if metadata.RegistrationEndpoint == "" {
				logrus.Error("auth: server does not support dynamic client registration")
				return fmt.Errorf("server does not support dynamic client registration and no static client info provided")
			}
			clientInfo, err = p.registerClient(ctx, metadata)
			if err != nil {
				logrus.WithError(err).Error("auth: client registration failed")
				return fmt.Errorf("failed to register client: %w", err)
			}
			logrus.WithField("client_id", clientInfo.ClientID).Info("auth: client registered successfully")
		} else {
			logrus.WithField("client_id", clientInfo.ClientID).Debug("auth: loaded stored client info")
		}
		p.mu.Lock()
		p.clientInfo = clientInfo
		p.mu.Unlock()
	}

	logrus.Debug("auth: provider initialised")
	return nil
}

// registerClient performs dynamic client registration.
func (p *Provider) registerClient(ctx context.Context, metadata *ServerMetadata) (*ClientInfo, error) {
	redirectURI := fmt.Sprintf("http://%s:%d/callback", p.callbackHost, p.callbackPort)
	logrus.WithFields(logrus.Fields{
		"redirect_uri": redirectURI,
		"endpoint":     metadata.RegistrationEndpoint,
	}).Debug("auth: registering client")

	clientMeta := &ClientMetadata{
		RedirectURIs:            []string{redirectURI},
		TokenEndpointAuthMethod: "none",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		ClientName:              p.clientName,
	}

	// Merge with static metadata if provided
	if p.staticClientMetadata != nil {
		logrus.Debug("auth: merging static client metadata")
		if p.staticClientMetadata.Scope != "" {
			clientMeta.Scope = p.staticClientMetadata.Scope
		}
		if p.staticClientMetadata.SoftwareID != "" {
			clientMeta.SoftwareID = p.staticClientMetadata.SoftwareID
		}
		if p.staticClientMetadata.SoftwareVersion != "" {
			clientMeta.SoftwareVersion = p.staticClientMetadata.SoftwareVersion
		}
	}

	data, err := json.Marshal(clientMeta)
	if err != nil {
		logrus.WithError(err).Error("auth: failed to marshal client metadata")
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.RegistrationEndpoint,
		strings.NewReader(string(data)))
	if err != nil {
		logrus.WithError(err).Error("auth: failed to create registration request")
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	logrus.Debug("auth: sending registration request")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.WithError(err).Error("auth: registration request failed")
		return nil, err
	}
	defer resp.Body.Close()

	logrus.WithField("status", resp.StatusCode).Debug("auth: registration response")
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		logrus.WithField("status", resp.StatusCode).Error("auth: registration failed")
		return nil, fmt.Errorf("registration failed: %d", resp.StatusCode)
	}

	var clientInfo ClientInfo
	if err := json.NewDecoder(resp.Body).Decode(&clientInfo); err != nil {
		logrus.WithError(err).Error("auth: failed to decode registration response")
		return nil, err
	}

	logrus.WithField("client_id", clientInfo.ClientID).Debug("auth: saving client info")
	if err := SaveClientInfo(p.cacheDir, p.serverHash, &clientInfo); err != nil {
		logrus.WithError(err).Error("auth: failed to save client info")
		return nil, err
	}

	return &clientInfo, nil
}

// GetAuthorizationURL returns the OAuth authorisation URL.
func (p *Provider) GetAuthorizationURL(resource string) (string, error) {
	logrus.WithField("resource", resource).Debug("auth: building authorisation URL")

	p.mu.RLock()
	metadata := p.metadata
	clientInfo := p.clientInfo
	p.mu.RUnlock()

	if metadata == nil || clientInfo == nil {
		logrus.Error("auth: provider not initialised")
		return "", fmt.Errorf("provider not initialised")
	}

	// Generate PKCE
	logrus.Debug("auth: generating PKCE challenge")
	pkce, err := NewPKCE()
	if err != nil {
		logrus.WithError(err).Error("auth: failed to generate PKCE")
		return "", err
	}
	p.mu.Lock()
	p.pkce = pkce
	p.mu.Unlock()

	redirectURI := fmt.Sprintf("http://%s:%d/callback", p.callbackHost, p.callbackPort)

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientInfo.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("code_challenge", pkce.Challenge)
	params.Set("code_challenge_method", pkce.Method)

	if resource != "" {
		params.Set("resource", resource)
	}

	if clientInfo.Scope != "" {
		params.Set("scope", clientInfo.Scope)
	}

	authURL := metadata.AuthorizationEndpoint + "?" + params.Encode()
	logrus.WithField("endpoint", metadata.AuthorizationEndpoint).Debug("auth: authorisation URL built")
	return authURL, nil
}

// ExchangeCode exchanges an authorisation code for tokens.
func (p *Provider) ExchangeCode(ctx context.Context, code string) error {
	logrus.Debug("auth: exchanging authorisation code for tokens")

	p.mu.RLock()
	metadata := p.metadata
	clientInfo := p.clientInfo
	pkce := p.pkce
	p.mu.RUnlock()

	if metadata == nil || clientInfo == nil || pkce == nil {
		logrus.Error("auth: provider not initialised for code exchange")
		return fmt.Errorf("provider not initialised")
	}

	redirectURI := fmt.Sprintf("http://%s:%d/callback", p.callbackHost, p.callbackPort)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientInfo.ClientID)
	data.Set("code_verifier", pkce.Verifier)
	if clientInfo.ClientSecret != "" {
		data.Set("client_secret", clientInfo.ClientSecret)
	}

	logrus.WithField("endpoint", metadata.TokenEndpoint).Debug("auth: sending token exchange request")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.TokenEndpoint,
		strings.NewReader(data.Encode()))
	if err != nil {
		logrus.WithError(err).Error("auth: failed to create token exchange request")
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.WithError(err).Error("auth: token exchange request failed")
		return err
	}
	defer resp.Body.Close()

	logrus.WithField("status", resp.StatusCode).Debug("auth: token exchange response")
	if resp.StatusCode != http.StatusOK {
		logrus.WithField("status", resp.StatusCode).Error("auth: token exchange failed")
		return fmt.Errorf("token exchange failed: %d", resp.StatusCode)
	}

	var tokens Tokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		logrus.WithError(err).Error("auth: failed to decode token response")
		return err
	}

	if tokens.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	}

	p.mu.Lock()
	p.tokens = &tokens
	p.mu.Unlock()

	logrus.WithField("expires_at", tokens.ExpiresAt).Info("auth: tokens obtained successfully")
	return SaveTokens(p.cacheDir, p.serverHash, &tokens)
}

// HasValidTokens returns true if valid tokens are available.
func (p *Provider) HasValidTokens() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tokens != nil && !p.tokens.IsExpired()
}

// Port returns the configured callback port.
func (p *Provider) Port() int {
	return p.callbackPort
}
