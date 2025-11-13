package imports

import (
	// Standard tools - always available
	_ "github.com/sammcj/mcp-devtools/internal/tools/api"
	_ "github.com/sammcj/mcp-devtools/internal/tools/aws_documentation"
	_ "github.com/sammcj/mcp-devtools/internal/tools/calculator"
	_ "github.com/sammcj/mcp-devtools/internal/tools/claudeagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/code_rename"
	// codeskim is conditionally imported in tools_codeskim.go based on platform support
	_ "github.com/sammcj/mcp-devtools/internal/tools/codexagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/copilotagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/docprocessing"
	_ "github.com/sammcj/mcp-devtools/internal/tools/excel"
	_ "github.com/sammcj/mcp-devtools/internal/tools/filelength"
	_ "github.com/sammcj/mcp-devtools/internal/tools/filesystem"
	_ "github.com/sammcj/mcp-devtools/internal/tools/geminiagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/github"
	_ "github.com/sammcj/mcp-devtools/internal/tools/internetsearch/unified"
	_ "github.com/sammcj/mcp-devtools/internal/tools/m2e"
	_ "github.com/sammcj/mcp-devtools/internal/tools/memory"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packagedocs"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/unified"
	_ "github.com/sammcj/mcp-devtools/internal/tools/pdf"
	_ "github.com/sammcj/mcp-devtools/internal/tools/qdeveloperagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/securityoverride"
	_ "github.com/sammcj/mcp-devtools/internal/tools/sequentialthinking"
	_ "github.com/sammcj/mcp-devtools/internal/tools/shadcnui"
	_ "github.com/sammcj/mcp-devtools/internal/tools/terraform_documentation"
	_ "github.com/sammcj/mcp-devtools/internal/tools/think"
	_ "github.com/sammcj/mcp-devtools/internal/tools/utilities/toolhelp"
	_ "github.com/sammcj/mcp-devtools/internal/tools/webfetch"
)
