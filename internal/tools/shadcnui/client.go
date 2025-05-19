package shadcnui

import (
	"net/http"
	"time"
)

const (
	ShadcnDocsURL        = "https://ui.shadcn.com"
	ShadcnDocsComponents = ShadcnDocsURL + "/docs/components"
	ShadcnGitHubURL      = "https://github.com/shadcn-ui/ui"
	ShadcnRawGitHubURL   = "https://raw.githubusercontent.com/shadcn-ui/ui/main"
)

// HTTPClient defines the interface for an HTTP client.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// DefaultHTTPClient is the default HTTP client implementation.
var DefaultHTTPClient HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// You can add shared utility functions here for making requests and scraping if needed.
// For now, we'll rely on goquery within specific tool files.
