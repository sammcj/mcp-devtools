package tools

import (
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/aggregator"
)

func TestFilter_IgnoreOnly(t *testing.T) {
	filter := aggregator.NewFilter(nil, []string{"debug_*", "admin_*"})

	tests := []struct {
		toolName string
		expected bool
	}{
		{"get_user", true},
		{"debug_log", false},
		{"admin_delete", false},
		{"search_items", true},
		{"debug_trace", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := filter.ShouldInclude(tt.toolName)
			if result != tt.expected {
				t.Errorf("ShouldInclude(%s) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestFilter_IncludeOnly(t *testing.T) {
	filter := aggregator.NewFilter([]string{"get_*", "search_*"}, nil)

	tests := []struct {
		toolName string
		expected bool
	}{
		{"get_user", true},
		{"get_items", true},
		{"search_products", true},
		{"create_user", false},
		{"delete_item", false},
		{"update_status", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := filter.ShouldInclude(tt.toolName)
			if result != tt.expected {
				t.Errorf("ShouldInclude(%s) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestFilter_IncludeAndIgnore(t *testing.T) {
	// Include search* and get*, but exclude *_debug
	filter := aggregator.NewFilter([]string{"search_*", "get_*"}, []string{"*_debug"})

	tests := []struct {
		toolName string
		expected bool
	}{
		{"get_user", true},
		{"search_items", true},
		{"get_debug", false},     // Matches include but also matches ignore
		{"search_debug", false},  // Matches include but also matches ignore
		{"create_user", false},   // Doesn't match include
		{"delete_debug", false},  // Doesn't match include
		{"get_products", true},   // Matches include and not ignore
		{"search_records", true}, // Matches include and not ignore
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := filter.ShouldInclude(tt.toolName)
			if result != tt.expected {
				t.Errorf("ShouldInclude(%s) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestFilter_NoPatterns(t *testing.T) {
	filter := aggregator.NewFilter(nil, nil)

	tests := []string{
		"any_tool",
		"get_user",
		"debug_log",
		"admin_delete",
	}

	for _, toolName := range tests {
		t.Run(toolName, func(t *testing.T) {
			result := filter.ShouldInclude(toolName)
			if !result {
				t.Errorf("ShouldInclude(%s) = false, want true (no patterns should include all)", toolName)
			}
		})
	}
}

func TestFilter_ComplexPatterns(t *testing.T) {
	filter := aggregator.NewFilter(
		[]string{"*user*", "*account*"},
		[]string{"*delete*", "*remove*"},
	)

	tests := []struct {
		toolName string
		expected bool
	}{
		{"get_user_info", true},
		{"create_user", true},
		{"delete_user", false},    // Matches include but also matches ignore
		{"remove_account", false}, // Matches include but also matches ignore
		{"list_accounts", true},
		{"get_product", false}, // Doesn't match include
		{"delete_item", false}, // Doesn't match include
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := filter.ShouldInclude(tt.toolName)
			if result != tt.expected {
				t.Errorf("ShouldInclude(%s) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestFilter_CaseInsensitive(t *testing.T) {
	filter := aggregator.NewFilter(nil, []string{"*jira*", "*update*"})

	tests := []struct {
		toolName string
		expected bool
	}{
		{"getJiraIssue", false},        // Should match *jira* (case-insensitive)
		{"transitionJiraIssue", false}, // Should match *jira* (case-insensitive)
		{"getJIRATicket", false},       // Should match *jira* (case-insensitive)
		{"updateUser", false},          // Should match *update* (case-insensitive)
		{"UpdateItem", false},          // Should match *update* (case-insensitive)
		{"getUser", true},              // Should not match
		{"searchItems", true},          // Should not match
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := filter.ShouldInclude(tt.toolName)
			if result != tt.expected {
				t.Errorf("ShouldInclude(%s) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestFilter_RealWorldAtlassian(t *testing.T) {
	// Simulate the user's actual configuration
	filter := aggregator.NewFilter(nil, []string{"*update*", "*create*", "*add*", "*edit*", "*jira*"})

	tests := []struct {
		toolName string
		expected bool
	}{
		{"getJiraIssue", false},
		{"transitionJiraIssue", false},
		{"createConfluencePage", false},
		{"updateJiraIssue", false},
		{"addComment", false},
		{"editPage", false},
		{"searchConfluence", true},
		{"getUser", true},
		{"listProjects", true},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := filter.ShouldInclude(tt.toolName)
			if result != tt.expected {
				t.Errorf("ShouldInclude(%s) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}
