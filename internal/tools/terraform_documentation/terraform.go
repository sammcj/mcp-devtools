package terraform_documentation

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// TerraformDocumentationTool implements the tools.Tool interface
type TerraformDocumentationTool struct {
	client *Client
}

// init registers the tool with the registry
func init() {
	registry.Register(&TerraformDocumentationTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *TerraformDocumentationTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"terraform_documentation",
		mcp.WithDescription("Access Terraform Registry APIs for providers, modules, and policies with search and documentation capabilities."),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform"),
			mcp.Enum("search_providers", "get_provider_details", "get_latest_provider_version", "search_modules", "get_module_details", "get_latest_module_version", "search_policies", "get_policy_details"),
		),
		// Common search parameter (consolidates *_query parameters)
		mcp.WithString("query",
			mcp.Description("Search query (for search_modules, search_policies, or service slug for search_providers). Examples: 'vpc', 'kubernetes', 'security'"),
		),
		// Resource-specific ID parameters (semantically different, keep separate)
		mcp.WithString("provider_doc_id",
			mcp.Description("Terraform provider document ID (required for get_provider_details) - must be a numeric ID from the provider's documentation index"),
		),
		mcp.WithString("module_id",
			mcp.Description("Terraform module ID (required for get_module_details and get_latest_module_version). Format: 'namespace/name/provider/version' (e.g., 'terraform-aws-modules/vpc/aws/3.14.0')"),
		),
		mcp.WithString("policy_id",
			mcp.Description("Terraform policy ID (required for get_policy_details)"),
		),
		// Provider-specific parameters (grouped together for clarity)
		mcp.WithString("provider_name",
			mcp.Description("Name of the Terraform provider (required for all provider actions). Examples: 'aws', 'azurerm', 'google'"),
		),
		mcp.WithString("provider_namespace",
			mcp.Description("Publisher namespace of the Terraform provider (required for all provider actions). Examples: 'hashicorp', 'integrations'"),
		),
		mcp.WithString("provider_data_type",
			mcp.Description("Type of provider documentation to retrieve"),
			mcp.Enum("resources", "data-sources", "functions", "guides", "overview"),
			mcp.DefaultString("resources"),
		),
		mcp.WithString("provider_version",
			mcp.Description("Provider version (Default: 'latest', optional: specific version 'x.y.z')"),
			mcp.DefaultString("latest"),
		),
		// General pagination and limits
		mcp.WithNumber("current_offset",
			mcp.Description("Pagination offset for search operations"),
			mcp.DefaultNumber(0),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return for search operations"),
			mcp.DefaultNumber(5),
		),
	)
}

// Execute executes the tool's logic
func (t *TerraformDocumentationTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if terraform_documentation tool is enabled (disabled by default)
	if !tools.IsToolEnabled("terraform_documentation") {
		return nil, fmt.Errorf("terraform documentation tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'terraform_documentation'")
	}

	// Initialise client if needed
	if t.client == nil {
		t.client = NewClient(logger)
	}

	// Parse action parameter
	action, ok := args["action"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: action")
	}

	action = strings.TrimSpace(action)
	if action == "" {
		return nil, fmt.Errorf("action parameter cannot be empty")
	}

	// Dispatch to appropriate handler
	switch action {
	case "search_providers":
		return t.executeSearchProviders(ctx, logger, cache, args)
	case "get_provider_details":
		return t.executeGetProviderDetails(ctx, logger, cache, args)
	case "get_latest_provider_version":
		return t.executeGetLatestProviderVersion(ctx, logger, cache, args)
	case "search_modules":
		return t.executeSearchModules(ctx, logger, cache, args)
	case "get_module_details":
		return t.executeGetModuleDetails(ctx, logger, cache, args)
	case "get_latest_module_version":
		return t.executeGetLatestModuleVersion(ctx, logger, cache, args)
	case "search_policies":
		return t.executeSearchPolicies(ctx, logger, cache, args)
	case "get_policy_details":
		return t.executeGetPolicyDetails(ctx, logger, cache, args)
	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// executeSearchProviders handles search_providers action
func (t *TerraformDocumentationTool) executeSearchProviders(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	providerName, ok := args["provider_name"].(string)
	if !ok || providerName == "" {
		return nil, fmt.Errorf("search_providers requires 'provider_name' parameter (e.g., 'aws', 'azurerm', 'google')")
	}

	providerNamespace, ok := args["provider_namespace"].(string)
	if !ok || providerNamespace == "" {
		return nil, fmt.Errorf("search_providers requires 'provider_namespace' parameter (e.g., 'hashicorp', 'integrations')")
	}

	serviceSlug, ok := args["query"].(string)
	if !ok || serviceSlug == "" {
		return nil, fmt.Errorf("search_providers requires 'query' parameter for service slug (e.g., 's3', 'compute', 'networking')")
	}

	providerDataType := "resources"
	if pdt, ok := args["provider_data_type"].(string); ok && pdt != "" {
		providerDataType = pdt
	}

	providerVersion := "latest"
	if pv, ok := args["provider_version"].(string); ok && pv != "" {
		providerVersion = pv
	}

	return t.client.SearchProviders(ctx, providerName, providerNamespace, serviceSlug, providerDataType, providerVersion)
}

// executeGetProviderDetails handles get_provider_details action
func (t *TerraformDocumentationTool) executeGetProviderDetails(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	providerDocID, ok := args["provider_doc_id"].(string)
	if !ok || providerDocID == "" {
		return nil, fmt.Errorf("missing required parameter: provider_doc_id")
	}

	return t.client.GetProviderDetails(ctx, providerDocID)
}

// executeGetLatestProviderVersion handles get_latest_provider_version action
func (t *TerraformDocumentationTool) executeGetLatestProviderVersion(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	providerName, ok := args["provider_name"].(string)
	if !ok || providerName == "" {
		return nil, fmt.Errorf("missing required parameter: provider_name")
	}

	providerNamespace, ok := args["provider_namespace"].(string)
	if !ok || providerNamespace == "" {
		return nil, fmt.Errorf("missing required parameter: provider_namespace")
	}

	return t.client.GetLatestProviderVersion(ctx, providerNamespace, providerName)
}

// executeSearchModules handles search_modules action
func (t *TerraformDocumentationTool) executeSearchModules(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	moduleQuery, ok := args["query"].(string)
	if !ok || moduleQuery == "" {
		return nil, fmt.Errorf("missing required parameter: query")
	}

	currentOffset := 0
	if co, ok := args["current_offset"].(float64); ok {
		currentOffset = int(co)
	}

	return t.client.SearchModules(ctx, moduleQuery, currentOffset)
}

// executeGetModuleDetails handles get_module_details action
func (t *TerraformDocumentationTool) executeGetModuleDetails(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	moduleID, ok := args["module_id"].(string)
	if !ok || moduleID == "" {
		return nil, fmt.Errorf("missing required parameter: module_id")
	}

	return t.client.GetModuleDetails(ctx, moduleID)
}

// executeGetLatestModuleVersion handles get_latest_module_version action
func (t *TerraformDocumentationTool) executeGetLatestModuleVersion(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	moduleID, ok := args["module_id"].(string)
	if !ok || moduleID == "" {
		return nil, fmt.Errorf("missing required parameter: module_id")
	}

	return t.client.GetLatestModuleVersion(ctx, moduleID)
}

// executeSearchPolicies handles search_policies action
func (t *TerraformDocumentationTool) executeSearchPolicies(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	policyQuery, ok := args["query"].(string)
	if !ok || policyQuery == "" {
		return nil, fmt.Errorf("missing required parameter: query")
	}

	return t.client.SearchPolicies(ctx, policyQuery)
}

// executeGetPolicyDetails handles get_policy_details action
func (t *TerraformDocumentationTool) executeGetPolicyDetails(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	policyID, ok := args["policy_id"].(string)
	if !ok || policyID == "" {
		return nil, fmt.Errorf("missing required parameter: policy_id")
	}

	return t.client.GetPolicyDetails(ctx, policyID)
}

// ProvideExtendedInfo provides extended help information for the tool
func (t *TerraformDocumentationTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Search for AWS provider resources",
				Arguments: map[string]any{
					"action":             "search_providers",
					"provider_name":      "aws",
					"provider_namespace": "hashicorp",
					"query":              "s3",
					"provider_data_type": "resources",
				},
				ExpectedResult: "Returns list of AWS provider resources related to S3 with document IDs",
			},
			{
				Description: "Get specific provider documentation",
				Arguments: map[string]any{
					"action":          "get_provider_details",
					"provider_doc_id": "8894603",
				},
				ExpectedResult: "Returns detailed documentation for the specified provider resource in markdown format",
			},
			{
				Description: "Search for Terraform modules",
				Arguments: map[string]any{
					"action": "search_modules",
					"query":  "vpc aws",
					"limit":  5,
				},
				ExpectedResult: "Returns list of Terraform modules matching the VPC query with module IDs",
			},
			{
				Description: "Get module details",
				Arguments: map[string]any{
					"action":    "get_module_details",
					"module_id": "terraform-aws-modules/vpc/aws",
				},
				ExpectedResult: "Returns detailed module documentation including inputs, outputs, and examples",
			},
		},
		CommonPatterns: []string{
			"Always use search_providers before get_provider_details to get valid document IDs",
			"Always use search_modules before get_module_details to get valid module IDs",
			"Provider actions require provider_name and provider_namespace parameters",
			"Use query parameter for service slug (providers), search terms (modules/policies)",
			"All search operations support pagination using current_offset parameter",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Invalid provider_doc_id error",
				Solution: "First run search_providers to get valid provider document IDs, don't guess the ID",
			},
			{
				Problem:  "No provider documentation found",
				Solution: "Check provider_name and provider_namespace are correct, try 'hashicorp' namespace for official providers",
			},
			{
				Problem:  "Module search returns no results",
				Solution: "Try broader search terms, check module_query spelling, or search without specific provider names",
			},
		},
		ParameterDetails: map[string]string{
			"action":             "Determines which Terraform Registry API to call - providers, modules, or policies",
			"query":              "Unified search parameter - service slug for providers (e.g., 's3'), search terms for modules/policies",
			"provider_name":      "The provider name (e.g., 'aws', 'google', 'azurerm') - usually matches the provider binary name",
			"provider_namespace": "The publisher namespace (e.g., 'hashicorp', 'integrations') - check Terraform Registry for correct namespace",
			"provider_doc_id":    "Numeric ID from search_providers results - required for getting specific documentation",
			"module_id":          "Full module identifier in format 'namespace/name/provider' (e.g., 'terraform-aws-modules/vpc/aws')",
			"policy_id":          "Policy identifier from search_policies results - required for getting specific policy details",
		},
		WhenToUse:    "Use when you need Terraform provider documentation, module information, or policy details from the public Terraform Registry",
		WhenNotToUse: "Don't use for local Terraform state inspection, private registries, or Terraform CLI operations",
	}
}
