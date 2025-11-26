package proxy

import (
	"context"
	"fmt"
	"sync"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/aggregator"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy/types"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy/upstream"
	"github.com/sirupsen/logrus"
)

// ProxyManager is a singleton that manages upstream connections and is shared across all dynamic tools.
type ProxyManager struct {
	config       *types.ProxyConfig
	manager      *upstream.Manager
	aggregator   *aggregator.Aggregator
	initialised  bool
	initialiseMu sync.Mutex
}

var (
	globalProxyManager     *ProxyManager
	globalProxyManagerOnce sync.Once
)

// GetGlobalProxyManager returns the singleton proxy manager instance.
func GetGlobalProxyManager() *ProxyManager {
	globalProxyManagerOnce.Do(func() {
		globalProxyManager = &ProxyManager{}
	})
	return globalProxyManager
}

// EnsureInitialised ensures the proxy manager is initialised (lazy initialisation).
func (pm *ProxyManager) EnsureInitialised(ctx context.Context, logger *logrus.Logger) error {
	pm.initialiseMu.Lock()
	defer pm.initialiseMu.Unlock()

	if pm.initialised {
		return nil
	}

	logger.Info("initialising proxy manager")

	// Parse configuration
	config, err := ParseConfig()
	if err != nil {
		return fmt.Errorf("failed to parse proxy configuration: %w", err)
	}

	pm.config = config

	// Ensure cache directory exists
	if err := EnsureCacheDir(config.CacheDir); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create manager and aggregator
	pm.manager = upstream.NewManager(config)
	pm.aggregator = aggregator.NewAggregator(config)

	// Connect to upstreams
	logger.Info("connecting to upstream servers")
	if err := pm.manager.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to upstreams: %w", err)
	}

	// Aggregate tools
	allTools := pm.manager.GetAllTools()
	pm.aggregator.AggregateTools(allTools)

	pm.initialised = true
	logger.WithField("count", len(pm.aggregator.GetTools())).Info("proxy manager initialised")

	return nil
}

// GetManager returns the upstream manager (after initialisation).
func (pm *ProxyManager) GetManager() *upstream.Manager {
	return pm.manager
}

// GetAggregator returns the tool aggregator (after initialisation).
func (pm *ProxyManager) GetAggregator() *aggregator.Aggregator {
	return pm.aggregator
}

// IsInitialised returns whether the manager has been initialised.
func (pm *ProxyManager) IsInitialised() bool {
	pm.initialiseMu.Lock()
	defer pm.initialiseMu.Unlock()
	return pm.initialised
}
