package client

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// BrowserAuthFlow manages the complete browser-based OAuth authentication flow
type BrowserAuthFlow struct {
	client OAuth2Client
	config *OAuth2ClientConfig
	logger *logrus.Logger
}

// NewBrowserAuthFlow creates a new browser authentication flow manager
func NewBrowserAuthFlow(config *OAuth2ClientConfig, logger *logrus.Logger) (*BrowserAuthFlow, error) {
	client, err := NewOAuth2Client(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth client: %w", err)
	}

	return &BrowserAuthFlow{
		client: client,
		config: config,
		logger: logger,
	}, nil
}

// Authenticate performs the complete browser-based authentication flow
func (f *BrowserAuthFlow) Authenticate(ctx context.Context) (*TokenResponse, error) {
	f.logger.Info("Starting browser-based OAuth authentication flow")
	f.logger.Infof("Please complete authentication in your browser...")

	// Start the authentication session
	session, err := f.client.StartAuthentication(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start authentication: %w", err)
	}

	// Wait for the authentication to complete
	select {
	case result := <-session.ResultCh:
		if result.Success {
			f.logger.Info("OAuth authentication completed successfully")
			return result.TokenResponse, nil
		}
		return nil, fmt.Errorf("authentication failed: %v", result.Error)

	case err := <-session.ErrorCh:
		return nil, fmt.Errorf("authentication error: %w", err)

	case <-session.Context.Done():
		return nil, fmt.Errorf("authentication timed out")

	case <-ctx.Done():
		return nil, fmt.Errorf("authentication cancelled: %w", ctx.Err())
	}
}

// AuthenticateWithTimeout performs authentication with a custom timeout
func (f *BrowserAuthFlow) AuthenticateWithTimeout(timeout time.Duration) (*TokenResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return f.Authenticate(ctx)
}

// AuthenticateIfRequired performs authentication only if required by configuration
func (f *BrowserAuthFlow) AuthenticateIfRequired(ctx context.Context, required bool) (*TokenResponse, error) {
	if !required {
		f.logger.Debug("Browser authentication not required, skipping")
		return nil, nil
	}

	return f.Authenticate(ctx)
}

// ValidateConfig validates the authentication flow configuration
func (f *BrowserAuthFlow) ValidateConfig() error {
	return f.client.ValidateConfiguration()
}

// GetRedirectURI returns the redirect URI that would be used for authentication
func (f *BrowserAuthFlow) GetRedirectURI() string {
	// Create a temporary callback server to get the redirect URI
	callbackServer := NewCallbackServer(f.logger)

	// Start it briefly to get the URI
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := callbackServer.Start(ctx, f.config.ServerPort); err == nil {
		uri := callbackServer.GetRedirectURI()
		_ = callbackServer.Stop() // Error already logged in Stop()
		return uri
	}

	// Fallback to default format
	port := f.config.ServerPort
	if port == 0 {
		port = 8080 // Default port
	}
	return fmt.Sprintf("http://127.0.0.1:%d/callback", port)
}
