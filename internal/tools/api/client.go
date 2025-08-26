package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/security"
)

// HTTPClient wraps http.Client with API-specific configuration
type HTTPClient struct {
	client   *http.Client
	apiDef   APIDefinition
	security *security.Operations
}

// NewHTTPClient creates a new HTTP client for an API
func NewHTTPClient(apiDef APIDefinition) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: time.Duration(apiDef.Timeout) * time.Second,
		},
		apiDef:   apiDef,
		security: security.NewOperations("api_tool"),
	}
}

// ExecuteRequest executes an API request
func (c *HTTPClient) ExecuteRequest(ctx context.Context, endpoint EndpointConfig, parameters map[string]any) (map[string]any, error) {
	// Build the request URL
	requestURL, err := c.buildURL(endpoint, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Security check - domain access
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if err := security.CheckDomainAccess(parsedURL.Hostname()); err != nil {
		return nil, err
	}

	// Build the request
	req, err := c.buildRequest(ctx, endpoint, requestURL, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Execute the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Security check - content analysis
	source := security.SourceContext{
		Tool:        "api_tool",
		Domain:      parsedURL.Hostname(),
		ContentType: "api_response",
	}

	if result, err := security.AnalyseContent(string(bodyBytes), source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, fmt.Errorf("content blocked by security policy [ID: %s]: %s. Check with the user if you may use security_override tool with ID %s", result.ID, result.Message, result.ID)
		case security.ActionWarn:
		}
	}

	// Check HTTP status
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var responseData any
	if len(bodyBytes) > 0 {
		// Try to parse as JSON first
		if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
			// If JSON parsing fails, return as raw text
			responseData = string(bodyBytes)
		}
	}

	result := map[string]any{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"data":        responseData,
	}

	return result, nil
}

// buildURL constructs the full request URL with path and query parameters
func (c *HTTPClient) buildURL(endpoint EndpointConfig, parameters map[string]any) (string, error) {
	// Start with base URL
	baseURL := strings.TrimSuffix(c.apiDef.BaseURL, "/")
	path := strings.TrimPrefix(endpoint.Path, "/")

	// Replace path parameters
	for _, param := range endpoint.Parameters {
		if param.Location == "path" {
			if value, exists := parameters[param.Name]; exists {
				placeholder := fmt.Sprintf("{%s}", param.Name)
				path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", value))
			}
		}
	}

	// Build query parameters
	queryParams := url.Values{}
	for _, param := range endpoint.Parameters {
		if param.Location == "query" {
			if value, exists := parameters[param.Name]; exists {
				queryParams.Add(param.Name, fmt.Sprintf("%v", value))
			}
		}
	}

	// Add auth query parameter if needed
	if c.apiDef.Auth.Type == "api_key" && c.apiDef.Auth.Location == "query" {
		credential := ResolveEnvVar(c.apiDef.Auth.EnvVar)
		if credential != "" {
			queryParams.Add(c.apiDef.Auth.Header, credential) // Header field is used as query param name
		}
	}

	// Construct final URL
	fullURL := fmt.Sprintf("%s/%s", baseURL, path)
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	return fullURL, nil
}

// buildRequest creates the HTTP request with headers, auth, and body
func (c *HTTPClient) buildRequest(ctx context.Context, endpoint EndpointConfig, requestURL string, parameters map[string]any) (*http.Request, error) {
	// Prepare request body
	var bodyReader io.Reader
	// Check if this method typically supports a body
	methodSupportsBody := endpoint.Method == "POST" || endpoint.Method == "PUT" ||
		endpoint.Method == "PATCH" || endpoint.Method == "DELETE"

	if methodSupportsBody {
		// Extract body parameters
		bodyData := make(map[string]any)
		hasBodyParams := false

		for _, param := range endpoint.Parameters {
			if param.Location == "body" {
				if value, exists := parameters[param.Name]; exists {
					bodyData[param.Name] = value
					hasBodyParams = true
				}
			}
		}

		if hasBodyParams {
			if endpoint.Body != nil && endpoint.Body.Type == "json" {
				jsonBody, err := json.Marshal(bodyData)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
				}
				bodyReader = bytes.NewReader(jsonBody)
			} else {
				// Default to JSON if no body type specified but we have body params
				jsonBody, err := json.Marshal(bodyData)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
				}
				bodyReader = bytes.NewReader(jsonBody)
			}
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("User-Agent", "mcp-devtools/1.0")

	// Set content type for JSON bodies
	if bodyReader != nil && (endpoint.Body == nil || endpoint.Body.ContentType == "") {
		req.Header.Set("Content-Type", "application/json")
	} else if endpoint.Body != nil && endpoint.Body.ContentType != "" {
		req.Header.Set("Content-Type", endpoint.Body.ContentType)
	}

	// Add API-level headers
	for key, value := range c.apiDef.Headers {
		req.Header.Set(key, value)
	}

	// Add endpoint-level headers
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	// Add parameter headers
	for _, param := range endpoint.Parameters {
		if param.Location == "header" {
			if value, exists := parameters[param.Name]; exists {
				req.Header.Set(param.Name, fmt.Sprintf("%v", value))
			}
		}
	}

	// Add authentication
	if err := c.addAuthentication(req); err != nil {
		return nil, fmt.Errorf("failed to add authentication: %w", err)
	}

	return req, nil
}

// addAuthentication adds authentication to the request
func (c *HTTPClient) addAuthentication(req *http.Request) error {
	switch c.apiDef.Auth.Type {
	case "bearer":
		credential := ResolveEnvVar(c.apiDef.Auth.EnvVar)
		if credential == "" {
			return fmt.Errorf("bearer token not found in environment variable %s", c.apiDef.Auth.EnvVar)
		}
		req.Header.Set("Authorization", "Bearer "+credential)

	case "api_key":
		credential := ResolveEnvVar(c.apiDef.Auth.EnvVar)
		if credential == "" {
			return fmt.Errorf("API key not found in environment variable %s", c.apiDef.Auth.EnvVar)
		}
		if c.apiDef.Auth.Location == "header" {
			req.Header.Set(c.apiDef.Auth.Header, credential)
		}
		// Query param auth is handled in buildURL

	case "basic":
		username := ResolveEnvVar(c.apiDef.Auth.Username)
		password := ResolveEnvVar(c.apiDef.Auth.Password)
		if username == "" || password == "" {
			return fmt.Errorf("basic auth credentials not found")
		}
		req.SetBasicAuth(username, password)

	case "none", "":
		// No authentication needed
	}

	return nil
}
