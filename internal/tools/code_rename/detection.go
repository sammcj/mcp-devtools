package code_rename

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// DetectLanguage determines the language from a file path
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".py", ".pyi":
		return "python"
	case ".rs":
		return "rust"
	case ".sh", ".bash":
		return "bash"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	case ".less":
		return "less"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".swift":
		return "swift"
	default:
		return ""
	}
}

// FindServerForLanguage finds an available LSP server for the given language
func FindServerForLanguage(ctx context.Context, logger *logrus.Logger, language string) (*LanguageServer, error) {
	cache := GetServerCache()

	// Find all servers that support this language
	var candidates []LanguageServer
	for _, server := range SupportedServers {
		if server.Language == language {
			candidates = append(candidates, server)
		}
	}

	if len(candidates) == 0 {
		logger.WithField("language", language).Debug("No LSP servers configured for language")
		return nil, nil
	}

	// Check each candidate to see if it's available
	for _, server := range candidates {
		// Check cache first
		if available, exists := cache.IsAvailable(server.Command); exists {
			if available {
				logger.WithFields(logrus.Fields{
					"language": language,
					"command":  server.Command,
				}).Debug("Found cached available LSP server")
				return &server, nil
			}
			continue
		}

		// Not in cache, check availability
		if isCommandAvailable(ctx, server.Command) {
			cache.SetAvailable(server.Command, true)
			logger.WithFields(logrus.Fields{
				"language": language,
				"command":  server.Command,
			}).Debug("Found available LSP server")
			return &server, nil
		}

		// Cache negative result
		cache.SetAvailable(server.Command, false)
	}

	logger.WithField("language", language).Debug("No available LSP servers found for language")
	return nil, nil
}

// isCommandAvailable checks if a command is available in PATH
func isCommandAvailable(_ context.Context, command string) bool {
	// Use exec.LookPath which is portable and doesn't require external commands
	_, err := exec.LookPath(command)
	return err == nil
}

// GetAvailableLanguages returns a list of languages with available LSP servers
func GetAvailableLanguages(ctx context.Context, logger *logrus.Logger) []string {
	cache := GetServerCache()
	var available []string
	seen := make(map[string]bool)

	for _, server := range SupportedServers {
		// Skip if we've already checked this language
		if seen[server.Language] {
			continue
		}
		seen[server.Language] = true

		// Check cache first
		if avail, exists := cache.IsAvailable(server.Command); exists {
			if avail {
				available = append(available, server.Language)
			}
			continue
		}

		// Not in cache, check availability
		if isCommandAvailable(ctx, server.Command) {
			cache.SetAvailable(server.Command, true)
			available = append(available, server.Language)
		} else {
			cache.SetAvailable(server.Command, false)
		}
	}

	return available
}
