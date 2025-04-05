package main

import (
	"fmt"
	"os"

	"github.com/sammcj/mcp-package-version-go/pkg/server"
	"github.com/sammcj/mcp-package-version-go/pkg/version"
	"github.com/urfave/cli/v2"
)

func main() {
	// Print version information directly
	fmt.Printf("Direct version info: %s, %s, %s\n", version.Version, version.Commit, version.BuildDate)

	// Create a new package version server
	packageVersionServer := server.NewPackageVersionServer(version.Version, version.Commit, version.BuildDate)

	// Create and run the CLI app
	app := &cli.App{
		Name:    "mcp-package-version-go",
		Usage:   "MCP server for checking package versions",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.BuildDate),
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
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Print version information",
				Action: func(c *cli.Context) error {
					fmt.Printf("mcp-package-version-go version %s\n", version.Version)
					fmt.Printf("Commit: %s\n", version.Commit)
					fmt.Printf("Built: %s\n", version.BuildDate)
					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			transport := c.String("transport")
			port := c.String("port")
			baseURL := c.String("base-url")

			// Log version information
			fmt.Printf("Starting mcp-package-version-go version %s (commit: %s, built: %s)\n",
				version.Version, version.Commit, version.BuildDate)

			// Start the MCP server with the specified transport
			return packageVersionServer.Start(transport, port, baseURL)
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
