package confluence

import "time"

// ConfluenceConfig holds configuration for Confluence API access
type ConfluenceConfig struct {
	BaseURL  string
	Username string
	Token    string

	// OAuth 2.0 configuration (alternative to username/token)
	UseOAuth          bool   `json:"use_oauth"`
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
	OAuthIssuerURL    string `json:"oauth_issuer_url"`
	OAuthScope        string `json:"oauth_scope"`
	OAuthTokenFile    string `json:"oauth_token_file"` // File to cache OAuth tokens

	// Session cookie authentication (for SAML/SSO environments)
	UseSessionCookies bool   `json:"use_session_cookies"`
	SessionCookies    string `json:"session_cookies"` // Raw cookie string from browser
	BrowserType       string `json:"browser_type"`    // Browser to extract cookies from
}

// SearchRequest represents a Confluence search request
type SearchRequest struct {
	Query        string   `json:"query"`
	SpaceKey     string   `json:"space_key,omitempty"`
	MaxResults   int      `json:"max_results"`
	ContentTypes []string `json:"content_types,omitempty"`
}

// SearchResponse represents the complete response from a Confluence search
type SearchResponse struct {
	Query      string          `json:"query"`
	Results    []ContentResult `json:"results"`
	TotalCount int             `json:"total_count"`
	Message    string          `json:"message,omitempty"`
}

// ContentResult represents a single piece of content from Confluence
type ContentResult struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Title          string                 `json:"title"`
	Space          SpaceInfo              `json:"space"`
	URL            string                 `json:"url"`
	WebURL         string                 `json:"web_url"`
	LastModified   time.Time              `json:"last_modified"`
	Author         Author                 `json:"author"`
	Content        string                 `json:"content"`
	ContentPreview string                 `json:"content_preview"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// SpaceInfo represents Confluence space information
type SpaceInfo struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Author represents content author information
type Author struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
}

// APISearchResponse represents the raw Confluence API search response
type APISearchResponse struct {
	Results []APIContent `json:"results"`
	Start   int          `json:"start"`
	Limit   int          `json:"limit"`
	Size    int          `json:"size"`
	Links   struct {
		Self string `json:"self"`
		Next string `json:"next,omitempty"`
	} `json:"_links"`
}

// APIContent represents raw content from Confluence API
type APIContent struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Status     string            `json:"status"`
	Title      string            `json:"title"`
	Space      SpaceInfo         `json:"space"`
	History    History           `json:"history"`
	Version    Version           `json:"version"`
	Ancestors  []APIContent      `json:"ancestors,omitempty"`
	Body       Body              `json:"body,omitempty"`
	Links      Links             `json:"_links"`
	Expandable map[string]string `json:"_expandable,omitempty"`
}

// History represents content history information
type History struct {
	Latest      bool        `json:"latest"`
	CreatedBy   Author      `json:"createdBy"`
	CreatedDate string      `json:"createdDate"`
	LastUpdated LastUpdated `json:"lastUpdated"`
}

// LastUpdated represents last update information
type LastUpdated struct {
	By        Author `json:"by"`
	When      string `json:"when"`
	Message   string `json:"message,omitempty"`
	Number    int    `json:"number"`
	MinorEdit bool   `json:"minorEdit"`
}

// Version represents content version information
type Version struct {
	By        Author `json:"by"`
	When      string `json:"when"`
	Message   string `json:"message,omitempty"`
	Number    int    `json:"number"`
	MinorEdit bool   `json:"minorEdit"`
}

// Body represents the content body with different representations
type Body struct {
	Storage Storage `json:"storage,omitempty"`
	View    View    `json:"view,omitempty"`
}

// Storage represents the storage format (XHTML) of content
type Storage struct {
	Value          string            `json:"value"`
	Representation string            `json:"representation"`
	Embeddable     map[string]string `json:"_embeddable,omitempty"`
}

// View represents the view format (HTML) of content
type View struct {
	Value          string            `json:"value"`
	Representation string            `json:"representation"`
	Embeddable     map[string]string `json:"_embeddable,omitempty"`
}

// Links represents API links for content
type Links struct {
	Self    string `json:"self"`
	WebUI   string `json:"webui"`
	Edit    string `json:"edit,omitempty"`
	TinyUI  string `json:"tinyui,omitempty"`
	Context string `json:"context,omitempty"`
}

// ErrorResponse represents an error response from Confluence API
type ErrorResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Reason     string `json:"reason,omitempty"`
}

// OAuthTokenCache represents cached OAuth tokens
type OAuthTokenCache struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	CachedAt     time.Time `json:"cached_at"`
}

// IsExpired checks if the OAuth token is expired
func (t *OAuthTokenCache) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second)) // 30 second buffer
}

// IsValid checks if the OAuth token is valid and not expired
func (t *OAuthTokenCache) IsValid() bool {
	return t.AccessToken != "" && !t.IsExpired()
}
