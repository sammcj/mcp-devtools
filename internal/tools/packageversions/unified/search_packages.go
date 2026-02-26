package unified

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/anthropic"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/bedrock"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/docker"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/githubactions"
	go_tool "github.com/sammcj/mcp-devtools/internal/tools/packageversions/go"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/java"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/npm"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/python"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/rust"
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
		mcp.WithDescription("Search for software packages / libraries (by name) and check versions (npm, Go, Python, Java, Swift, GitHub Actions, Docker, Anthropic, AWS Bedrock, Rust). Use when adding or updating dependencies or Anthropic model IDs in code to ensure you get the latest stable version. Pass multiple packages in a single call using the 'data' parameter"),
		mcp.WithString("ecosystem",
			mcp.Description("Ecosystem to search: 'npm', 'go', 'python', 'python-pyproject' (pyproject.toml), 'java-maven', 'java-gradle', 'swift', 'github-actions', 'docker', 'anthropic' (Anthropic Claude models), 'bedrock' (AWS Bedrock models), 'rust'"),
			mcp.Enum("npm", "go", "python", "python-pyproject", "java-maven", "java-gradle", "swift", "github-actions", "docker", "anthropic", "bedrock", "rust"),
			mcp.Required(),
		),
		mcp.WithString("query",
			mcp.Description("Search query. Package name or dependency object, for bedrock use model names, for docker use image names"),
			mcp.Required(),
		),
		mcp.WithObject("data",
			mcp.Description("Ecosystem-specific data object for checking multiple packages / libraries, structure depends on the ecosystem (e.g., for python: `[\"requests\", \"numpy\"]`, for npm: `{\"react\": \"latest\", \"lodash\": \"^4.0.0\"}`) (Optional)"),
		),
		mcp.WithObject("constraints",
			mcp.Description("Constraints for specific packages / libraries (version constraints, exclusions, etc.) (Optional)"),
		),
		mcp.WithString("action",
			mcp.Description("Action for ecosystem. Bedrock: 'list', 'search', 'get'. Docker: 'tags', 'info'. Defaults to appropriate action for ecosystem (Optional)"),
			mcp.Enum("list", "search", "get", "tags", "info"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results to return (Optional)"),
		),
		mcp.WithString("registry",
			mcp.Description("Registry to use (for docker: 'dockerhub', 'ghcr', 'custom') (Optional)"),
		),
		mcp.WithBoolean("includeDetails",
			mcp.Description("Verbose details in results (Optional)"),
		),
		// Read-only annotations for package search tool
		mcp.WithReadOnlyHintAnnotation(true),     // Only searches packages, doesn't modify environment
		mcp.WithDestructiveHintAnnotation(false), // No destructive operations
		mcp.WithIdempotentHintAnnotation(true),   // Same search query returns same results
		mcp.WithOpenWorldHintAnnotation(true),    // Searches external package registries
	)
}

// Execute executes the unified package search tool
func (t *SearchPackagesTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
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
	case "anthropic":
		result, err = t.handleAnthropic(ctx, logger, cache, args)
	case "bedrock":
		result, err = t.handleBedrock(ctx, logger, cache, args)
	case "rust":
		result, err = t.handleRust(ctx, logger, cache, args)
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
func (t *SearchPackagesTool) handleNpm(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Check if query contains comma-separated packages
		if strings.Contains(query, ",") {
			// Split comma-separated packages and create dependencies object
			packages := strings.Split(query, ",")
			deps := make(map[string]any)
			for _, pkg := range packages {
				pkg = strings.TrimSpace(pkg)
				if pkg != "" {
					deps[pkg] = "latest"
				}
			}
			args["dependencies"] = deps
		} else {
			// Convert single package query to dependencies object
			args["dependencies"] = map[string]any{
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
func (t *SearchPackagesTool) handleGo(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies object
		args["dependencies"] = map[string]any{
			query: "latest",
		}
	}

	tool := go_tool.NewGoTool(t.client)
	return tool.Execute(ctx, logger, cache, args)
}

// handlePython handles Python package searches
func (t *SearchPackagesTool) handlePython(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to requirements format if needed
	if data, ok := args["data"]; ok {
		args["requirements"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to requirements array
		args["requirements"] = []any{query}
	}

	tool := python.NewPythonTool(t.client)
	return tool.Execute(ctx, logger, cache, args)
}

// handlePythonPyproject handles Python pyproject.toml package searches
func (t *SearchPackagesTool) handlePythonPyproject(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies object
		args["dependencies"] = map[string]any{
			"dependencies": map[string]any{
				query: "latest",
			},
		}
	}

	tool := &python.PyProjectTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleJavaMaven handles Maven dependency searches
func (t *SearchPackagesTool) handleJavaMaven(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies array
		// For Maven, we need groupId and artifactId, so we'll try to parse
		args["dependencies"] = []any{
			map[string]any{
				"groupId":    query,
				"artifactId": query,
			},
		}
	}

	tool := &java.MavenTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleJavaGradle handles Gradle dependency searches
func (t *SearchPackagesTool) handleJavaGradle(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		args["dependencies"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single package query to dependencies array
		args["dependencies"] = []any{
			map[string]any{
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
func (t *SearchPackagesTool) handleSwift(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
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
func (t *SearchPackagesTool) handleGitHubActions(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to actions format if needed
	if data, ok := args["data"]; ok {
		args["actions"] = data
	} else if query, ok := args["query"].(string); ok {
		// Convert single query to actions array
		args["actions"] = []any{query}
	}

	if includeDetails, ok := args["includeDetails"]; ok {
		args["includeDetails"] = includeDetails
	}

	tool := &githubactions.GitHubActionsTool{}
	return tool.Execute(ctx, logger, cache, args)
}

// handleDocker handles Docker image searches
func (t *SearchPackagesTool) handleDocker(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
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

// handleAnthropic handles Anthropic Claude model searches
func (t *SearchPackagesTool) handleAnthropic(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Set default action if not provided
	if _, ok := args["action"]; !ok {
		if query, ok := args["query"].(string); ok && query != "" {
			args["action"] = "search"
			args["query"] = query
		} else {
			args["action"] = "list"
		}
	}

	tool := anthropic.NewAnthropicTool()
	return tool.Execute(ctx, logger, cache, args)
}

// handleBedrock handles AWS Bedrock model searches
func (t *SearchPackagesTool) handleBedrock(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
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

// handleRust handles Rust crate searches
func (t *SearchPackagesTool) handleRust(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Convert query to dependencies format if needed
	if data, ok := args["data"]; ok {
		// Handle array format by converting to object: ["serde", "tokio"] -> {"serde": "latest", "tokio": "latest"}
		switch v := data.(type) {
		case []any:
			deps := make(map[string]any)
			for _, item := range v {
				if name, isStr := item.(string); isStr && name != "" {
					deps[strings.TrimSpace(name)] = "latest"
				}
			}
			args["dependencies"] = deps
		case string:
			// Handle JSON string that represents an array: "[\"serde\", \"tokio\"]"
			trimmed := strings.TrimSpace(v)
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				var arr []string
				if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
					deps := make(map[string]any)
					for _, name := range arr {
						if strings.TrimSpace(name) != "" {
							deps[strings.TrimSpace(name)] = "latest"
						}
					}
					args["dependencies"] = deps
				} else {
					args["dependencies"] = data
				}
			} else {
				args["dependencies"] = data
			}
		default:
			args["dependencies"] = data
		}
	} else if query, ok := args["query"].(string); ok {
		// Check if query contains comma-separated crates
		if strings.Contains(query, ",") {
			// Split comma-separated crates and create dependencies object
			crates := strings.Split(query, ",")
			deps := make(map[string]any)
			for _, crate := range crates {
				crate = strings.TrimSpace(crate)
				if crate != "" {
					deps[crate] = "latest"
				}
			}
			args["dependencies"] = deps
		} else {
			// Convert single crate query to dependencies object
			args["dependencies"] = map[string]any{
				query: "latest",
			}
		}
	}

	// Pass through includeDetails parameter
	if includeDetails, ok := args["includeDetails"]; ok {
		args["includeDetails"] = includeDetails
	}

	tool := rust.NewRustTool(t.client)
	return tool.Execute(ctx, logger, cache, args)
}

// validateAndEnhanceResult checks if the result contains useful information and provides helpful error messages
func (t *SearchPackagesTool) validateAndEnhanceResult(result *mcp.CallToolResult, query, ecosystem string) (*mcp.CallToolResult, error) {
	if result == nil {
		return packageversions.NewToolResultJSON(map[string]any{
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
				return packageversions.NewToolResultJSON(map[string]any{
					"error":     fmt.Sprintf("No results for query '%s' in ecosystem '%s'", query, ecosystem),
					"message":   "Try searching for the specific package name instead of a description.",
					"query":     query,
					"ecosystem": ecosystem,
				})
			}

			// Try to parse as JSON to check for empty arrays or objects with only skipped results
			var jsonData any
			if err := json.Unmarshal([]byte(text), &jsonData); err == nil {
				if array, ok := jsonData.([]any); ok {
					if len(array) == 0 {
						return packageversions.NewToolResultJSON(map[string]any{
							"error":     fmt.Sprintf("No results for query '%s' in ecosystem '%s'", query, ecosystem),
							"message":   "Try searching for the specific package name instead of a description.",
							"query":     query,
							"ecosystem": ecosystem,
						})
					}

					// Check if all results are skipped
					allSkipped := true
					for _, item := range array {
						if itemMap, ok := item.(map[string]any); ok {
							if skipped, exists := itemMap["skipped"]; !exists || !skipped.(bool) {
								allSkipped = false
								break
							}
						}
					}

					if allSkipped {
						return packageversions.NewToolResultJSON(map[string]any{
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

// ProvideExtendedInfo provides detailed usage information for the search_packages tool
func (t *SearchPackagesTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Check single npm package version",
				Arguments: map[string]any{
					"ecosystem": "npm",
					"query":     "react",
				},
				ExpectedResult: "Returns latest version information for React from npm registry",
			},
			{
				Description: "Check multiple npm packages efficiently",
				Arguments: map[string]any{
					"ecosystem": "npm",
					"query":     "react,lodash,axios",
					"data": map[string]any{
						"react":  "latest",
						"lodash": "^4.0.0",
						"axios":  "~1.0.0",
					},
				},
				ExpectedResult: "Returns version information for multiple packages in a single call with specific version constraints",
			},
			{
				Description: "Search for Python packages",
				Arguments: map[string]any{
					"ecosystem": "python",
					"query":     "requests",
					"data":      []any{"requests", "numpy", "pandas"},
				},
				ExpectedResult: "Returns PyPI information for Python packages including latest versions and dependencies",
			},
			{
				Description: "Find Docker image tags",
				Arguments: map[string]any{
					"ecosystem": "docker",
					"query":     "nginx",
					"action":    "tags",
					"limit":     10,
				},
				ExpectedResult: "Returns available Docker image tags for nginx from DockerHub",
			},
			{
				Description: "Search AWS Bedrock models",
				Arguments: map[string]any{
					"ecosystem": "bedrock",
					"query":     "anthropic",
					"action":    "search",
				},
				ExpectedResult: "Returns available Anthropic models on AWS Bedrock with their details",
			},
			{
				Description: "Check Go module with specific version",
				Arguments: map[string]any{
					"ecosystem": "go",
					"query":     "github.com/gin-gonic/gin",
					"data": map[string]any{
						"github.com/gin-gonic/gin": "v1.9.0",
					},
				},
				ExpectedResult: "Returns Go module information and version details from the Go module proxy",
			},
			{
				Description: "Check Rust crate versions with object format",
				Arguments: map[string]any{
					"ecosystem": "rust",
					"query":     "serde",
					"data": map[string]any{
						"serde": "1.0",
						"tokio": "1.0",
						"clap":  map[string]any{"version": "4.0", "features": []string{"derive"}},
					},
				},
				ExpectedResult: "Returns Rust crate information and latest versions from crates.io",
			},
			{
				Description: "Check Rust crate versions with array format (simpler)",
				Arguments: map[string]any{
					"ecosystem": "rust",
					"query":     "serde",
					"data":      []string{"serde", "tokio", "tower-lsp", "serde_json"},
				},
				ExpectedResult: "Returns latest versions for multiple Rust crates from crates.io",
			},
		},
		CommonPatterns: []string{
			"Use 'data' parameter for batch checking multiple packages - much more efficient than separate calls",
			"Specify version constraints in data object (npm: '^1.0.0', python: '>=1.0.0', etc.)",
			"For Docker: use 'tags' action to see available versions, 'info' for metadata",
			"For Bedrock: use 'list' to see all models, 'search' to find specific providers",
			"Common workflow: search → check versions → update dependency files",
			"Combine with package documentation tools for complete development workflow",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "No results returned for package query",
				Solution: "Ensure you're using the exact package name, not a description. Try searching on the ecosystem's official site first to get the correct name.",
			},
			{
				Problem:  "Unsupported ecosystem error",
				Solution: "Check that the ecosystem parameter matches one of: npm, go, python, python-pyproject, java-maven, java-gradle, swift, github-actions, docker, bedrock.",
			},
			{
				Problem:  "Version constraint errors with npm or Python",
				Solution: "Use proper semver format for npm (^1.0.0, ~1.0.0) or Python version specifiers (>=1.0.0, ==1.0.0). Check ecosystem-specific documentation for correct syntax.",
			},
			{
				Problem:  "Docker registry not found errors",
				Solution: "Specify the registry parameter ('dockerhub', 'ghcr') or ensure the image name includes the registry prefix for custom registries.",
			},
			{
				Problem:  "Bedrock models not accessible",
				Solution: "Bedrock models may be region-specific or require specific AWS permissions. The tool shows available models but access depends on your AWS configuration.",
			},
			{
				Problem:  "Java Maven/Gradle dependency format issues",
				Solution: "For Java, provide groupId:artifactId format (e.g., 'org.springframework:spring-core') or use the data parameter with proper Maven/Gradle dependency structure.",
			},
		},
		ParameterDetails: map[string]string{
			"ecosystem":      "The package ecosystem to search. Each ecosystem has different capabilities: npm (Node.js), python (PyPI), go (modules), java-maven/gradle (JVM), swift (SPM), github-actions (workflows), docker (containers), bedrock (AI models), rust (crates.io).",
			"query":          "Package identifier - exact names work best. For multiple packages, can use comma-separated list or better yet use the 'data' parameter for batch operations.",
			"data":           "Ecosystem-specific bulk data structure. Much more efficient than multiple individual calls. Format varies by ecosystem - check examples for correct structure.",
			"constraints":    "Version constraints or filters. Format depends on ecosystem (npm: semver, python: PEP 440, etc.). Use for dependency resolution and compatibility checking.",
			"action":         "Operation type for specific ecosystems. Docker: 'tags' (list versions), 'info' (metadata). Bedrock: 'list' (all models), 'search' (by provider), 'get' (specific model).",
			"limit":          "Maximum results to return. Useful for large package lists or when you only need recent versions. Different ecosystems have different default limits.",
			"registry":       "Registry to use for ecosystems that support multiple registries (mainly Docker: 'dockerhub', 'ghcr'). Most ecosystems use their default official registry.",
			"includeDetails": "Whether to include additional metadata like descriptions, download stats, etc. Increases response size but provides richer information for decision-making.",
		},
		WhenToUse:    "Use when adding dependencies to projects, checking for updates, comparing package versions, or researching available packages. Essential for maintaining up-to-date and secure dependencies in software projects.",
		WhenNotToUse: "Don't use for packages not in public registries, for general software discovery (use internet_search instead), or when you need detailed API documentation (use get_library_documentation tool for that).",
	}
}
