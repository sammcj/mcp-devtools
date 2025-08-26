package terraform_documentation

import "time"

// Provider-related structures

// ProviderDocs represents the response from the v1 providers API
type ProviderDocs struct {
	ID          string        `json:"id"`
	Owner       string        `json:"owner"`
	Namespace   string        `json:"namespace"`
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Tag         string        `json:"tag"`
	Description string        `json:"description"`
	Source      string        `json:"source"`
	Published   time.Time     `json:"published_at"`
	Downloads   int           `json:"downloads"`
	Verified    bool          `json:"verified"`
	Docs        []ProviderDoc `json:"docs"`
}

// ProviderDoc represents a single documentation item
type ProviderDoc struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	Slug        string `json:"slug"`
	Category    string `json:"category"`
	Language    string `json:"language"`
	Subcategory string `json:"subcategory"`
}

// ProviderVersionsResponse represents provider versions from v1 API
type ProviderVersionsResponse struct {
	Versions []ProviderVersion `json:"versions"`
}

// ProviderVersion represents a single provider version
type ProviderVersion struct {
	Version     string     `json:"version"`
	Protocols   []string   `json:"protocols"`
	Platforms   []Platform `json:"platforms"`
	PublishedAt time.Time  `json:"published_at"`
}

// Platform represents a platform for a provider version
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// ProviderVersionsV2Response represents provider versions from v2 API
type ProviderVersionsV2Response struct {
	Data []ProviderVersionV2 `json:"data"`
	Meta ResponseMeta        `json:"meta"`
}

// ProviderVersionV2 represents a provider version from v2 API
type ProviderVersionV2 struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes ProviderVersionAttributes `json:"attributes"`
}

// ProviderVersionAttributes represents attributes of a provider version
type ProviderVersionAttributes struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"published_at"`
}

// ProviderDocsV2Response represents provider docs from v2 API
type ProviderDocsV2Response struct {
	Data []ProviderDocV2 `json:"data"`
	Meta ResponseMeta    `json:"meta"`
}

// ProviderDocV2 represents a provider document from v2 API
type ProviderDocV2 struct {
	ID         string                `json:"id"`
	Type       string                `json:"type"`
	Attributes ProviderDocAttributes `json:"attributes"`
}

// ProviderDocAttributes represents attributes of a provider document
type ProviderDocAttributes struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Language    string `json:"language"`
	Subcategory string `json:"subcategory"`
	Path        string `json:"path"`
	Slug        string `json:"slug"`
}

// ProviderResourceDetails represents detailed provider resource documentation
type ProviderResourceDetails struct {
	Data ProviderResourceData `json:"data"`
}

// ProviderResourceData represents the data section of provider resource details
type ProviderResourceData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes ProviderResourceAttributes `json:"attributes"`
}

// ProviderResourceAttributes represents the attributes of provider resource details
type ProviderResourceAttributes struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Language    string `json:"language"`
	Subcategory string `json:"subcategory"`
	Path        string `json:"path"`
	Slug        string `json:"slug"`
	Content     string `json:"content"`
}

// ProviderOverviewResponse represents provider overview from v2 API
type ProviderOverviewResponse struct {
	Data ProviderOverviewData `json:"data"`
}

// ProviderOverviewData represents the data section of provider overview
type ProviderOverviewData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes ProviderOverviewAttributes `json:"attributes"`
}

// ProviderOverviewAttributes represents the attributes of provider overview
type ProviderOverviewAttributes struct {
	Description string `json:"description"`
	Source      string `json:"source"`
	Version     string `json:"version"`
}

// Module-related structures

// ModuleSearchResponse represents the response from module search
type ModuleSearchResponse struct {
	Modules []Module     `json:"modules"`
	Meta    ResponseMeta `json:"meta"`
}

// Module represents a Terraform module
type Module struct {
	ID          string    `json:"id"`
	Owner       string    `json:"owner"`
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Provider    string    `json:"provider"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	Tag         string    `json:"tag"`
	PublishedAt time.Time `json:"published_at"`
	Downloads   int       `json:"downloads"`
	Verified    bool      `json:"verified"`
}

// ModuleDetails represents detailed module information
type ModuleDetails struct {
	ID           string             `json:"id"`
	Owner        string             `json:"owner"`
	Namespace    string             `json:"namespace"`
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	Provider     string             `json:"provider"`
	Description  string             `json:"description"`
	Source       string             `json:"source"`
	Tag          string             `json:"tag"`
	PublishedAt  time.Time          `json:"published_at"`
	Downloads    int                `json:"downloads"`
	Verified     bool               `json:"verified"`
	Inputs       []ModuleInput      `json:"inputs"`
	Outputs      []ModuleOutput     `json:"outputs"`
	Dependencies []ModuleDependency `json:"dependencies"`
	Resources    []ModuleResource   `json:"resources"`
}

// ModuleInput represents a module input variable
type ModuleInput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     any    `json:"default"`
	Required    bool   `json:"required"`
}

// ModuleOutput represents a module output
type ModuleOutput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ModuleDependency represents a module dependency
type ModuleDependency struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version"`
}

// ModuleResource represents a resource used by the module
type ModuleResource struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Policy-related structures

// PolicySearchResponse represents the response from policy search
type PolicySearchResponse struct {
	Policies []Policy     `json:"policies"`
	Meta     ResponseMeta `json:"meta"`
}

// Policy represents a Terraform policy
type Policy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Downloads   int       `json:"downloads"`
	PublishedAt time.Time `json:"published_at"`
}

// PolicyDetails represents detailed policy information
type PolicyDetails struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Downloads   int       `json:"downloads"`
	PublishedAt time.Time `json:"published_at"`
	Content     string    `json:"content"`
	Readme      string    `json:"readme"`
}

// Common structures

// ResponseMeta represents common metadata in API responses
type ResponseMeta struct {
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination represents pagination information
type Pagination struct {
	CurrentPage int `json:"current_page"`
	NextPage    int `json:"next_page"`
	PrevPage    int `json:"prev_page"`
	TotalPages  int `json:"total_pages"`
	TotalCount  int `json:"total_count"`
}
