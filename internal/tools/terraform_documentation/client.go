package terraform_documentation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/utils/httpclient"
	"github.com/sirupsen/logrus"
)

const (
	terraformRegistryBaseURL = "https://registry.terraform.io"
	terraformRegistryAPIv1   = "https://registry.terraform.io/v1"
	terraformRegistryAPIv2   = "https://registry.terraform.io/v2"
	defaultTimeout           = 30 * time.Second
	maxContentLength         = 1024 * 1024 // 1MB limit
	userAgent                = "mcp-devtools-terraform-documentation/1.0"
)

// Client handles communication with the Terraform Registry API
type Client struct {
	httpClient *http.Client
	logger     *logrus.Logger
	ops        *security.Operations
}

// NewClient creates a new Terraform Registry API client with proxy support
func NewClient(logger *logrus.Logger) *Client {
	return &Client{
		httpClient: httpclient.NewHTTPClientWithProxyAndLogger(defaultTimeout, logger),
		logger:     logger,
		ops:        security.NewOperations("terraform_documentation"),
	}
}

// makeRequest performs HTTP request with security checks
func (c *Client) makeRequest(ctx context.Context, url string) ([]byte, error) {
	// Use security operations for HTTP GET
	safeResp, err := c.ops.SafeHTTPGet(ctx, url)
	if err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, security.FormatSecurityBlockError(secErr)
		}
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check for security warnings
	if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
		c.logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
	}

	return safeResp.Content, nil
}

// SearchProviders searches for provider documentation
func (c *Client) SearchProviders(ctx context.Context, providerName, providerNamespace, serviceSlug, providerDataType, providerVersion string) (*mcp.CallToolResult, error) {
	c.logger.Infof("Searching providers: %s/%s, service: %s, type: %s, version: %s",
		providerNamespace, providerName, serviceSlug, providerDataType, providerVersion)

	providerName = strings.ToLower(providerName)
	providerNamespace = strings.ToLower(providerNamespace)
	serviceSlug = strings.ToLower(serviceSlug)

	// Get latest version if not specified
	if providerVersion == "" || providerVersion == "latest" {
		latestVersion, err := c.getLatestProviderVersionInternal(ctx, providerNamespace, providerName)
		if err != nil {
			return nil, fmt.Errorf("getting latest provider version: %w", err)
		}
		providerVersion = latestVersion
	}

	// Check if we need to use v2 API for guides, functions, or overview
	if isV2ProviderDataType(providerDataType) {
		content, err := c.getProviderDetailsV2(ctx, providerNamespace, providerName, providerVersion, providerDataType)
		if err != nil {
			return nil, fmt.Errorf("getting provider details from v2 API: %w", err)
		}
		fullContent := fmt.Sprintf("# %s provider docs\n\n%s", providerName, content)
		result := map[string]any{
			"content": fullContent,
		}
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}

	// For resources/data-sources, use the v1 API
	apiURL := fmt.Sprintf("%s/providers/%s/%s/%s", terraformRegistryAPIv1, providerNamespace, providerName, providerVersion)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching provider documentation: %w", err)
	}

	var providerDocs ProviderDocs
	if err := json.Unmarshal(response, &providerDocs); err != nil {
		return nil, fmt.Errorf("unmarshalling provider docs: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Available Documentation (top matches) for %s in Terraform provider %s/%s version: %s\n\n",
		providerDataType, providerNamespace, providerName, providerVersion))
	builder.WriteString("Each result includes:\n- providerDocID: tfprovider-compatible identifier\n- Title: Service or resource name\n- Category: Type of document\n- Description: Brief summary of the document\n")
	builder.WriteString("For best results, select libraries based on the service_slug match and category of information requested.\n\n---\n\n")

	contentAvailable := false
	for _, doc := range providerDocs.Docs {
		if doc.Language == "hcl" && doc.Category == providerDataType {
			if containsSlug(doc.Slug, serviceSlug) || containsSlug(fmt.Sprintf("%s_%s", providerName, doc.Slug), serviceSlug) {
				contentAvailable = true
				descriptionSnippet, err := c.getContentSnippet(ctx, doc.ID)
				if err != nil {
					c.logger.Warnf("Error fetching content snippet for provider doc ID: %s: %v", doc.ID, err)
				}
				builder.WriteString(fmt.Sprintf("- providerDocID: %s\n- Title: %s\n- Category: %s\n- Description: %s\n---\n",
					doc.ID, doc.Title, doc.Category, descriptionSnippet))
			}
		}
	}

	if !contentAvailable {
		return nil, fmt.Errorf("no documentation found for service_slug %s, provide a more relevant service_slug or use the provider_name for its value", serviceSlug)
	}

	result := map[string]any{
		"content": builder.String(),
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetProviderDetails gets detailed provider documentation
func (c *Client) GetProviderDetails(ctx context.Context, providerDocID string) (*mcp.CallToolResult, error) {
	c.logger.Infof("Getting provider details for doc ID: %s", providerDocID)

	if _, err := strconv.Atoi(providerDocID); err != nil {
		return nil, fmt.Errorf("provider_doc_id must be a numeric ID from the provider's documentation index, not a resource name. Got: '%s' (hint: use search_providers first to find valid IDs)", providerDocID)
	}

	apiURL := fmt.Sprintf("%s/provider-docs/%s", terraformRegistryAPIv2, providerDocID)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching provider documentation: %w", err)
	}

	var docResponse ProviderResourceDetails
	if err := json.Unmarshal(response, &docResponse); err != nil {
		return nil, fmt.Errorf("unmarshalling provider documentation: %w", err)
	}

	content := fmt.Sprintf("# %s\n\n%s", docResponse.Data.Attributes.Title, docResponse.Data.Attributes.Content)

	result := map[string]any{
		"content":  content,
		"title":    docResponse.Data.Attributes.Title,
		"category": docResponse.Data.Attributes.Category,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetLatestProviderVersion gets the latest version of a provider
func (c *Client) GetLatestProviderVersion(ctx context.Context, providerNamespace, providerName string) (*mcp.CallToolResult, error) {
	c.logger.Infof("Getting latest version for provider: %s/%s", providerNamespace, providerName)

	version, err := c.getLatestProviderVersionInternal(ctx, providerNamespace, providerName)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"version":  version,
		"provider": fmt.Sprintf("%s/%s", providerNamespace, providerName),
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// SearchModules searches for Terraform modules
func (c *Client) SearchModules(ctx context.Context, moduleQuery string, currentOffset int) (*mcp.CallToolResult, error) {
	c.logger.Infof("Searching modules: query=%s, offset=%d", moduleQuery, currentOffset)

	params := url.Values{}
	params.Set("q", moduleQuery)
	params.Set("offset", strconv.Itoa(currentOffset))
	params.Set("limit", "10")

	apiURL := fmt.Sprintf("%s/modules?%s", terraformRegistryAPIv1, params.Encode())
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("searching modules: %w", err)
	}

	var moduleResponse ModuleSearchResponse
	if err := json.Unmarshal(response, &moduleResponse); err != nil {
		return nil, fmt.Errorf("unmarshalling module search response: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Terraform Modules Search Results for: \"%s\"\n\n", moduleQuery))
	builder.WriteString("Each result includes:\n- moduleID: Module identifier for get_module_details\n- Name: Module name\n- Description: Module description\n- Downloads: Total download count\n- Verified: Official verification status\n- PublishedAt: Publication date\n\n---\n\n")

	for _, module := range moduleResponse.Modules {
		verified := "No"
		if module.Verified {
			verified = "Yes"
		}

		builder.WriteString(fmt.Sprintf("- moduleID: %s\n- Name: %s\n- Description: %s\n- Downloads: %d\n- Verified: %s\n- PublishedAt: %s\n---\n",
			module.ID, module.Name, module.Description, module.Downloads, verified, module.PublishedAt))
	}

	result := map[string]any{
		"content": builder.String(),
		"total":   len(moduleResponse.Modules),
		"offset":  currentOffset,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetModuleDetails gets detailed information about a module
func (c *Client) GetModuleDetails(ctx context.Context, moduleID string) (*mcp.CallToolResult, error) {
	c.logger.Infof("Getting module details for: %s", moduleID)

	apiURL := fmt.Sprintf("%s/modules/%s", terraformRegistryAPIv1, moduleID)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching module details: %w", err)
	}

	var moduleDetails ModuleDetails
	if err := json.Unmarshal(response, &moduleDetails); err != nil {
		return nil, fmt.Errorf("unmarshalling module details: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("# Terraform Module: %s\n\n", moduleDetails.Name))
	builder.WriteString(fmt.Sprintf("**Description:** %s\n\n", moduleDetails.Description))
	builder.WriteString(fmt.Sprintf("**Source:** %s\n", moduleDetails.Source))
	builder.WriteString(fmt.Sprintf("**Version:** %s\n", moduleDetails.Version))
	builder.WriteString(fmt.Sprintf("**Downloads:** %d\n", moduleDetails.Downloads))
	builder.WriteString(fmt.Sprintf("**Verified:** %t\n\n", moduleDetails.Verified))

	if len(moduleDetails.Inputs) > 0 {
		builder.WriteString("## Inputs\n\n")
		for _, input := range moduleDetails.Inputs {
			builder.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", input.Name, input.Type, input.Description))
		}
		builder.WriteString("\n")
	}

	if len(moduleDetails.Outputs) > 0 {
		builder.WriteString("## Outputs\n\n")
		for _, output := range moduleDetails.Outputs {
			builder.WriteString(fmt.Sprintf("- **%s**: %s\n", output.Name, output.Description))
		}
		builder.WriteString("\n")
	}

	result := map[string]any{
		"content":   builder.String(),
		"module_id": moduleID,
		"version":   moduleDetails.Version,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetLatestModuleVersion gets the latest version of a module
func (c *Client) GetLatestModuleVersion(ctx context.Context, moduleID string) (*mcp.CallToolResult, error) {
	c.logger.Infof("Getting latest version for module: %s", moduleID)

	apiURL := fmt.Sprintf("%s/modules/%s", terraformRegistryAPIv1, moduleID)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching module version: %w", err)
	}

	var moduleDetails ModuleDetails
	if err := json.Unmarshal(response, &moduleDetails); err != nil {
		return nil, fmt.Errorf("unmarshalling module details: %w", err)
	}

	result := map[string]any{
		"version":   moduleDetails.Version,
		"module_id": moduleID,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// SearchPolicies searches for Terraform policies
func (c *Client) SearchPolicies(ctx context.Context, policyQuery string) (*mcp.CallToolResult, error) {
	c.logger.Infof("Searching policies: %s", policyQuery)

	params := url.Values{}
	params.Set("q", policyQuery)
	params.Set("limit", "10")

	apiURL := fmt.Sprintf("%s/policies?%s", terraformRegistryAPIv1, params.Encode())
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("searching policies: %w", err)
	}

	var policyResponse PolicySearchResponse
	if err := json.Unmarshal(response, &policyResponse); err != nil {
		return nil, fmt.Errorf("unmarshalling policy search response: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Terraform Policy Search Results for: \"%s\"\n\n", policyQuery))
	builder.WriteString("Each result includes:\n- policyID: Policy identifier for get_policy_details\n- Name: Policy name\n- Title: Policy title\n- Downloads: Total download count\n\n---\n\n")

	for _, policy := range policyResponse.Policies {
		builder.WriteString(fmt.Sprintf("- policyID: %s\n- Name: %s\n- Title: %s\n- Downloads: %d\n---\n",
			policy.ID, policy.Name, policy.Title, policy.Downloads))
	}

	result := map[string]any{
		"content": builder.String(),
		"total":   len(policyResponse.Policies),
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetPolicyDetails gets detailed information about a policy
func (c *Client) GetPolicyDetails(ctx context.Context, policyID string) (*mcp.CallToolResult, error) {
	c.logger.Infof("Getting policy details for: %s", policyID)

	apiURL := fmt.Sprintf("%s/policies/%s", terraformRegistryAPIv1, policyID)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching policy details: %w", err)
	}

	var policyDetails PolicyDetails
	if err := json.Unmarshal(response, &policyDetails); err != nil {
		return nil, fmt.Errorf("unmarshalling policy details: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("# Terraform Policy: %s\n\n", policyDetails.Name))
	builder.WriteString(fmt.Sprintf("**Title:** %s\n\n", policyDetails.Title))
	builder.WriteString(fmt.Sprintf("**Downloads:** %d\n", policyDetails.Downloads))
	builder.WriteString(fmt.Sprintf("**Description:**\n%s\n\n", policyDetails.Description))

	result := map[string]any{
		"content":   builder.String(),
		"policy_id": policyID,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// Helper functions

func (c *Client) getLatestProviderVersionInternal(ctx context.Context, providerNamespace, providerName string) (string, error) {
	apiURL := fmt.Sprintf("%s/providers/%s/%s/versions", terraformRegistryAPIv1, providerNamespace, providerName)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("fetching provider versions: %w", err)
	}

	var versionsResponse ProviderVersionsResponse
	if err := json.Unmarshal(response, &versionsResponse); err != nil {
		return "", fmt.Errorf("unmarshalling provider versions: %w", err)
	}

	if len(versionsResponse.Versions) == 0 {
		return "", fmt.Errorf("no versions found for provider %s/%s", providerNamespace, providerName)
	}

	// The first version should be the latest
	return versionsResponse.Versions[0].Version, nil
}

func (c *Client) getProviderDetailsV2(ctx context.Context, providerNamespace, providerName, providerVersion, category string) (string, error) {
	providerVersionID, err := c.getProviderVersionID(ctx, providerNamespace, providerName, providerVersion)
	if err != nil {
		return "", fmt.Errorf("getting provider version ID: %w", err)
	}

	if category == "overview" {
		return c.getProviderOverviewDocs(ctx, providerVersionID)
	}

	params := url.Values{}
	params.Set("filter[provider-version]", providerVersionID)
	params.Set("filter[category]", category)
	params.Set("filter[language]", "hcl")

	apiURL := fmt.Sprintf("%s/provider-docs?%s", terraformRegistryAPIv2, params.Encode())
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("getting provider documentation: %w", err)
	}

	var docsResponse ProviderDocsV2Response
	if err := json.Unmarshal(response, &docsResponse); err != nil {
		return "", fmt.Errorf("unmarshalling provider docs: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Available Documentation for %s in Terraform provider %s/%s version: %s\n\n",
		category, providerNamespace, providerName, providerVersion))

	for _, doc := range docsResponse.Data {
		descriptionSnippet, err := c.getContentSnippet(ctx, doc.ID)
		if err != nil {
			c.logger.Warnf("Error fetching content snippet for provider doc ID: %s: %v", doc.ID, err)
		}
		builder.WriteString(fmt.Sprintf("- providerDocID: %s\n- Title: %s\n- Category: %s\n- Description: %s\n---\n",
			doc.ID, doc.Attributes.Title, doc.Attributes.Category, descriptionSnippet))
	}

	return builder.String(), nil
}

func (c *Client) getProviderVersionID(ctx context.Context, providerNamespace, providerName, providerVersion string) (string, error) {
	apiURL := fmt.Sprintf("%s/providers/%s/%s/versions", terraformRegistryAPIv2, providerNamespace, providerName)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("fetching provider versions: %w", err)
	}

	var versionsResponse ProviderVersionsV2Response
	if err := json.Unmarshal(response, &versionsResponse); err != nil {
		return "", fmt.Errorf("unmarshalling provider versions: %w", err)
	}

	for _, version := range versionsResponse.Data {
		if version.Attributes.Version == providerVersion {
			return version.ID, nil
		}
	}

	return "", fmt.Errorf("provider version %s not found", providerVersion)
}

func (c *Client) getProviderOverviewDocs(ctx context.Context, providerVersionID string) (string, error) {
	apiURL := fmt.Sprintf("%s/provider-versions/%s", terraformRegistryAPIv2, providerVersionID)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("fetching provider overview: %w", err)
	}

	var overviewResponse ProviderOverviewResponse
	if err := json.Unmarshal(response, &overviewResponse); err != nil {
		return "", fmt.Errorf("unmarshalling provider overview: %w", err)
	}

	return overviewResponse.Data.Attributes.Description, nil
}

func (c *Client) getContentSnippet(ctx context.Context, docID string) (string, error) {
	apiURL := fmt.Sprintf("%s/provider-docs/%s", terraformRegistryAPIv2, docID)
	response, err := c.makeRequest(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("fetching provider-docs/%s: %w", docID, err)
	}

	var docDescription ProviderResourceDetails
	if err := json.Unmarshal(response, &docDescription); err != nil {
		return "", fmt.Errorf("unmarshalling provider-docs/%s: %w", docID, err)
	}

	content := docDescription.Data.Attributes.Content
	// Try to extract description from markdown content
	desc := ""
	if start := strings.Index(content, "description: |-"); start != -1 {
		if end := strings.Index(content[start:], "\n---"); end != -1 {
			substring := content[start+len("description: |-") : start+end]
			trimmed := strings.TrimSpace(substring)
			desc = strings.ReplaceAll(trimmed, "\n", " ")
		} else {
			substring := content[start+len("description: |-"):]
			trimmed := strings.TrimSpace(substring)
			desc = strings.ReplaceAll(trimmed, "\n", " ")
		}
	}

	if len(desc) > 300 {
		return desc[:300] + "...", nil
	}
	return desc, nil
}

func isV2ProviderDataType(dataType string) bool {
	return dataType == "guides" || dataType == "functions" || dataType == "overview"
}

func containsSlug(slug, searchSlug string) bool {
	return strings.Contains(strings.ToLower(slug), strings.ToLower(searchSlug))
}
