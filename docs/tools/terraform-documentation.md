# Terraform Documentation

Access Terraform Registry APIs for providers, modules, and policies with comprehensive search and documentation capabilities.

## Overview

The terraform_documentation tool provides unified access to the public Terraform Registry, enabling you to:

- Search for and retrieve provider documentation
- Find and explore Terraform modules
- Search for Terraform policies
- Get detailed documentation for resources, data sources, guides, and functions
- Access the latest versions of providers and modules

## Usage

**Action-based tool:** All operations are performed through the single `terraform_documentation` tool using different `action` parameters.

### Provider Operations

#### Search Providers
Find provider documentation by searching for specific services:

```json
{
  "action": "search_providers",
  "provider_name": "aws",
  "provider_namespace": "hashicorp",
  "query": "s3",
  "provider_data_type": "resources"
}
```

#### Get Provider Details
Retrieve detailed documentation using a numeric document ID (obtain from search_providers results):

```json
{
  "action": "get_provider_details",
  "provider_doc_id": "8894603"
}
```

**Note:** The `provider_doc_id` must be a numeric ID from the provider's documentation index, not a resource name like "s3_bucket". Use `search_providers` first to find valid IDs.

#### Get Latest Provider Version
Check the latest available version for a provider:

```json
{
  "action": "get_latest_provider_version",
  "provider_name": "aws",
  "provider_namespace": "hashicorp"
}
```

### Module Operations

#### Search Modules
Find Terraform modules by query:

```json
{
  "action": "search_modules",
  "query": "vpc aws",
  "limit": 5
}
```

#### Get Module Details
Retrieve detailed module information including inputs, outputs, and examples:

```json
{
  "action": "get_module_details",
  "module_id": "terraform-aws-modules/vpc/aws/3.14.0"
}
```

**Note:** The `module_id` should include the full path: `namespace/name/provider/version` (e.g., `terraform-aws-modules/vpc/aws/3.14.0`).

#### Get Latest Module Version
Get the latest version of a specific module:

```json
{
  "action": "get_latest_module_version",
  "module_id": "terraform-aws-modules/vpc/aws"
}
```

**Note:** For version queries, you can omit the version from the module_id.

### Policy Operations

#### Search Policies
Find Terraform Sentinel policies:

```json
{
  "action": "search_policies",
  "query": "security compliance"
}
```

#### Get Policy Details
Retrieve detailed policy information:

```json
{
  "action": "get_policy_details",
  "policy_id": "policy-12345"
}
```

## Parameters

### Common Parameters

| Parameter | Type              | Description                             |
|-----------|-------------------|-----------------------------------------|
| `action`  | String (required) | The operation to perform                |
| `query`   | String            | Search query (service slug for providers, search terms for modules/policies) |
| `limit`   | Number            | Maximum results to return (default: 10) |

### Provider Parameters

| Parameter            | Type   | Description                                                                        |
|----------------------|--------|------------------------------------------------------------------------------------|
| `provider_name`      | String | Provider name (e.g., 'aws', 'google')                                              |
| `provider_namespace` | String | Publisher namespace (e.g., 'hashicorp')                                            |
| `provider_data_type` | String | Documentation type: 'resources', 'data-sources', 'functions', 'guides', 'overview' |
| `provider_version`   | String | Specific version or 'latest'                                                       |
| `provider_doc_id`    | String | Document ID from search results                                                    |

### Module Parameters

| Parameter        | Type   | Description                              |
|------------------|--------|------------------------------------------|
| `module_id`      | String | Full module ID (namespace/name/provider) |
| `current_offset` | Number | Pagination offset (default: 0)           |

### Policy Parameters

| Parameter      | Type   | Description                   |
|----------------|--------|-------------------------------|
| `policy_id`    | String | Policy ID from search results |

## Configuration

The tool is **disabled by default** for security reasons. To enable:

```json
{
  "env": {
    "ENABLE_ADDITIONAL_TOOLS": "terraform_documentation"
  }
}
```

Or combine with other tools:
```json
{
  "env": {
    "ENABLE_ADDITIONAL_TOOLS": "terraform_documentation,memory,security"
  }
}
```

## Examples

### Complete Provider Documentation Workflow

1. Search for AWS S3 resources:
```json
{
  "action": "search_providers",
  "provider_name": "aws",
  "provider_namespace": "hashicorp",
  "query": "s3",
  "provider_data_type": "resources"
}
```

2. Use the returned `providerDocID` to get detailed documentation:
```json
{
  "action": "get_provider_details",
  "provider_doc_id": "8894603"
}
```

### Module Discovery and Analysis

1. Search for VPC modules:
```json
{
  "action": "search_modules",
  "query": "vpc aws official"
}
```

2. Get details for a specific module:
```json
{
  "action": "get_module_details",
  "module_id": "terraform-aws-modules/vpc/aws"
}
```

## Limitations

- Only supports the public Terraform Registry (registry.terraform.io)
- Cannot access private registries or Terraform Enterprise/Cloud
- Read-only operations - cannot publish or modify resources
- Rate limited by Terraform Registry API limits

## Security

- All HTTP requests go through security analysis
- Only accesses public Terraform Registry APIs
- No authentication or sensitive data required
- Content filtering applies to returned documentation

## Troubleshooting

**"terraform_documentation tool is not enabled"**
- Add 'terraform_documentation' to ENABLE_ADDITIONAL_TOOLS environment variable

**"No documentation found for query"**
- Try broader search terms or use the provider name as query
- Check that provider_name and provider_namespace are correct

**"Invalid provider_doc_id error"**
- Always run search_providers first to get valid document IDs
- Don't manually construct or guess document IDs

**"No versions found for provider"**
- Verify the provider exists in the public registry
- Try 'hashicorp' namespace for official providers
- Check provider_name spelling
