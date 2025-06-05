package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	// Import all tool packages to register them
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/bedrock"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/docker"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/githubactions"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/go"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/java"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/npm"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/python"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/swift"
	_ "github.com/sammcj/mcp-devtools/internal/tools/pythonexec" // Import the pythonexec tool package
	_ "github.com/sammcj/mcp-devtools/internal/tools/shadcnui"   // Import for shadcnui tools
)

// Version information (set during build)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	// Create a logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Initialize the registry
	registry.Init(logger)

	// Create and run the CLI app
	app := &cli.App{
		Name:    "mcp-devtools",
		Usage:   "MCP server for developer tools",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "transport",
				Aliases: []string{"t"},
				Value:   "stdio",
				Usage:   "Transport type (stdio or sse)",
			},
			&cli.StringFlag{
				Name:  "port",
				Value: "18080",
				Usage: "Port to use for SSE transport",
			},
			&cli.StringFlag{
				Name:  "base-url",
				Value: "http://localhost",
				Usage: "Base URL for SSE transport",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Value:   false,
				Usage:   "Enable debug logging",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Print version information",
				Action: func(c *cli.Context) error {
					fmt.Printf("mcp-devtools version %s\n", Version)
					fmt.Printf("Commit: %s\n", Commit)
					fmt.Printf("Built: %s\n", BuildDate)
					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			// Set debug level if requested
			if c.Bool("debug") {
				logger.SetLevel(logrus.DebugLevel)
				logger.Debug("Debug logging enabled")
			}

			// Get transport settings
			transport := c.String("transport")
			port := c.String("port")
			baseURL := c.String("base-url")

			// Log version information
			logger.Infof("Starting mcp-devtools version %s (commit: %s, built: %s)",
				Version, Commit, BuildDate)

			// Create MCP server
			server := mcpserver.NewMCPServer("mcp-devtools", "MCP DevTools Server")

			// Register tools
			for name, tool := range registry.GetTools() {
				logger.Infof("Registering tool: %s", name)
				server.AddTool(tool.Definition(), func(toolCtx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					// Get tool from registry
					tool, ok := registry.GetTool(name)
					if !ok {
						return nil, fmt.Errorf("tool not found: %s", name)
					}

				// Execute tool
				logger.Infof("Executing tool: %s", name)

				// Type assert the arguments to map[string]interface{}
				args, ok := request.Params.Arguments.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}, got %T", request.Params.Arguments)
				}

				return tool.Execute(toolCtx, registry.GetLogger(), registry.GetCache(), args)
				})
			}

			// Start the server
			switch transport {
			case "stdio":
				return mcpserver.ServeStdio(server)
			case "sse":
				sseServer := mcpserver.NewSSEServer(server, mcpserver.WithBaseURL(baseURL))
				return sseServer.Start(":" + port)
			default:
				return fmt.Errorf("unsupported transport: %s", transport)
			}
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatalf("Error: %v", err)
	}
}
