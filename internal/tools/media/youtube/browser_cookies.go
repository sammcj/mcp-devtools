package youtube

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// BrowserCookieExtractor extracts cookies from various browsers
type BrowserCookieExtractor struct {
	logger *logrus.Logger
}

// BrowserType represents different browser types
type BrowserType int

const (
	Chrome BrowserType = iota
	Firefox
	FirefoxNightly
	Safari
	Edge
)

// CookieInfo represents a browser cookie
type CookieInfo struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Secure   bool
	HttpOnly bool
}

// NewBrowserCookieExtractor creates a new browser cookie extractor
func NewBrowserCookieExtractor(logger *logrus.Logger) *BrowserCookieExtractor {
	return &BrowserCookieExtractor{
		logger: logger,
	}
}

// ExtractYouTubeCookies extracts YouTube cookies from available browsers
func (e *BrowserCookieExtractor) ExtractYouTubeCookies() ([]*http.Cookie, error) {
	var allCookies []*http.Cookie

	// Try each browser in order of preference (Firefox first per user preference)
	browsers := []BrowserType{Firefox, FirefoxNightly, Safari, Chrome, Edge}

	for _, browser := range browsers {
		cookies, err := e.extractFromBrowser(browser)
		if err != nil {
			e.logger.Debugf("Failed to extract cookies from %s: %v", e.getBrowserName(browser), err)
			continue
		}

		if len(cookies) > 0 {
			e.logger.Debugf("Successfully extracted %d cookies from %s", len(cookies), e.getBrowserName(browser))
			allCookies = append(allCookies, cookies...)
			break // Use first successful browser
		}
	}

	if len(allCookies) == 0 {
		return nil, fmt.Errorf("no YouTube cookies found in any browser")
	}

	return allCookies, nil
}

// extractFromBrowser extracts cookies from a specific browser
func (e *BrowserCookieExtractor) extractFromBrowser(browser BrowserType) ([]*http.Cookie, error) {
	switch browser {
	case Chrome:
		return e.extractChromeNetCookies()
	case Firefox:
		return e.extractFirefoxCookies(false)
	case FirefoxNightly:
		return e.extractFirefoxCookies(true)
	case Safari:
		return e.extractSafariCookies()
	case Edge:
		return e.extractEdgeCookies()
	default:
		return nil, fmt.Errorf("unsupported browser type")
	}
}

// extractChromeNetCookies extracts cookies from Chrome
func (e *BrowserCookieExtractor) extractChromeNetCookies() ([]*http.Cookie, error) {
	var cookiePath string

	switch runtime.GOOS {
	case "darwin": // macOS
		homeDir, _ := os.UserHomeDir()
		cookiePath = filepath.Join(homeDir, "Library/Application Support/Google/Chrome/Default/Cookies")
	case "linux":
		homeDir, _ := os.UserHomeDir()
		cookiePath = filepath.Join(homeDir, ".config/google-chrome/Default/Cookies")
	case "windows":
		appData := os.Getenv("LOCALAPPDATA")
		cookiePath = filepath.Join(appData, "Google/Chrome/User Data/Default/Cookies")
	default:
		return nil, fmt.Errorf("unsupported operating system for Chrome cookies")
	}

	return e.extractSQLiteCookies(cookiePath, "chrome")
}

// extractFirefoxCookies extracts cookies from Firefox
func (e *BrowserCookieExtractor) extractFirefoxCookies(nightly bool) ([]*http.Cookie, error) {
	var profilesPath string

	switch runtime.GOOS {
	case "darwin": // macOS
		homeDir, _ := os.UserHomeDir()
		if nightly {
			profilesPath = filepath.Join(homeDir, "Library/Application Support/Firefox Nightly/Profiles")
		} else {
			profilesPath = filepath.Join(homeDir, "Library/Application Support/Firefox/Profiles")
		}
	case "linux":
		homeDir, _ := os.UserHomeDir()
		if nightly {
			profilesPath = filepath.Join(homeDir, ".mozilla/firefox-nightly")
		} else {
			profilesPath = filepath.Join(homeDir, ".mozilla/firefox")
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if nightly {
			profilesPath = filepath.Join(appData, "Mozilla/Firefox Nightly/Profiles")
		} else {
			profilesPath = filepath.Join(appData, "Mozilla/Firefox/Profiles")
		}
	default:
		return nil, fmt.Errorf("unsupported operating system for Firefox cookies")
	}

	// Find the default profile
	entries, err := os.ReadDir(profilesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Firefox profiles directory: %w", err)
	}

	// Try to find any Firefox profile with cookies (not just default profiles)
	for _, entry := range entries {
		if entry.IsDir() {
			cookiePath := filepath.Join(profilesPath, entry.Name(), "cookies.sqlite")
			if _, err := os.Stat(cookiePath); err == nil {
				// Found a profile with cookies.sqlite, try to extract
				return e.extractSQLiteCookies(cookiePath, "firefox")
			}
		}
	}

	return nil, fmt.Errorf("no default Firefox profile found")
}

// extractSafariCookies extracts cookies from Safari
func (e *BrowserCookieExtractor) extractSafariCookies() ([]*http.Cookie, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("safari cookies only available on macOS")
	}

	homeDir, _ := os.UserHomeDir()
	_ = filepath.Join(homeDir, "Library/Cookies/Cookies.binarycookies")

	// Safari uses binary cookies format which is more complex
	// For now, return an error suggesting manual extraction
	return nil, fmt.Errorf("safari cookie extraction not yet implemented - please use Chrome or Firefox")
}

// extractEdgeCookies extracts cookies from Microsoft Edge
func (e *BrowserCookieExtractor) extractEdgeCookies() ([]*http.Cookie, error) {
	var cookiePath string

	switch runtime.GOOS {
	case "darwin": // macOS
		homeDir, _ := os.UserHomeDir()
		cookiePath = filepath.Join(homeDir, "Library/Application Support/Microsoft Edge/Default/Cookies")
	case "linux":
		homeDir, _ := os.UserHomeDir()
		cookiePath = filepath.Join(homeDir, ".config/microsoft-edge/Default/Cookies")
	case "windows":
		appData := os.Getenv("LOCALAPPDATA")
		cookiePath = filepath.Join(appData, "Microsoft/Edge/User Data/Default/Cookies")
	default:
		return nil, fmt.Errorf("unsupported operating system for Edge cookies")
	}

	return e.extractSQLiteCookies(cookiePath, "edge")
}

// extractSQLiteCookies extracts cookies from SQLite database
func (e *BrowserCookieExtractor) extractSQLiteCookies(cookiePath, browserType string) ([]*http.Cookie, error) {
	if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cookie database not found: %s", cookiePath)
	}

	// Create a temporary copy to avoid file locking issues
	tempPath := cookiePath + ".tmp"
	if err := e.copyFile(cookiePath, tempPath); err != nil {
		return nil, fmt.Errorf("failed to create temporary cookie file: %w", err)
	}
	defer func() { _ = os.Remove(tempPath) }()

	db, err := sql.Open("sqlite3", tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cookie database: %w", err)
	}
	defer func() { _ = db.Close() }()

	var query string
	switch browserType {
	case "chrome", "edge":
		query = `SELECT name, value, host_key, path, is_secure, is_httponly 
				 FROM cookies 
				 WHERE host_key LIKE '%youtube.com%' OR host_key LIKE '%.youtube.com%'`
	case "firefox":
		query = `SELECT name, value, host, path, isSecure, isHttpOnly 
				 FROM moz_cookies 
				 WHERE host LIKE '%youtube.com%' OR host LIKE '%.youtube.com%'`
	default:
		return nil, fmt.Errorf("unsupported browser type for SQLite extraction")
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query cookies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cookies []*http.Cookie
	for rows.Next() {
		var name, value, domain, path string
		var secure, httpOnly bool

		if err := rows.Scan(&name, &value, &domain, &path, &secure, &httpOnly); err != nil {
			e.logger.Debugf("Failed to scan cookie row: %v", err)
			continue
		}

		// Filter for important YouTube cookies
		if e.isImportantYouTubeCookie(name) {
			cookie := &http.Cookie{
				Name:     name,
				Value:    value,
				Domain:   domain,
				Path:     path,
				Secure:   secure,
				HttpOnly: httpOnly,
			}
			cookies = append(cookies, cookie)
		}
	}

	return cookies, nil
}

// isImportantYouTubeCookie checks if a cookie is important for YouTube authentication
func (e *BrowserCookieExtractor) isImportantYouTubeCookie(name string) bool {
	importantCookies := []string{
		"VISITOR_INFO1_LIVE",
		"YSC",
		"PREF",
		"CONSENT",
		"SOCS",
		"__Secure-YEC",
		"LOGIN_INFO",
		"SAPISID",
		"APISID",
		"SSID",
		"HSID",
		"SID",
		"__Secure-3PAPISID",
		"__Secure-3PSID",
		"SESSION_TOKEN",
	}

	for _, important := range importantCookies {
		if name == important {
			return true
		}
	}
	return false
}

// copyFile copies a file from src to dst
func (e *BrowserCookieExtractor) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

// getBrowserName returns the human-readable name of a browser type
func (e *BrowserCookieExtractor) getBrowserName(browser BrowserType) string {
	switch browser {
	case Chrome:
		return "Chrome"
	case Firefox:
		return "Firefox"
	case FirefoxNightly:
		return "Firefox Nightly"
	case Safari:
		return "Safari"
	case Edge:
		return "Edge"
	default:
		return "Unknown"
	}
}

// AddCookiesToRequest adds cookies to an HTTP request
func AddCookiesToRequest(req *http.Request, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
}
