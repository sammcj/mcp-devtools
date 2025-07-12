package validation

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key type
	Use string `json:"use"` // Key use
	Kid string `json:"kid"` // Key ID
	Alg string `json:"alg"` // Algorithm
	N   string `json:"n"`   // RSA modulus
	E   string `json:"e"`   // RSA exponent
}

// JWKSClient handles fetching and caching JWKS
type JWKSClient struct {
	jwksURL    string
	logger     *logrus.Logger
	httpClient *http.Client
	cache      *jwksCache
}

// jwksCache implements a simple cache for JWKS with TTL
type jwksCache struct {
	mutex     sync.RWMutex
	jwks      *JWKS
	expiresAt time.Time
	ttl       time.Duration
}

// NewJWKSClient creates a new JWKS client
func NewJWKSClient(jwksURL string, logger *logrus.Logger) (*JWKSClient, error) {
	if jwksURL == "" {
		return nil, fmt.Errorf("JWKS URL is required")
	}

	return &JWKSClient{
		jwksURL: jwksURL,
		logger:  logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: &jwksCache{
			ttl: 5 * time.Minute, // Cache JWKS for 5 minutes
		},
	}, nil
}

// GetKey retrieves a specific key by ID from the JWKS
func (c *JWKSClient) GetKey(ctx context.Context, keyID string) (interface{}, error) {
	jwks, err := c.getJWKS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	for _, key := range jwks.Keys {
		if key.Kid == keyID {
			return c.convertJWKToRSAPublicKey(&key)
		}
	}

	return nil, fmt.Errorf("key not found: %s", keyID)
}

// GetJWKS returns the full JWKS
func (c *JWKSClient) GetJWKS(ctx context.Context) (*JWKS, error) {
	return c.getJWKS(ctx)
}

// getJWKS fetches JWKS from the URL with caching
func (c *JWKSClient) getJWKS(ctx context.Context) (*JWKS, error) {
	c.cache.mutex.RLock()
	if c.cache.jwks != nil && time.Now().Before(c.cache.expiresAt) {
		jwks := c.cache.jwks
		c.cache.mutex.RUnlock()
		c.logger.Debug("Returning cached JWKS")
		return jwks, nil
	}
	c.cache.mutex.RUnlock()

	c.logger.Debug("Fetching JWKS from URL")
	
	req, err := http.NewRequestWithContext(ctx, "GET", c.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mcp-devtools OAuth2 client")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Cache the result
	c.cache.mutex.Lock()
	c.cache.jwks = &jwks
	c.cache.expiresAt = time.Now().Add(c.cache.ttl)
	c.cache.mutex.Unlock()

	c.logger.WithField("key_count", len(jwks.Keys)).Debug("Successfully fetched and cached JWKS")
	return &jwks, nil
}

// convertJWKToRSAPublicKey converts a JWK to an RSA public key
func (c *JWKSClient) convertJWKToRSAPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}

	// Decode modulus
	nBytes, err := base64URLDecode(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode exponent
	eBytes, err := base64URLDecode(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	// Create RSA public key
	pubKey := &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}

	return pubKey, nil
}

// InvalidateCache invalidates the JWKS cache
func (c *JWKSClient) InvalidateCache() {
	c.cache.mutex.Lock()
	defer c.cache.mutex.Unlock()
	
	c.cache.jwks = nil
	c.cache.expiresAt = time.Time{}
	c.logger.Debug("JWKS cache invalidated")
}