package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// APIConfig represents the complete configuration for all APIs
type APIConfig struct {
	APIs map[string]APIDefinition `yaml:"apis"`
}

// APIDefinition defines a single API configuration
type APIDefinition struct {
	BaseURL     string            `yaml:"base_url"`
	Description string            `yaml:"description"`
	Auth        AuthConfig        `yaml:"auth"`
	Timeout     int               `yaml:"timeout"`   // timeout in seconds, default 30
	CacheTTL    int               `yaml:"cache_ttl"` // cache TTL in seconds, default 300
	Endpoints   []EndpointConfig  `yaml:"endpoints"`
	Headers     map[string]string `yaml:"headers"` // additional headers to send with all requests
}

// AuthConfig defines authentication configuration
type AuthConfig struct {
	Type     string `yaml:"type"`     // "bearer", "api_key", "basic", "none"
	EnvVar   string `yaml:"env_var"`  // environment variable containing the credential
	Header   string `yaml:"header"`   // custom header name for API key auth (default: "X-API-Key")
	Location string `yaml:"location"` // "header" or "query" for API key auth (default: "header")
	Username string `yaml:"username"` // username for basic auth (can be env var reference)
	Password string `yaml:"password"` // password env var for basic auth
}

// EndpointConfig defines a single API endpoint
type EndpointConfig struct {
	Name        string            `yaml:"name"`
	Method      string            `yaml:"method"`
	Path        string            `yaml:"path"`
	Description string            `yaml:"description"`
	Parameters  []ParameterConfig `yaml:"parameters"`
	Body        *BodyConfig       `yaml:"body"`
	Headers     map[string]string `yaml:"headers"` // endpoint-specific headers
}

// ParameterConfig defines a parameter for an endpoint
type ParameterConfig struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"` // "string", "number", "boolean", "array", "object"
	Required    bool     `yaml:"required"`
	Description string   `yaml:"description"`
	Default     any      `yaml:"default"`
	Enum        []string `yaml:"enum"`
	Location    string   `yaml:"location"` // "path", "query", "header"
}

// BodyConfig defines request body configuration
type BodyConfig struct {
	Type        string         `yaml:"type"`         // "json", "form", "raw"
	ContentType string         `yaml:"content_type"` // override content type
	Schema      map[string]any `yaml:"schema"`       // JSON schema for body validation
}

// LoadAPIConfig loads the API configuration from the specified file
func LoadAPIConfig(configPath string) (*APIConfig, error) {
	// Expand home directory
	if strings.HasPrefix(configPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, configPath[1:])
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Log missing config file
		logrus.WithField("config_path", configPath).Info("API configuration file not found, using empty config")
		// Return empty config if file doesn't exist (not an error)
		return &APIConfig{APIs: make(map[string]APIDefinition)}, nil
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML
	var config APIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Validate and set defaults
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// validate validates the configuration and sets defaults
func (c *APIConfig) validate() error {
	if c.APIs == nil {
		c.APIs = make(map[string]APIDefinition)
		return nil
	}

	for apiName, apiDef := range c.APIs {
		// Validate required fields
		if apiDef.BaseURL == "" {
			return fmt.Errorf("API '%s': base_url is required", apiName)
		}

		// Set defaults
		if apiDef.Timeout == 0 {
			apiDef.Timeout = 30
		}
		if apiDef.CacheTTL == 0 {
			apiDef.CacheTTL = 300
		}

		// Validate auth configuration
		if err := apiDef.Auth.validate(); err != nil {
			return fmt.Errorf("API '%s': %w", apiName, err)
		}

		// Validate endpoints
		for i, endpoint := range apiDef.Endpoints {
			if err := endpoint.validate(); err != nil {
				return fmt.Errorf("API '%s', endpoint %d: %w", apiName, i, err)
			}
		}

		// Update the config with defaults
		c.APIs[apiName] = apiDef
	}

	return nil
}

// validate validates the auth configuration
func (a *AuthConfig) validate() error {
	validAuthTypes := map[string]bool{
		"bearer":  true,
		"api_key": true,
		"basic":   true,
		"none":    true,
		"":        true, // empty means none
	}

	if !validAuthTypes[a.Type] {
		return fmt.Errorf("invalid auth type '%s', must be one of: bearer, api_key, basic, none", a.Type)
	}

	// Set defaults for api_key auth
	if a.Type == "api_key" {
		if a.Header == "" {
			a.Header = "X-API-Key"
		}
		if a.Location == "" {
			a.Location = "header"
		}
		if a.Location != "header" && a.Location != "query" {
			return fmt.Errorf("api_key auth location must be 'header' or 'query', got '%s'", a.Location)
		}
	}

	// Validate that required env vars are specified
	if a.Type == "bearer" || a.Type == "api_key" {
		if a.EnvVar == "" {
			return fmt.Errorf("env_var is required for %s auth", a.Type)
		}
	}

	if a.Type == "basic" {
		if a.Username == "" || a.Password == "" {
			return fmt.Errorf("username and password are required for basic auth")
		}
	}

	return nil
}

// validate validates the endpoint configuration
func (e *EndpointConfig) validate() error {
	if e.Name == "" {
		return fmt.Errorf("endpoint name is required")
	}

	if e.Method == "" {
		return fmt.Errorf("endpoint method is required")
	}

	if e.Path == "" {
		return fmt.Errorf("endpoint path is required")
	}

	// Validate method
	validMethods := map[string]bool{
		"GET":     true,
		"POST":    true,
		"PUT":     true,
		"PATCH":   true,
		"DELETE":  true,
		"HEAD":    true,
		"OPTIONS": true,
	}

	e.Method = strings.ToUpper(e.Method)
	if !validMethods[e.Method] {
		return fmt.Errorf("invalid HTTP method '%s'", e.Method)
	}

	// Validate parameters
	for i, param := range e.Parameters {
		if err := param.validate(); err != nil {
			return fmt.Errorf("parameter %d: %w", i, err)
		}
	}

	return nil
}

// validate validates the parameter configuration
func (p *ParameterConfig) validate() error {
	if p.Name == "" {
		return fmt.Errorf("parameter name is required")
	}

	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	if p.Type == "" {
		p.Type = "string" // default to string
	}

	if !validTypes[p.Type] {
		return fmt.Errorf("invalid parameter type '%s'", p.Type)
	}

	validLocations := map[string]bool{
		"path":   true,
		"query":  true,
		"header": true,
		"body":   true,
	}

	if p.Location == "" {
		p.Location = "query" // default to query
	}

	if !validLocations[p.Location] {
		return fmt.Errorf("invalid parameter location '%s'", p.Location)
	}

	return nil
}

// ResolveEnvVar resolves an environment variable reference
// For auth configs, the value should be the environment variable name
// For other configs that start with $, it's treated as an env var reference
func ResolveEnvVar(value string) string {
	if after, ok := strings.CutPrefix(value, "$"); ok {
		envVar := after
		if envValue := os.Getenv(envVar); envValue != "" {
			return envValue
		}
		// Return empty string for missing env vars to ensure auth fails clearly
		// rather than using literal strings like "$API_TOKEN"
		return ""
	}
	// For auth configs (env_var field), treat the value as an env var name directly
	if envValue := os.Getenv(value); envValue != "" {
		return envValue
	}
	// Return empty string for missing env vars to ensure auth fails clearly
	// rather than using literal strings like "API_TOKEN"
	return ""
}
