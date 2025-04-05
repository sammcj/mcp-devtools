package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/sammcj/mcp-package-version-go/internal/cache"
	"github.com/sammcj/mcp-package-version-go/internal/handlers"
	"github.com/sirupsen/logrus"
)

const (
	// CacheTTL is the time-to-live for cached data (1 hour)
	CacheTTL = 1 * time.Hour
)

// PackageVersionServer implements the MCPServerHandler interface for the package version server
type PackageVersionServer struct {
	logger      *logrus.Logger
	cache       *cache.Cache
	sharedCache *sync.Map
	Version     string
	Commit      string
	BuildDate   string
}

// NewPackageVersionServer creates a new package version server
func NewPackageVersionServer(version, commit, buildDate string) *PackageVersionServer {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return &PackageVersionServer{
		logger:      logger,
		cache:       cache.NewCache(CacheTTL),
		sharedCache: &sync.Map{},
		Version:     version,
		Commit:      commit,
		BuildDate:   buildDate,
	}
}

// Name returns the display name of the server
func (s *PackageVersionServer) Name() string {
	return "Package Version"
}

// Capabilities returns the server capabilities
func (s *PackageVersionServer) Capabilities() []mcpserver.ServerOption {
	return []mcpserver.ServerOption{
		mcpserver.WithToolCapabilities(true),
	}
}

// Initialize sets up the server
func (s *PackageVersionServer) Initialize(srv *mcpserver.MCPServer) error {
	// Set up the logger
	pid := os.Getpid()
	s.logger.WithFields(logrus.Fields{
		"pid": pid,
	}).Info("Starting package-version MCP server")

	s.logger.Info("Initializing package version handlers")

	// Register tools and handlers
	s.registerNpmTool(srv)
	s.registerPythonTools(srv)
	s.registerJavaTools(srv)
	s.registerGoTool(srv)
	s.registerBedrockTools(srv)
	s.registerDockerTool(srv)
	s.registerSwiftTool(srv)
	s.registerGitHubActionsTool(srv)

	s.logger.Info("All handlers registered successfully")
	return nil
}

// Start starts the MCP server with the specified transport
func (s *PackageVersionServer) Start(transport, port, baseURL string) error {
	// Create a new MCP server
	s.logger.WithFields(logrus.Fields{
		"transport": transport,
		"port":      port,
		"baseURL":   baseURL,
	}).Info("Starting MCP server")

	// Create a context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new server
	srv := mcpserver.NewMCPServer("package-version", "Package Version MCP Server")

	// Initialize the server
	if err := s.Initialize(srv); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Run the server based on the transport type
	errCh := make(chan error, 1)

	if transport == "sse" {
		// Create an SSE server
		// Ensure the baseURL has the correct format: http://hostname:port
		// Remove trailing slash if present
		if baseURL[len(baseURL)-1] == '/' {
			baseURL = baseURL[:len(baseURL)-1]
		}

		// Ensure the baseURL is correctly formatted for SSE
		// The mcp-go package expects the baseURL to be in the format: http://hostname:port
		// without any trailing slashes or paths

		// First, check if baseURL already includes a port
		var sseBaseURL string
		if baseURL == "http://localhost" || baseURL == "https://localhost" {
			// If baseURL is just http://localhost or https://localhost, append the port
			sseBaseURL = fmt.Sprintf("%s:%s", baseURL, port)
		} else {
			// Otherwise, use the baseURL as is, assuming it already includes the port if needed
			sseBaseURL = baseURL
		}

		// Try without any path component first
		// The mcp-go package might expect the baseURL to be just the host and port
		// without any path component

		// Remove any path component from the baseURL
		if strings.Contains(sseBaseURL, "/") {
			parts := strings.Split(sseBaseURL, "/")
			if len(parts) >= 3 { // http://hostname/path -> ["http:", "", "hostname", "path"]
				sseBaseURL = parts[0] + "//" + parts[2] // Reconstruct http://hostname
				if strings.Contains(parts[2], ":") {
					// If hostname already includes port, use it as is
					sseBaseURL = parts[0] + "//" + parts[2]
				} else {
					// Otherwise, append the port
					sseBaseURL = fmt.Sprintf("%s:%s", sseBaseURL, port)
				}
			}
		}

		s.logger.WithField("baseURL", sseBaseURL).Info("Configuring SSE server with base URL")

		// Create the SSE server with the correct base URL
		// The WithBaseURL option is critical for the client to connect properly
		// Try with different options to see what works

		// Try with a specific path for the SSE endpoint
		// The client might be expecting a specific path like /mcp/sse
		// Let's try with just the base URL without any path
		sseBaseURL = strings.TrimSuffix(sseBaseURL, "/mcp")

		// Add SSE server options
		sseOptions := []mcpserver.SSEOption{
			mcpserver.WithBaseURL(sseBaseURL),
			// Try adding a path option if available
			// This is a guess since we don't have direct access to the mcp-go package source
			// The SSE server might expect a specific path for the SSE endpoint
		}

		// Create the SSE server with the options
		sseServer := mcpserver.NewSSEServer(srv, sseOptions...)

		// Start the SSE server in a goroutine
		go func() {
			s.logger.WithFields(logrus.Fields{
				"baseURL": sseBaseURL,
				"port":    port,
			}).Info("SSE server is running. Press Ctrl+C to stop.")

			// Start the SSE server on the specified port
			// The server will listen on all interfaces (0.0.0.0)
			listenAddr := ":" + port
			s.logger.WithFields(logrus.Fields{
				"listenAddr": listenAddr,
				"baseURL":    sseBaseURL,
				"serverName": "package-version", // Use the known server name
			}).Info("Starting SSE server")

			// Log the available routes for debugging
			s.logger.Info("Expected SSE routes:")
			s.logger.Info("- " + sseBaseURL + "/")
			s.logger.Info("- " + sseBaseURL + "/sse")
			s.logger.Info("- " + sseBaseURL + "/events")
			s.logger.Info("- " + sseBaseURL + "/mcp")
			s.logger.Info("- " + sseBaseURL + "/mcp/sse")

			// Try accessing the routes to see if they're available
			s.logger.Info("Checking routes availability:")
			s.logger.Info("To test routes, run: curl " + sseBaseURL + "/sse")

			if err := sseServer.Start(listenAddr); err != nil {
				errCh <- fmt.Errorf("SSE server error: %w", err)
			}
		}()

		// Wait for signal to shut down
		<-sigCh
		s.logger.Info("Shutting down SSE server...")
		cancel()
		errCh <- nil
	} else {
		// Default to stdio transport
		go func() {
			s.logger.Info("STDIO server is running. Press Ctrl+C to stop.")

			if err := mcpserver.ServeStdio(srv); err != nil {
				errCh <- fmt.Errorf("STDIO server error: %w", err)
			}
		}()

		// Wait for signal to shut down
		<-sigCh
		s.logger.Info("Shutting down STDIO server...")
		cancel()
		errCh <- nil
	}

	// Wait for server to exit or error
	return <-errCh
}

// registerNpmTool registers the npm version checking tool
func (s *PackageVersionServer) registerNpmTool(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering NPM version checking tool")

	// Create NPM handler
	npmHandler := handlers.NewNpmHandler(s.logger, s.sharedCache)

	// Add NPM tool
	npmTool := mcp.NewTool("check_npm_versions",
		mcp.WithDescription("Check latest stable versions for npm packages"),
		mcp.WithObject("dependencies",
			mcp.Required(),
			mcp.Description("Dependencies object from package.json"),
		),
		mcp.WithObject("constraints",
			mcp.Description("Optional constraints for specific packages"),
		),
	)

	// Add NPM handler
	srv.AddTool(npmTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_npm_versions").Info("Received request")
		return npmHandler.GetLatestVersion(ctx, request.Params.Arguments)
	})
}

// registerPythonTools registers the Python version checking tools
func (s *PackageVersionServer) registerPythonTools(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering Python version checking tools")

	// Create Python handler
	pythonHandler := handlers.NewPythonHandler(s.logger, s.sharedCache)

	// Tool for requirements.txt
	pythonTool := mcp.NewTool("check_python_versions",
		mcp.WithDescription("Check latest stable versions for Python packages"),
		mcp.WithArray("requirements",
			mcp.Required(),
			mcp.Description("Array of requirements from requirements.txt"),
		),
	)

	// Add Python requirements.txt handler
	srv.AddTool(pythonTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_python_versions").Info("Received request")
		return pythonHandler.GetLatestVersionFromRequirements(ctx, request.Params.Arguments)
	})

	// Tool for pyproject.toml
	pyprojectTool := mcp.NewTool("check_pyproject_versions",
		mcp.WithDescription("Check latest stable versions for Python packages in pyproject.toml"),
		mcp.WithObject("dependencies",
			mcp.Required(),
			mcp.Description("Dependencies object from pyproject.toml"),
		),
	)

	// Add Python pyproject.toml handler
	srv.AddTool(pyprojectTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_pyproject_versions").Info("Received request")
		return pythonHandler.GetLatestVersionFromPyProject(ctx, request.Params.Arguments)
	})
}

// registerJavaTools registers the Java version checking tools
func (s *PackageVersionServer) registerJavaTools(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering Java version checking tools")

	// Create Java handler
	javaHandler := handlers.NewJavaHandler(s.logger, s.sharedCache)

	// Tool for Maven
	mavenTool := mcp.NewTool("check_maven_versions",
		mcp.WithDescription("Check latest stable versions for Java packages in pom.xml"),
		mcp.WithArray("dependencies",
			mcp.Required(),
			mcp.Description("Array of Maven dependencies"),
		),
	)

	// Add Maven handler
	srv.AddTool(mavenTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_maven_versions").Info("Received request")
		return javaHandler.GetLatestVersionFromMaven(ctx, request.Params.Arguments)
	})

	// Tool for Gradle
	gradleTool := mcp.NewTool("check_gradle_versions",
		mcp.WithDescription("Check latest stable versions for Java packages in build.gradle"),
		mcp.WithArray("dependencies",
			mcp.Required(),
			mcp.Description("Array of Gradle dependencies"),
		),
	)

	// Add Gradle handler
	srv.AddTool(gradleTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_gradle_versions").Info("Received request")
		return javaHandler.GetLatestVersionFromGradle(ctx, request.Params.Arguments)
	})
}

// registerGoTool registers the Go version checking tool
func (s *PackageVersionServer) registerGoTool(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering Go version checking tool")

	// Create Go handler
	goHandler := handlers.NewGoHandler(s.logger, s.sharedCache)

	goTool := mcp.NewTool("check_go_versions",
		mcp.WithDescription("Check latest stable versions for Go packages in go.mod"),
		mcp.WithObject("dependencies",
			mcp.Required(),
			mcp.Description("Dependencies from go.mod"),
		),
	)

	// Add Go handler
	srv.AddTool(goTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_go_versions").Info("Received request")
		return goHandler.GetLatestVersion(ctx, request.Params.Arguments)
	})
}

// registerBedrockTools registers the AWS Bedrock tools
func (s *PackageVersionServer) registerBedrockTools(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering AWS Bedrock tools")

	// Create Bedrock handler
	bedrockHandler := handlers.NewBedrockHandler(s.logger, s.sharedCache)

	// Tool for searching Bedrock models
	bedrockTool := mcp.NewTool("check_bedrock_models",
		mcp.WithDescription("Search, list, and get information about Amazon Bedrock models"),
		mcp.WithString("action",
			mcp.Description("Action to perform: list all models, search for models, or get a specific model"),
			mcp.Enum("list", "search", "get"),
			mcp.DefaultString("list"),
		),
		mcp.WithString("query",
			mcp.Description("Search query for model name or ID (used with action: \"search\")"),
		),
		mcp.WithString("provider",
			mcp.Description("Filter by provider name (used with action: \"search\")"),
		),
		mcp.WithString("region",
			mcp.Description("Filter by AWS region (used with action: \"search\")"),
		),
		mcp.WithString("modelId",
			mcp.Description("Model ID to retrieve (used with action: \"get\")"),
		),
	)

	// Add Bedrock handler
	srv.AddTool(bedrockTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithFields(logrus.Fields{
			"tool":   "check_bedrock_models",
			"action": request.Params.Arguments["action"],
		}).Info("Received request")
		return bedrockHandler.GetLatestVersion(ctx, request.Params.Arguments)
	})

	// Tool for getting the latest Claude Sonnet model
	sonnetTool := mcp.NewTool("get_latest_bedrock_model",
		mcp.WithDescription("Get the latest Claude Sonnet model from Amazon Bedrock (best for coding tasks)"),
	)

	// Add Bedrock Claude Sonnet handler
	srv.AddTool(sonnetTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "get_latest_bedrock_model").Info("Received request")
		// Set the action to get_latest_claude_sonnet to use the specialized method
		return bedrockHandler.GetLatestVersion(ctx, map[string]interface{}{
			"action": "get_latest_claude_sonnet",
		})
	})
}

// registerDockerTool registers the Docker version checking tool
func (s *PackageVersionServer) registerDockerTool(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering Docker version checking tool")

	// Create Docker handler
	dockerHandler := handlers.NewDockerHandler(s.logger, s.sharedCache)

	dockerTool := mcp.NewTool("check_docker_tags",
		mcp.WithDescription("Check available tags for Docker container images from Docker Hub, GitHub Container Registry, or custom registries"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Docker image name (e.g., \"nginx\", \"ubuntu\", \"ghcr.io/owner/repo\")"),
		),
		mcp.WithString("registry",
			mcp.Description("Registry to check (dockerhub, ghcr, or custom)"),
			mcp.Enum("dockerhub", "ghcr", "custom"),
			mcp.DefaultString("dockerhub"),
		),
		mcp.WithString("customRegistry",
			mcp.Description("URL for custom registry (required when registry is \"custom\")"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of tags to return"),
			mcp.DefaultNumber(10),
		),
		mcp.WithArray("filterTags",
			mcp.Description("Array of regex patterns to filter tags"),
		),
		mcp.WithBoolean("includeDigest",
			mcp.Description("Include image digest in results"),
			mcp.DefaultBool(false),
		),
	)

	// Add Docker handler
	srv.AddTool(dockerTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithFields(logrus.Fields{
			"tool":     "check_docker_tags",
			"image":    request.Params.Arguments["image"],
			"registry": request.Params.Arguments["registry"],
		}).Info("Received request")
		return dockerHandler.GetLatestVersion(ctx, request.Params.Arguments)
	})
}

// registerSwiftTool registers the Swift version checking tool
func (s *PackageVersionServer) registerSwiftTool(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering Swift version checking tool")

	// Create Swift handler
	swiftHandler := handlers.NewSwiftHandler(s.logger, s.sharedCache)

	swiftTool := mcp.NewTool("check_swift_versions",
		mcp.WithDescription("Check latest stable versions for Swift packages in Package.swift"),
		mcp.WithArray("dependencies",
			mcp.Required(),
			mcp.Description("Array of Swift package dependencies"),
		),
		mcp.WithObject("constraints",
			mcp.Description("Optional constraints for specific packages"),
		),
	)

	// Add Swift handler
	srv.AddTool(swiftTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_swift_versions").Info("Received request")
		return swiftHandler.GetLatestVersion(ctx, request.Params.Arguments)
	})
}

// registerGitHubActionsTool registers the GitHub Actions version checking tool
func (s *PackageVersionServer) registerGitHubActionsTool(srv *mcpserver.MCPServer) {
	s.logger.Info("Registering GitHub Actions version checking tool")

	// Create GitHub Actions handler
	githubActionsHandler := handlers.NewGitHubActionsHandler(s.logger, s.sharedCache)

	githubActionsTool := mcp.NewTool("check_github_actions",
		mcp.WithDescription("Check latest versions for GitHub Actions"),
		mcp.WithArray("actions",
			mcp.Required(),
			mcp.Description("Array of GitHub Actions to check"),
		),
		mcp.WithBoolean("includeDetails",
			mcp.Description("Include additional details like published date and URL"),
			mcp.DefaultBool(false),
		),
	)

	// Add GitHub Actions handler
	srv.AddTool(githubActionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.WithField("tool", "check_github_actions").Info("Received request")
		return githubActionsHandler.GetLatestVersion(ctx, request.Params.Arguments)
	})
}
