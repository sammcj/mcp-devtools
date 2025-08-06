package unified

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/bedrock"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/docker"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/githubactions"
	go_tool "github.com/sammcj/mcp-devtools/internal/tools/packageversions/go"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/java"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/npm"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/python"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/swift"
	"github.com/sirupsen/logrus"
)

// SearchPackagesTool handles unified package searching across multiple ecosystems
type SearchPackagesTool struct {
	client packageversions.HTTPClient
}

// init registers the unified search packages tool with the registry
func init() {
	registry.Register(&SearchPackagesTool{
		client: packageversions.DefaultHTTPClient,
	})
}

// Definition returns the tool's definition for MCP registration
func (t *SearchPackagesTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"search_packages",
		mcp.WithDescription("Search for software packages / libraries (by name) and check versions across multiple ecosystems (npm, Go, Python, Java, Swift, GitHub Actions, Docker, AWS Bedrock). This tool is especially useful when writing software and adding dependencies to projects to ensure you get the latest stable version. TIP: When checking multiple packages, pass them all in a single call using the 'data' parameter rather than making separate calls for each package - this is significantly more efficient than individual calls per package."),
		mcp.WithString("ecosystem",
			mcp.Description("Package ecosystem to search. Options: 'npm' (Node.js packages), 'go' (Go modules), 'python' (PyPI packages), 'python-pyproject' (pyproject.toml format), 'java-maven' (Maven dependencies), 'java-gradle' (Gradle dependencies), 'swift' (Swift Package Manager), 'github-actions' (GitHub Actions), 'docker' (container images), 'bedrock' (AWS Bedrock models)"),
			mcp.Enum("npm", "go", "python", "python-pyproject", "java-maven", "java-gradle", "swift", "github-actions", "docker", "bedrock"),
			mcp.Required(),
		),
		mcp.WithString("query",
			mcp.Description("The search query. For packages this should be a package name or dependency object. For bedrock, use model names. For docker, use image names."),
			mcp.Required(),
		),
		mcp.WithObject("data",
			mcp.Description("Ecosystem-specific data object for checking multiple packages / libraries, structure depends on the ecosystem (e.g., for python: `[\"requests\", \"numpy\"]`, for npm: `{\"react\": \"latest\", \"lodash\": \"^4.0.0\"}`). (Optional)"),
		),
		mcp.WithObject("constraints",
			mcp.Description("Constraints for specific packages / libraries (version constraints, exclusions, etc.) (Optional)"),
		),
		mcp.WithString("action",
			mcp.Description("Action for specific ecosystems. For bedrock: 'list', 'search', 'get'. For docker: 'tags', 'info'. Defaults to appropriate action for ecosystem. (Optional)"),
			mcp.Enum("list", "search", "get", "tags", "info"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (where applicable) (Optional)"),
		),
		mcp.WithString("registry",
			mcp.Description("Specific registry to use (for docker: 'dockerhub', 'ghcr', 'custom') (Optional)"),
		),
		mcp.WithBoolean("includeDetails",
			mcp.Description("Include additional details in results (where applicable) (Optional)"),
		),
	)
}

// Execute executes the unified package search tool
func (t *SearchPackagesTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse ecosystem
	ecosystem, ok := args["ecosystem"].(string)
	if !ok || ecosystem == "" {
		return nil, fmt.Errorf("missing required parameter: ecosystem")
	}

	query, _ := args["query"].(string)
	logger.WithFields(logrus.Fields{
		"ecosystem": ecosystem,
		"query":     query,
	}).Info("Executing unified package search")

	// Route to appropriate ecosystem handler
	var result *mcp.CallToolResult
	var err error

	switch ecosystem {
	case "npm":
		result, err = t.handleNpm(ctx, logger, cache, args)
	case "go":
		result, err = t.handleGo(ctx, logger, cache, args)
	case "python":
		result, err = t.handlePython(ctx, logger, cache, args)
	case "python-pyproject":
		result, err = t.handlePythonPyproject(ctx, logger, cache, args)
	case "java-maven":
		result, err = t.handleJavaMaven(ctx, logger, cache, args)
	case "java-gradle":
		result, err = t.handleJavaGradle(ctx, logger, cache, args)
	case "swift":
		result, err = t.handleSwift(ctx, logger, cache, args)
	case "github-actions":
		result, err = t.handleGitHubActions(ctx, logger, cache, args)
	case "docker":
		result, err = t.handleDocker(ctx, logger, cache, args)
	case "bedrock":
		result, err = t.handleBedrock(ctx, logger, cache, args)
	default:
		return nil, fmt.Errorf("unsupported ecosystem: %s", ecosystem)
	}

	if err != nil {
		return nil, err
	}

	// Check if result contains useful information
	return t.validateAndEnhanceResult(result, query, ecosystem)
}

// handleNpm handles npm package searches
func (t *SearchPackagesTool) handleNpm(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Check if query contains comma-separated packages
		if strings.Contains(query, ",") {
			// Split comma-separated packages and create dependencies object
			packages := strings.Split(query, ",")
			deps := make(map[string]interface{})
			for _, pkg := range packages {
				pkg = strings.TrimSpace(pkg)
				if pkg != "" {
					deps[pkg] = "latest"
				}
			}
			args["dependencies"] = deps
		} else {
			// Convert single package query to dependencies object
			args["dependencies"] = map[string]interface{}{
				query: "latest",
			}
		}
	}

	if constraints, ok := args["constraints"]; ok {
		args["constraints"] = constraints
	}

	tool := npm.NewNpmTool(t.client)
	return tool.Execute(ctx, logger, cache, args)
}

// handleGo handles Go module searches
func (t *SearchPackagesTool) handleGo(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies object
		args["dependencies"] = map[string]interface{}{
			query: "latest",
		}
	}

	tool := go_tool.NewGoTool(t.client)
	return tool.Execute(ctx, logger, cache, args)
}

// handlePython handles Python package searches
func (t *SearchPackagesTool) handlePython(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to requirements format if needed
	if data, ok := args["data"]; ok {
		args["requirements"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to requirements array
		args["requirements"] = []interface{}{query}
	}

	tool := python.NewPythonTool(t.client)
	return tool.Execute(ctx, logger, cache, args)
}

// handlePythonPyproject handles Python pyproject.toml package searches
func (t *SearchPackagesTool) handlePythonPyproject(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies object
		args["dependencies"] = map[string]interface{}{
			"dependencies": map[string]interface{}{
				query: "latest",
			},
		}
	}

	tool := &python.PyProjectTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleJavaMaven handles Maven dependency searches
func (t *SearchPackagesTool) handleJavaMaven(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies array
		// For Maven, we need groupId and artifactId, so we'll try to parse
		args["dependencies"] = []interface{}{
			map[string]interface{}{
				"groupId":    query,
				"artifactId": query,
			},
		}
	}

	tool := &java.MavenTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleJavaGradle handles Gradle dependency searches
func (t *SearchPackagesTool) handleJavaGradle(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies array
		args["dependencies"] = []interface{}{
			map[string]interface{}{
				"configuration": "implementation",
				"group":         query,
				"name":          query,
			},
		}
	}

	tool := &java.GradleTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleSwift handles Swift package searches
func (t *SearchPackagesTool) handleSwift(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	}
	if constraints, ok := args["constraints"]; ok {
		args["constraints"] = constraints
	}

	tool := &swift.SwiftTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleGitHubActions handles GitHub Actions searches
func (t *SearchPackagesTool) handleGitHubActions(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Convert query to actions format if needed
	if data, ok := args["data"]; ok {
		args["actions"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single query to actions array
		args["actions"] = []interface{}{query}
	}

	if includeDetails, ok := args["includeDetails"]; ok {
		args["includeDetails"] = includeDetails
	}

	tool := &githubactions.GitHubActionsTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleDocker handles Docker image searches
func (t *SearchPackagesTool) handleDocker(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Use query as image name
	if query, ok := args["query"].(string); ok {
		args["image"] = query
	}

	// Pass through other docker-specific parameters
	if registry, ok := args["registry"]; ok {
		args["registry"] = registry
	}
	if limit, ok := args["limit"]; ok {
		args["limit"] = limit
	}
	if includeDetails, ok := args["includeDetails"]; ok {
		args["includeDigest"] = includeDetails
	}

	tool := docker.NewDockerTool(t.client)
	return tool.Execute(ctx, logger, cache, args)
}

// handleBedrock handles AWS Bedrock model searches
func (t *SearchPackagesTool) handleBedrock(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Set default action if not provided
	if _, ok := args["action"]; !ok {
		if query, ok := args["query"].(string); ok && query != "" {
			args["action"] = "search"
			args["query"] = query
		} else {
			args["action"] = "list"
		}
	}

	tool := &bedrock.BedrockTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// validateAndEnhanceResult checks if the result contains useful information and provides helpful error messages
func (t *SearchPackagesTool) validateAndEnhanceResult(result *mcp.CallToolResult, query, ecosystem string) (*mcp.CallToolResult, error) {
	if result == nil {
		return packageversions.NewToolResultJSON(map[string]interface{}{
			"error":     fmt.Sprintf("No results for query '%s' in ecosystem '%s'", query, ecosystem),
			"message":   "Try searching for the specific package name instead of a description.",
			"query":     query,
			"ecosystem": ecosystem,
		})
	}

	// Check if result content indicates empty or failed results
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			text := textContent.Text

			// Check for various indicators of empty/failed results
			if text == "null" || text == "[]" || text == "{}" {
				return packageversions.NewToolResultJSON(map[string]interface{}{
					"error":     fmt.Sprintf("No results for query '%s' in ecosystem '%s'", query, ecosystem),
					"message":   "Try searching for the specific package name instead of a description.",
					"query":     query,
					"ecosystem": ecosystem,
				})
			}

			// Try to parse as JSON to check for empty arrays or objects with only skipped results
			var jsonData interface{}
			if err := json.Unmarshal([]byte(text), &jsonData); err == nil {
				if array, ok := jsonData.([]interface{}); ok {
					if len(array) == 0 {
						return packageversions.NewToolResultJSON(map[string]interface{}{
							"error":     fmt.Sprintf("No results for query '%s' in ecosystem '%s'", query, ecosystem),
							"message":   "Try searching for the specific package name instead of a description.",
							"query":     query,
							"ecosystem": ecosystem,
						})
					}

					// Check if all results are skipped
					allSkipped := true
					for _, item := range array {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if skipped, exists := itemMap["skipped"]; !exists || !skipped.(bool) {
								allSkipped = false
								break
							}
						}
					}

					if allSkipped {
						return packageversions.NewToolResultJSON(map[string]interface{}{
							"error":     fmt.Sprintf("No valid results for query '%s' in ecosystem '%s'", query, ecosystem),
							"message":   "The package was not found. Try searching for the specific package name instead of a description.",
							"query":     query,
							"ecosystem": ecosystem,
						})
					}
				}
			}
		}
	}

	return result, nil
}
