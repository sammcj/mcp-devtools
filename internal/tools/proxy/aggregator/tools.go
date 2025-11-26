package aggregator

import (
	"fmt"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/types"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy/upstream"
	"github.com/sirupsen/logrus"
)

// AggregatedTool represents a tool with its upstream origin.
type AggregatedTool struct {
	Name         string // Potentially prefixed with upstream name
	OriginalName string // Original tool name from upstream
	UpstreamName string
	Description  string
	InputSchema  any
}

// Aggregator combines and manages tools from multiple upstreams.
type Aggregator struct {
	config  *types.ProxyConfig
	filters map[string]*Filter // Per-upstream filters
	tools   []AggregatedTool
	toolMap map[string]string // Maps tool name to upstream name
}

// NewAggregator creates a new tool aggregator.
func NewAggregator(config *types.ProxyConfig) *Aggregator {
	agg := &Aggregator{
		config:  config,
		filters: make(map[string]*Filter),
		toolMap: make(map[string]string),
	}

	// Create filters for each upstream
	for i := range config.Upstreams {
		upstream := &config.Upstreams[i]
		if len(upstream.IncludeTools) > 0 || len(upstream.IgnoreTools) > 0 {
			agg.filters[upstream.Name] = NewFilter(upstream.IncludeTools, upstream.IgnoreTools)
			logrus.WithFields(logrus.Fields{
				"upstream":      upstream.Name,
				"include_count": len(upstream.IncludeTools),
				"ignore_count":  len(upstream.IgnoreTools),
			}).Debug("created filter for upstream")
		}
	}

	return agg
}

// AggregateTools combines tools from all upstreams, applying filters and handling name conflicts.
func (agg *Aggregator) AggregateTools(allTools map[string][]upstream.ToolInfo) []AggregatedTool {
	var aggregated []AggregatedTool
	toolCounts := make(map[string]int) // Track how many upstreams have each tool name

	// First pass: count occurrences of each tool name across all upstreams
	for _, tools := range allTools {
		for _, tool := range tools {
			toolCounts[tool.Name]++
		}
	}

	// Second pass: aggregate tools with appropriate naming
	for upstreamName, tools := range allTools {
		filter := agg.filters[upstreamName]

		for _, tool := range tools {
			// Apply filter if configured for this upstream
			if filter != nil && !filter.ShouldInclude(tool.Name) {
				logrus.WithFields(logrus.Fields{
					"upstream": upstreamName,
					"tool":     tool.Name,
				}).Debug("tool filtered out")
				continue
			}

			// Determine final tool name (with or without prefix)
			finalName := tool.Name
			needsPrefix := false

			// Add prefix if there's a name conflict or if there are multiple upstreams
			if toolCounts[tool.Name] > 1 || len(allTools) > 1 {
				needsPrefix = true
				finalName = fmt.Sprintf("%s:%s", upstreamName, tool.Name)
			}

			aggregated = append(aggregated, AggregatedTool{
				Name:         finalName,
				OriginalName: tool.Name,
				UpstreamName: upstreamName,
				Description:  tool.Description,
				InputSchema:  tool.InputSchema,
			})

			// Store mapping
			agg.toolMap[finalName] = upstreamName

			// Also store unprefixed version if no conflict
			if !needsPrefix {
				agg.toolMap[tool.Name] = upstreamName
			}

			logrus.WithFields(logrus.Fields{
				"upstream":      upstreamName,
				"original_name": tool.Name,
				"final_name":    finalName,
				"prefixed":      needsPrefix,
			}).Debug("aggregated tool")
		}
	}

	agg.tools = aggregated

	logrus.WithFields(logrus.Fields{
		"total":     len(aggregated),
		"upstreams": len(allTools),
	}).Info("aggregated tools from upstreams")

	return aggregated
}

// GetTools returns all aggregated tools.
func (agg *Aggregator) GetTools() []AggregatedTool {
	return agg.tools
}

// GetUpstreamForTool returns the upstream name for a given tool.
func (agg *Aggregator) GetUpstreamForTool(toolName string) (string, error) {
	upstreamName, ok := agg.toolMap[toolName]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}
	return upstreamName, nil
}

// GetOriginalToolName extracts the original tool name from a potentially prefixed name.
func (agg *Aggregator) GetOriginalToolName(toolName string) string {
	// Check if tool name has upstream prefix by checking against configured upstreams
	for i := range agg.config.Upstreams {
		upstreamName := agg.config.Upstreams[i].Name
		prefix := upstreamName + ":"
		if len(toolName) > len(prefix) && toolName[:len(prefix)] == prefix {
			return toolName[len(prefix):]
		}
	}
	// No prefix found, return as-is
	return toolName
}
