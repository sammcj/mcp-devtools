//go:build cgo && (darwin || (linux && amd64))

package imports

// Import codesearch tool for registration (CGO-only)
import _ "github.com/sammcj/mcp-devtools/internal/tools/codesearch"
