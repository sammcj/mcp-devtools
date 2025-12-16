package webfetch

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantURL      string
		wantFragment string
	}{
		{
			name:         "URL with fragment",
			input:        "https://example.com/page#section",
			wantURL:      "https://example.com/page",
			wantFragment: "section",
		},
		{
			name:         "URL without fragment",
			input:        "https://example.com/page",
			wantURL:      "https://example.com/page",
			wantFragment: "",
		},
		{
			name:         "URL with query and fragment",
			input:        "https://example.com/page?key=value#section",
			wantURL:      "https://example.com/page?key=value",
			wantFragment: "section",
		},
		{
			name:         "URL with empty fragment",
			input:        "https://example.com/page#",
			wantURL:      "https://example.com/page",
			wantFragment: "",
		},
		{
			name:         "URL with encoded fragment",
			input:        "https://example.com/page#my%20section",
			wantURL:      "https://example.com/page",
			wantFragment: "my section",
		},
		{
			name:         "URL with port and fragment",
			input:        "https://example.com:8080/page#section",
			wantURL:      "https://example.com:8080/page",
			wantFragment: "section",
		},
		{
			name:         "URL with complex path and fragment",
			input:        "https://docs.example.com/api/v2/reference#authentication",
			wantURL:      "https://docs.example.com/api/v2/reference",
			wantFragment: "authentication",
		},
		{
			name:         "URL with multiple query params and fragment",
			input:        "https://example.com/search?q=test&page=1#results",
			wantURL:      "https://example.com/search?q=test&page=1",
			wantFragment: "results",
		},
		{
			name:         "HTTP URL with fragment",
			input:        "http://example.com/page#section",
			wantURL:      "http://example.com/page",
			wantFragment: "section",
		},
		{
			name:         "URL with hyphenated fragment",
			input:        "https://example.com/docs#getting-started",
			wantURL:      "https://example.com/docs",
			wantFragment: "getting-started",
		},
		{
			name:         "URL with underscore fragment",
			input:        "https://example.com/docs#api_reference",
			wantURL:      "https://example.com/docs",
			wantFragment: "api_reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseURL(tt.input)

			if result.URLWithoutFragment != tt.wantURL {
				t.Errorf("URLWithoutFragment = %q, want %q", result.URLWithoutFragment, tt.wantURL)
			}

			if result.Fragment != tt.wantFragment {
				t.Errorf("Fragment = %q, want %q", result.Fragment, tt.wantFragment)
			}
		})
	}
}
