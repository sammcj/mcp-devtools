//go:build cgo && (darwin || (linux && amd64))

package imports

import (
	// codeskim - only available on supported platforms
	_ "github.com/sammcj/mcp-devtools/internal/tools/codeskim"
)
