package upstream

import (
	"context"
	"fmt"
	"sync"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/types"
	"github.com/sirupsen/logrus"
)

// Manager manages connections to multiple upstream MCP servers.
type Manager struct {
	connections map[string]*Connection
	config      *types.ProxyConfig
	mu          sync.RWMutex
}

// NewManager creates a new upstream manager.
func NewManager(config *types.ProxyConfig) *Manager {
	return &Manager{
		connections: make(map[string]*Connection),
		config:      config,
	}
}

// Connect establishes connections to all configured upstreams.
func (m *Manager) Connect(ctx context.Context) error {
	logrus.WithField("count", len(m.config.Upstreams)).Info("connecting to upstream servers")

	// Connect to each upstream concurrently
	var wg sync.WaitGroup
	errors := make(chan error, len(m.config.Upstreams))

	for i := range m.config.Upstreams {
		upstream := &m.config.Upstreams[i]
		wg.Add(1)

		go func(upstreamConfig *types.UpstreamConfig) {
			defer wg.Done()

			conn, err := NewConnection(upstreamConfig, m.config.CacheDir, m.config.CallbackPort)
			if err != nil {
				errors <- fmt.Errorf("failed to create connection to %s: %w", upstreamConfig.Name, err)
				return
			}

			if err := conn.Connect(ctx); err != nil {
				logrus.WithError(err).WithField("name", upstreamConfig.Name).Warn("failed to connect to upstream (will continue with other upstreams)")
				errors <- fmt.Errorf("failed to connect to %s: %w", upstreamConfig.Name, err)
				return
			}

			// Fetch tools from this upstream
			if err := conn.FetchTools(ctx); err != nil {
				logrus.WithError(err).WithField("name", upstreamConfig.Name).Error("failed to fetch tools from upstream")
				// Don't treat this as fatal - we can still use the connection
			} else {
				tools := conn.GetTools()
				logrus.WithFields(logrus.Fields{
					"name":  upstreamConfig.Name,
					"count": len(tools),
				}).Info("successfully fetched tools from upstream")
			}

			m.mu.Lock()
			m.connections[upstreamConfig.Name] = conn
			m.mu.Unlock()

			logrus.WithField("name", upstreamConfig.Name).Info("upstream connection established")
		}(upstream)
	}

	wg.Wait()
	close(errors)

	// Collect all errors
	var allErrors []error
	for err := range errors {
		allErrors = append(allErrors, err)
	}

	m.mu.RLock()
	connectedCount := len(m.connections)
	m.mu.RUnlock()

	if connectedCount == 0 {
		return fmt.Errorf("failed to connect to any upstream servers: %d errors", len(allErrors))
	}

	if len(allErrors) > 0 {
		logrus.WithFields(logrus.Fields{
			"connected": connectedCount,
			"total":     len(m.config.Upstreams),
			"errors":    len(allErrors),
		}).Warn("some upstream connections failed")
	}

	logrus.WithField("count", connectedCount).Info("upstream connections established")
	return nil
}

// GetConnection returns the connection for a specific upstream by name.
func (m *Manager) GetConnection(name string) (*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.connections[name]
	if !ok {
		return nil, fmt.Errorf("upstream not found: %s", name)
	}

	return conn, nil
}

// GetAllTools returns all tools from all upstreams.
func (m *Manager) GetAllTools() map[string][]ToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allTools := make(map[string][]ToolInfo)
	for name, conn := range m.connections {
		allTools[name] = conn.GetTools()
	}

	return allTools
}

// ExecuteTool executes a tool on the appropriate upstream.
// The toolName should be in the format "upstream:tool" or just "tool" for single upstream.
func (m *Manager) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (*Message, error) {
	// Parse tool name to extract upstream and tool
	upstreamName, actualToolName := m.parseToolName(toolName)

	conn, err := m.GetConnection(upstreamName)
	if err != nil {
		return nil, err
	}

	return conn.ExecuteTool(ctx, actualToolName, args)
}

// parseToolName extracts the upstream name and tool name from a potentially prefixed tool name.
// Format: "upstream:tool" or just "tool" (uses first available upstream).
func (m *Manager) parseToolName(toolName string) (upstreamName string, actualToolName string) {
	// Hold lock for entire function to prevent race conditions
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if tool name contains upstream prefix
	for name := range m.connections {
		prefix := name + ":"
		if len(toolName) > len(prefix) && toolName[:len(prefix)] == prefix {
			return name, toolName[len(prefix):]
		}
	}

	// No prefix found - use first available upstream (or default if only one)
	if len(m.connections) == 1 {
		for name := range m.connections {
			return name, toolName
		}
	}

	// Multiple upstreams but no prefix - this is ambiguous
	// Return empty upstream name to trigger an error
	return "", toolName
}

// Close closes all upstream connections.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for name, conn := range m.connections {
		if err := conn.Close(); err != nil {
			logrus.WithError(err).WithField("name", name).Error("failed to close upstream connection")
			errors = append(errors, err)
		}
	}

	m.connections = make(map[string]*Connection)

	if len(errors) > 0 {
		return fmt.Errorf("failed to close %d connections", len(errors))
	}

	logrus.Info("all upstream connections closed")
	return nil
}
