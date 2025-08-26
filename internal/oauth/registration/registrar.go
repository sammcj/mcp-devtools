package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sammcj/mcp-devtools/internal/oauth/types"
	"github.com/sammcj/mcp-devtools/internal/oauth/validation"
	"github.com/sirupsen/logrus"
)

// InMemoryRegistrar implements dynamic client registration using in-memory storage
type InMemoryRegistrar struct {
	logger  *logrus.Logger
	clients map[string]*types.DynamicClientRegistrationResponse
	mutex   sync.RWMutex
	config  *RegistrarConfig
}

// RegistrarConfig contains configuration for the client registrar
type RegistrarConfig struct {
	AllowedRedirectSchemes []string      `json:"allowed_redirect_schemes"`
	RequireRedirectURIs    bool          `json:"require_redirect_uris"`
	DefaultGrantTypes      []string      `json:"default_grant_types"`
	DefaultResponseTypes   []string      `json:"default_response_types"`
	DefaultScope           string        `json:"default_scope"`
	ClientSecretTTL        time.Duration `json:"client_secret_ttl"`
	MaxRedirectURIs        int           `json:"max_redirect_uris"`
}

// NewInMemoryRegistrar creates a new in-memory client registrar
func NewInMemoryRegistrar(logger *logrus.Logger) *InMemoryRegistrar {
	return &InMemoryRegistrar{
		logger:  logger,
		clients: make(map[string]*types.DynamicClientRegistrationResponse),
		config: &RegistrarConfig{
			AllowedRedirectSchemes: []string{"https", "http"}, // http only for localhost
			RequireRedirectURIs:    true,
			DefaultGrantTypes:      []string{"authorization_code"},
			DefaultResponseTypes:   []string{"code"},
			DefaultScope:           "openid profile",
			ClientSecretTTL:        24 * time.Hour, // 24 hours
			MaxRedirectURIs:        5,
		},
	}
}

// RegisterClient registers a new OAuth client (RFC7591)
func (r *InMemoryRegistrar) RegisterClient(ctx context.Context, req *types.DynamicClientRegistrationRequest) (*types.DynamicClientRegistrationResponse, error) {
	// Validate the registration request
	if err := r.validateRegistrationRequest(req); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Generate client credentials
	clientID, err := validation.GenerateClientID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate client ID: %w", err)
	}

	// Generate client secret if needed (confidential client)
	var clientSecret string
	var secretExpiresAt int64
	authMethod := req.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "client_secret_basic" // Default
	}

	if authMethod != "none" {
		clientSecret, err = validation.GenerateClientSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate client secret: %w", err)
		}
		secretExpiresAt = time.Now().Add(r.config.ClientSecretTTL).Unix()
	}

	// Set defaults
	grantTypes := req.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = r.config.DefaultGrantTypes
	}

	responseTypes := req.ResponseTypes
	if len(responseTypes) == 0 {
		responseTypes = r.config.DefaultResponseTypes
	}

	scope := req.Scope
	if scope == "" {
		scope = r.config.DefaultScope
	}

	// Create client registration response
	response := &types.DynamicClientRegistrationResponse{
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		ClientIDIssuedAt:        time.Now().Unix(),
		ClientSecretExpiresAt:   secretExpiresAt,
		RedirectURIs:            req.RedirectURIs,
		TokenEndpointAuthMethod: authMethod,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		ClientName:              req.ClientName,
		ClientURI:               req.ClientURI,
		LogoURI:                 req.LogoURI,
		Scope:                   scope,
		Contacts:                req.Contacts,
		TosURI:                  req.TosURI,
		PolicyURI:               req.PolicyURI,
		JWKSUri:                 req.JWKSUri,
		SoftwareID:              req.SoftwareID,
		SoftwareVersion:         req.SoftwareVersion,
	}

	// Store the client
	r.mutex.Lock()
	r.clients[clientID] = response
	r.mutex.Unlock()

	r.logger.WithFields(logrus.Fields{
		"client_id":   clientID,
		"client_name": req.ClientName,
		"grant_types": grantTypes,
	}).Info("Registered new OAuth client")

	return response, nil
}

// GetClient retrieves a registered client by ID
func (r *InMemoryRegistrar) GetClient(ctx context.Context, clientID string) (*types.DynamicClientRegistrationResponse, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	client, exists := r.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client not found: %s", clientID)
	}

	// Check if client secret has expired
	if client.ClientSecretExpiresAt > 0 && time.Now().Unix() > client.ClientSecretExpiresAt {
		r.logger.WithField("client_id", clientID).Warn("Client secret has expired")
		// Note: In a production implementation, you might want to handle this differently
	}

	return client, nil
}

// UpdateClient updates an existing client registration
func (r *InMemoryRegistrar) UpdateClient(ctx context.Context, clientID string, req *types.DynamicClientRegistrationRequest) (*types.DynamicClientRegistrationResponse, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	client, exists := r.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client not found: %s", clientID)
	}

	// Validate the update request
	if err := r.validateRegistrationRequest(req); err != nil {
		return nil, fmt.Errorf("invalid update request: %w", err)
	}

	// Update mutable fields
	if req.RedirectURIs != nil {
		client.RedirectURIs = req.RedirectURIs
	}
	if req.ClientName != "" {
		client.ClientName = req.ClientName
	}
	if req.ClientURI != "" {
		client.ClientURI = req.ClientURI
	}
	if req.LogoURI != "" {
		client.LogoURI = req.LogoURI
	}
	if req.Scope != "" {
		client.Scope = req.Scope
	}
	if req.Contacts != nil {
		client.Contacts = req.Contacts
	}
	if req.TosURI != "" {
		client.TosURI = req.TosURI
	}
	if req.PolicyURI != "" {
		client.PolicyURI = req.PolicyURI
	}
	if req.JWKSUri != "" {
		client.JWKSUri = req.JWKSUri
	}

	r.logger.WithField("client_id", clientID).Info("Updated OAuth client")
	return client, nil
}

// DeleteClient deletes a registered client
func (r *InMemoryRegistrar) DeleteClient(ctx context.Context, clientID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.clients[clientID]; !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	delete(r.clients, clientID)
	r.logger.WithField("client_id", clientID).Info("Deleted OAuth client")
	return nil
}

// validateRegistrationRequest validates a client registration request
func (r *InMemoryRegistrar) validateRegistrationRequest(req *types.DynamicClientRegistrationRequest) error {
	// Validate redirect URIs
	if r.config.RequireRedirectURIs && len(req.RedirectURIs) == 0 {
		return fmt.Errorf("redirect_uris are required")
	}

	if len(req.RedirectURIs) > r.config.MaxRedirectURIs {
		return fmt.Errorf("too many redirect URIs (max: %d)", r.config.MaxRedirectURIs)
	}

	for _, redirectURI := range req.RedirectURIs {
		if err := r.validateRedirectURI(redirectURI); err != nil {
			return fmt.Errorf("invalid redirect URI %s: %w", redirectURI, err)
		}
	}

	// Validate grant types
	for _, grantType := range req.GrantTypes {
		if !r.isValidGrantType(grantType) {
			return fmt.Errorf("unsupported grant type: %s", grantType)
		}
	}

	// Validate response types
	for _, responseType := range req.ResponseTypes {
		if !r.isValidResponseType(responseType) {
			return fmt.Errorf("unsupported response type: %s", responseType)
		}
	}

	// Validate token endpoint auth method
	if req.TokenEndpointAuthMethod != "" && !r.isValidAuthMethod(req.TokenEndpointAuthMethod) {
		return fmt.Errorf("unsupported token endpoint auth method: %s", req.TokenEndpointAuthMethod)
	}

	return nil
}

// validateRedirectURI validates a redirect URI
func (r *InMemoryRegistrar) validateRedirectURI(redirectURI string) error {
	parsedURL, err := url.Parse(redirectURI)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check scheme
	scheme := strings.ToLower(parsedURL.Scheme)
	allowed := slices.Contains(r.config.AllowedRedirectSchemes, scheme)
	if !allowed {
		return fmt.Errorf("unsupported scheme: %s", scheme)
	}

	// For HTTP, only allow localhost
	if scheme == "http" {
		host := strings.ToLower(parsedURL.Hostname())
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return fmt.Errorf("HTTP redirect URIs only allowed for localhost")
		}
	}

	// Reject fragment
	if parsedURL.Fragment != "" {
		return fmt.Errorf("redirect URI must not contain fragment")
	}

	return nil
}

// isValidGrantType checks if a grant type is supported
func (r *InMemoryRegistrar) isValidGrantType(grantType string) bool {
	validGrantTypes := []string{
		"authorization_code",
		"refresh_token",
	}

	return slices.Contains(validGrantTypes, grantType)
}

// isValidResponseType checks if a response type is supported
func (r *InMemoryRegistrar) isValidResponseType(responseType string) bool {
	validResponseTypes := []string{
		"code",
	}

	return slices.Contains(validResponseTypes, responseType)
}

// isValidAuthMethod checks if a token endpoint auth method is supported
func (r *InMemoryRegistrar) isValidAuthMethod(authMethod string) bool {
	validAuthMethods := []string{
		"client_secret_basic",
		"client_secret_post",
		"none", // For public clients
	}

	return slices.Contains(validAuthMethods, authMethod)
}

// ServeRegistration handles HTTP requests for client registration
func (r *InMemoryRegistrar) ServeRegistration(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		r.handleRegisterClient(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRegisterClient handles POST requests to register a new client
func (r *InMemoryRegistrar) handleRegisterClient(w http.ResponseWriter, req *http.Request) {
	var regReq types.DynamicClientRegistrationRequest
	if err := json.NewDecoder(req.Body).Decode(&regReq); err != nil {
		oauth2Err := types.ErrInvalidRequest
		oauth2Err.ErrorDescription = "Invalid JSON in request body"
		oauth2Err.WriteHTTPResponse(w, http.StatusBadRequest)
		return
	}

	response, err := r.RegisterClient(req.Context(), &regReq)
	if err != nil {
		r.logger.WithError(err).Warn("Client registration failed")
		oauth2Err := types.ErrInvalidRequest
		oauth2Err.ErrorDescription = err.Error()
		oauth2Err.WriteHTTPResponse(w, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		r.logger.WithError(err).Error("Failed to encode registration response")
		return
	}
}
