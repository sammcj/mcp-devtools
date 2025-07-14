package client

import (
	"fmt"
	"os/exec"
	"runtime"
)

// DefaultBrowserLauncher implements BrowserLauncher for cross-platform browser opening
type DefaultBrowserLauncher struct{}

// NewBrowserLauncher creates a new browser launcher
func NewBrowserLauncher() BrowserLauncher {
	return &DefaultBrowserLauncher{}
}

// OpenURL opens a URL in the default system browser
func (b *DefaultBrowserLauncher) OpenURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Execute the command to open the browser
	if err := exec.Command(cmd, args...).Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}
