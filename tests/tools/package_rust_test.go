package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/rust"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRustTool_Execute(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping Rust tool test in short mode")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	tool := rust.NewRustTool(nil)

	tests := []struct {
		name     string
		args     map[string]any
		wantErr  bool
		validate func(t *testing.T, result any)
	}{
		{
			name: "single crate",
			args: map[string]any{
				"dependencies": map[string]any{
					"serde": "1.0",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result any) {
				versions := testutils.ExtractPackageVersions(t, result)
				require.Len(t, versions, 1)
				assert.Equal(t, "serde", versions[0].Name)
				assert.Equal(t, "crates.io", versions[0].Registry)
				assert.NotEmpty(t, versions[0].LatestVersion)
			},
		},
		{
			name: "multiple crates",
			args: map[string]any{
				"dependencies": map[string]any{
					"serde": "1.0",
					"tokio": "1.0",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result any) {
				versions := testutils.ExtractPackageVersions(t, result)
				require.Len(t, versions, 2)

				// Results should be sorted by name
				assert.Equal(t, "serde", versions[0].Name)
				assert.Equal(t, "tokio", versions[1].Name)

				for _, v := range versions {
					assert.Equal(t, "crates.io", v.Registry)
					assert.NotEmpty(t, v.LatestVersion)
				}
			},
		},
		{
			name: "complex dependency format",
			args: map[string]any{
				"dependencies": map[string]any{
					"clap": map[string]any{
						"version":  "4.0",
						"features": []string{"derive"},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result any) {
				versions := testutils.ExtractPackageVersions(t, result)
				require.Len(t, versions, 1)
				assert.Equal(t, "clap", versions[0].Name)
				assert.Equal(t, "crates.io", versions[0].Registry)
				assert.NotEmpty(t, versions[0].LatestVersion)
			},
		},
		{
			name: "nonexistent crate",
			args: map[string]any{
				"dependencies": map[string]any{
					"nonexistent-crate-12345": "1.0",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result any) {
				versions := testutils.ExtractPackageVersions(t, result)
				require.Len(t, versions, 1)
				assert.Equal(t, "nonexistent-crate-12345", versions[0].Name)
				assert.True(t, versions[0].Skipped)
				assert.Contains(t, versions[0].SkipReason, "Failed to fetch crate info")
			},
		},
		{
			name:    "missing dependencies",
			args:    map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), logger, cache, tt.args)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestRustTool_Definition(t *testing.T) {
	tool := rust.NewRustTool(nil)
	def := tool.Definition()

	assert.Equal(t, "check_rust_versions", def.Name)
	assert.Contains(t, def.Description, "Rust crates")
	assert.Contains(t, def.InputSchema.Properties, "dependencies")
}
