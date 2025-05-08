package registry

import (
	"sync"

	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

var (
	// toolRegistry is a map of tool names to tool implementations
	toolRegistry = make(map[string]tools.Tool) // Initialize here

	// logger is the shared logger instance
	logger *logrus.Logger

	// cache is the shared cache instance
	cache *sync.Map
)

// Init initialises the registry and shared resources
func Init(l *logrus.Logger) {
	logger = l
	cache = &sync.Map{}
	// toolRegistry = make(map[string]tools.Tool) // REMOVED: toolRegistry is initialized at declaration
}

// Register adds a tool implementation to the registry
func Register(tool tools.Tool) {
	// No need to check for nil if toolRegistry is initialized at declaration.
	// If it could somehow be nil due to other logic, the check can remain,
	// but the primary initialization is now at var declaration.
	// For safety, keeping the nil check might be okay, but it shouldn't be hit.
	if toolRegistry == nil { // This should ideally not be necessary now
		toolRegistry = make(map[string]tools.Tool)
	}
	toolRegistry[tool.Definition().Name] = tool
}

// GetTool retrieves a tool by name
func GetTool(name string) (tools.Tool, bool) {
	tool, ok := toolRegistry[name]
	return tool, ok
}

// GetTools returns all registered tools
func GetTools() map[string]tools.Tool {
	return toolRegistry
}

// GetLogger returns the shared logger instance
func GetLogger() *logrus.Logger {
	return logger
}

// GetCache returns the shared cache instance
func GetCache() *sync.Map {
	return cache
}
