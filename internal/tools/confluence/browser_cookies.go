package confluence

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver for cookie databases
	"github.com/sirupsen/logrus"
)

// BrowserCookieExtractor handles extracting cookies from various browsers
type BrowserCookieExtractor struct {
	logger *logrus.Logger
}

// NewBrowserCookieExtractor creates a new browser cookie extractor
func NewBrowserCookieExtractor(logger *logrus.Logger) *BrowserCookieExtractor {
	return &BrowserCookieExtractor{
		logger: logger,
	}
}

// BrowserType represents supported browser types
type BrowserType string

const (
	BrowserChrome         BrowserType = "chrome"
	BrowserChromium       BrowserType = "chromium"
	BrowserBrave          BrowserType = "brave"
	BrowserEdge           BrowserType = "edge"
	BrowserFirefox        BrowserType = "firefox"
	BrowserFirefoxNightly BrowserType = "firefox-nightly"
	BrowserSafari         BrowserType = "safari"
)

// Cookie represents a browser cookie
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  time.Time
	Secure   bool
	HttpOnly bool
}

// ExtractCookies extracts cookies for a specific domain from the specified browser
func (e *BrowserCookieExtractor) ExtractCookies(browserType BrowserType, domain string) ([]Cookie, error) {
	e.logger.WithFields(logrus.Fields{
		"browser": browserType,
		"domain":  domain,
	}).Debug("Extracting cookies from browser")

	switch browserType {
	case BrowserChrome, BrowserChromium, BrowserBrave, BrowserEdge:
		return e.extractChromiumCookies(browserType, domain)
	case BrowserFirefox, BrowserFirefoxNightly:
		return e.extractFirefoxCookies(browserType, domain)
	case BrowserSafari:
		return e.extractSafariCookies(domain)
	default:
		return nil, fmt.Errorf("unsupported browser type: %s", browserType)
	}
}

// FormatCookiesForHTTP formats cookies as an HTTP Cookie header value
func (e *BrowserCookieExtractor) FormatCookiesForHTTP(cookies []Cookie) string {
	var cookiePairs []string
	for _, cookie := range cookies {
		cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	return strings.Join(cookiePairs, "; ")
}

// extractChromiumCookies extracts cookies from Chromium-based browsers
func (e *BrowserCookieExtractor) extractChromiumCookies(browserType BrowserType, domain string) ([]Cookie, error) {
	cookieDBPath, err := e.getChromiumCookieDBPath(browserType)
	if err != nil {
		return nil, fmt.Errorf("failed to get cookie database path: %w", err)
	}

	// Copy the cookie database to a temporary location (browsers lock the original)
	tempDBPath, err := e.copyToTemp(cookieDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to copy cookie database: %w", err)
	}
	defer func() { _ = os.Remove(tempDBPath) }()

	db, err := sql.Open("sqlite3", tempDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cookie database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Query cookies for the domain
	query := `
		SELECT name, value, host_key, path, expires_utc, is_secure, is_httponly
		FROM cookies
		WHERE host_key LIKE ? OR host_key LIKE ?
		ORDER BY creation_utc DESC
	`

	rows, err := db.Query(query, "%"+domain, "%."+domain)
	if err != nil {
		return nil, fmt.Errorf("failed to query cookies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cookies []Cookie
	for rows.Next() {
		var cookie Cookie
		var expiresUTC int64
		var isSecure, isHttpOnly int

		err := rows.Scan(
			&cookie.Name,
			&cookie.Value,
			&cookie.Domain,
			&cookie.Path,
			&expiresUTC,
			&isSecure,
			&isHttpOnly,
		)
		if err != nil {
			e.logger.WithError(err).Warn("Failed to scan cookie row")
			continue
		}

		// Convert Chrome's epoch time (microseconds since 1601-01-01) to Go time
		if expiresUTC > 0 {
			// Chrome epoch starts at 1601-01-01, Unix epoch starts at 1970-01-01
			// Difference is 11644473600 seconds
			unixSeconds := (expiresUTC / 1000000) - 11644473600
			cookie.Expires = time.Unix(unixSeconds, 0)
		}

		cookie.Secure = isSecure == 1
		cookie.HttpOnly = isHttpOnly == 1

		cookies = append(cookies, cookie)
	}

	e.logger.WithField("count", len(cookies)).Debug("Extracted cookies from Chromium browser")
	return cookies, nil
}

// extractFirefoxCookies extracts cookies from Firefox browsers
func (e *BrowserCookieExtractor) extractFirefoxCookies(browserType BrowserType, domain string) ([]Cookie, error) {
	cookieDBPath, err := e.getFirefoxCookieDBPath(browserType)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firefox cookie database path: %w", err)
	}

	// Copy the cookie database to a temporary location
	tempDBPath, err := e.copyToTemp(cookieDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to copy cookie database: %w", err)
	}
	defer func() { _ = os.Remove(tempDBPath) }()

	db, err := sql.Open("sqlite3", tempDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cookie database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Query cookies for the domain
	query := `
		SELECT name, value, host, path, expiry, isSecure, isHttpOnly
		FROM moz_cookies
		WHERE host LIKE ? OR host LIKE ?
		ORDER BY creationTime DESC
	`

	rows, err := db.Query(query, "%"+domain, "%."+domain)
	if err != nil {
		return nil, fmt.Errorf("failed to query cookies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cookies []Cookie
	for rows.Next() {
		var cookie Cookie
		var expiry int64
		var isSecure, isHttpOnly int

		err := rows.Scan(
			&cookie.Name,
			&cookie.Value,
			&cookie.Domain,
			&cookie.Path,
			&expiry,
			&isSecure,
			&isHttpOnly,
		)
		if err != nil {
			e.logger.WithError(err).Warn("Failed to scan cookie row")
			continue
		}

		if expiry > 0 {
			cookie.Expires = time.Unix(expiry, 0)
		}

		cookie.Secure = isSecure == 1
		cookie.HttpOnly = isHttpOnly == 1

		cookies = append(cookies, cookie)
	}

	e.logger.WithField("count", len(cookies)).Debug("Extracted cookies from Firefox browser")
	return cookies, nil
}

// extractSafariCookies extracts cookies from Safari (macOS only)
func (e *BrowserCookieExtractor) extractSafariCookies(domain string) ([]Cookie, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("safari cookie extraction is only supported on macOS")
	}

	// Safari stores cookies in a binary plist format, which is more complex to parse
	// For now, return an error suggesting manual extraction
	return nil, fmt.Errorf("safari cookie extraction not yet implemented - please extract cookies manually from Safari's Web Inspector")
}

// getChromiumCookieDBPath returns the path to the cookie database for Chromium-based browsers
func (e *BrowserCookieExtractor) getChromiumCookieDBPath(browserType BrowserType) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	var basePath string
	switch runtime.GOOS {
	case "darwin": // macOS
		switch browserType {
		case BrowserChrome:
			basePath = filepath.Join(homeDir, "Library/Application Support/Google/Chrome/Default")
		case BrowserChromium:
			basePath = filepath.Join(homeDir, "Library/Application Support/Chromium/Default")
		case BrowserBrave:
			basePath = filepath.Join(homeDir, "Library/Application Support/BraveSoftware/Brave-Browser/Default")
		case BrowserEdge:
			basePath = filepath.Join(homeDir, "Library/Application Support/Microsoft Edge/Default")
		}
	case "linux":
		switch browserType {
		case BrowserChrome:
			basePath = filepath.Join(homeDir, ".config/google-chrome/Default")
		case BrowserChromium:
			basePath = filepath.Join(homeDir, ".config/chromium/Default")
		case BrowserBrave:
			basePath = filepath.Join(homeDir, ".config/BraveSoftware/Brave-Browser/Default")
		case BrowserEdge:
			basePath = filepath.Join(homeDir, ".config/microsoft-edge/Default")
		}
	case "windows":
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData/Local")
		}
		switch browserType {
		case BrowserChrome:
			basePath = filepath.Join(appData, "Google/Chrome/User Data/Default")
		case BrowserChromium:
			basePath = filepath.Join(appData, "Chromium/User Data/Default")
		case BrowserBrave:
			basePath = filepath.Join(appData, "BraveSoftware/Brave-Browser/User Data/Default")
		case BrowserEdge:
			basePath = filepath.Join(appData, "Microsoft/Edge/User Data/Default")
		}
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if basePath == "" {
		return "", fmt.Errorf("unsupported browser type: %s", browserType)
	}

	cookiePath := filepath.Join(basePath, "Cookies")
	if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
		// Try alternative path for newer Chrome versions
		cookiePath = filepath.Join(basePath, "Network/Cookies")
		if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
			return "", fmt.Errorf("cookie database not found at %s", cookiePath)
		}
	}

	return cookiePath, nil
}

// getFirefoxCookieDBPath returns the path to the cookie database for Firefox browsers
func (e *BrowserCookieExtractor) getFirefoxCookieDBPath(browserType BrowserType) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	var profilesDir string
	switch runtime.GOOS {
	case "darwin": // macOS
		switch browserType {
		case BrowserFirefox:
			profilesDir = filepath.Join(homeDir, "Library/Application Support/Firefox/Profiles")
		case BrowserFirefoxNightly:
			profilesDir = filepath.Join(homeDir, "Library/Application Support/Firefox Nightly/Profiles")
		}
	case "linux":
		switch browserType {
		case BrowserFirefox:
			profilesDir = filepath.Join(homeDir, ".mozilla/firefox")
		case BrowserFirefoxNightly:
			profilesDir = filepath.Join(homeDir, ".mozilla/firefox-nightly")
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData/Roaming")
		}
		switch browserType {
		case BrowserFirefox:
			profilesDir = filepath.Join(appData, "Mozilla/Firefox/Profiles")
		case BrowserFirefoxNightly:
			profilesDir = filepath.Join(appData, "Mozilla/Firefox Nightly/Profiles")
		}
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if profilesDir == "" {
		return "", fmt.Errorf("unsupported browser type: %s", browserType)
	}

	// Find the default profile directory
	profileDir, err := e.findFirefoxDefaultProfile(profilesDir)
	if err != nil {
		return "", fmt.Errorf("failed to find Firefox profile: %w", err)
	}

	cookiePath := filepath.Join(profileDir, "cookies.sqlite")
	if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
		return "", fmt.Errorf("cookie database not found at %s", cookiePath)
	}

	return cookiePath, nil
}

// findFirefoxDefaultProfile finds the default Firefox profile directory
func (e *BrowserCookieExtractor) findFirefoxDefaultProfile(profilesDir string) (string, error) {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return "", fmt.Errorf("failed to read profiles directory: %w", err)
	}

	// Look for profiles.ini to find the default profile
	profilesIni := filepath.Join(filepath.Dir(profilesDir), "profiles.ini")
	if _, err := os.Stat(profilesIni); err == nil {
		// Parse profiles.ini to find default profile
		// This is a simplified approach - in practice, you'd want to parse the INI file properly
		content, err := os.ReadFile(profilesIni)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Path=") {
					profilePath := strings.TrimPrefix(line, "Path=")
					profilePath = strings.TrimSpace(profilePath)
					if !filepath.IsAbs(profilePath) {
						profilePath = filepath.Join(profilesDir, profilePath)
					}
					if _, err := os.Stat(profilePath); err == nil {
						return profilePath, nil
					}
				}
			}
		}
	}

	// Fallback: find the most recently modified profile directory
	var newestProfile string
	var newestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), ".default") {
			profilePath := filepath.Join(profilesDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(newestTime) {
				newestTime = info.ModTime()
				newestProfile = profilePath
			}
		}
	}

	if newestProfile == "" {
		return "", fmt.Errorf("no Firefox profile found in %s", profilesDir)
	}

	return newestProfile, nil
}

// copyToTemp copies a file to a temporary location
func (e *BrowserCookieExtractor) copyToTemp(srcPath string) (string, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = src.Close() }()

	tempFile, err := os.CreateTemp("", "cookies_*.db")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() { _ = tempFile.Close() }()

	_, err = tempFile.ReadFrom(src)
	if err != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	return tempFile.Name(), nil
}

// GetSupportedBrowsers returns a list of supported browser types
func GetSupportedBrowsers() []BrowserType {
	browsers := []BrowserType{
		BrowserChrome,
		BrowserChromium,
		BrowserBrave,
		BrowserEdge,
		BrowserFirefox,
		BrowserFirefoxNightly,
	}

	// Add Safari only on macOS
	if runtime.GOOS == "darwin" {
		browsers = append(browsers, BrowserSafari)
	}

	return browsers
}

// ParseBrowserType parses a browser type string
func ParseBrowserType(browserStr string) (BrowserType, error) {
	browserStr = strings.ToLower(strings.TrimSpace(browserStr))

	switch browserStr {
	case "chrome":
		return BrowserChrome, nil
	case "chromium":
		return BrowserChromium, nil
	case "brave":
		return BrowserBrave, nil
	case "edge", "microsoft-edge":
		return BrowserEdge, nil
	case "firefox":
		return BrowserFirefox, nil
	case "firefox-nightly", "nightly":
		return BrowserFirefoxNightly, nil
	case "safari":
		if runtime.GOOS != "darwin" {
			return "", fmt.Errorf("safari is only supported on macOS")
		}
		return BrowserSafari, nil
	default:
		return "", fmt.Errorf("unsupported browser: %s", browserStr)
	}
}
