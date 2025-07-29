package youtube

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ExtractVideoID extracts the YouTube video ID from various URL formats
func ExtractVideoID(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty input")
	}

	// If it's already a video ID (11 characters, alphanumeric and some symbols)
	if IsValidVideoID(input) {
		return input, nil
	}

	// Parse as URL
	parsedURL, err := url.Parse(input)
	if err != nil {
		// Try to parse with scheme if missing
		if !strings.Contains(input, "://") {
			input = "https://" + input
			parsedURL, err = url.Parse(input)
			if err != nil {
				return "", fmt.Errorf("failed to parse URL: %w", err)
			}
		} else {
			return "", fmt.Errorf("failed to parse URL: %w", err)
		}
	}

	// Handle different YouTube URL formats
	switch {
	case strings.Contains(parsedURL.Host, "youtube.com"):
		return extractFromYouTubeURL(parsedURL)
	case strings.Contains(parsedURL.Host, "youtu.be"):
		return extractFromYouTuBeURL(parsedURL)
	case strings.Contains(parsedURL.Host, "m.youtube.com"):
		return extractFromYouTubeURL(parsedURL)
	default:
		return "", fmt.Errorf("unsupported URL format: %s", input)
	}
}

// extractFromYouTubeURL extracts video ID from youtube.com URLs
func extractFromYouTubeURL(parsedURL *url.URL) (string, error) {
	query := parsedURL.Query()

	// Standard watch URL: https://www.youtube.com/watch?v=VIDEO_ID
	if videoID := query.Get("v"); videoID != "" {
		if IsValidVideoID(videoID) {
			return videoID, nil
		}
		return "", fmt.Errorf("invalid video ID: %s", videoID)
	}

	// Embed URL: https://www.youtube.com/embed/VIDEO_ID
	if strings.HasPrefix(parsedURL.Path, "/embed/") {
		videoID := strings.TrimPrefix(parsedURL.Path, "/embed/")
		videoID = strings.Split(videoID, "?")[0] // Remove query parameters
		if IsValidVideoID(videoID) {
			return videoID, nil
		}
		return "", fmt.Errorf("invalid video ID in embed URL: %s", videoID)
	}

	// Live URL: https://www.youtube.com/live/VIDEO_ID
	if strings.HasPrefix(parsedURL.Path, "/live/") {
		videoID := strings.TrimPrefix(parsedURL.Path, "/live/")
		videoID = strings.Split(videoID, "?")[0] // Remove query parameters
		if IsValidVideoID(videoID) {
			return videoID, nil
		}
		return "", fmt.Errorf("invalid video ID in live URL: %s", videoID)
	}

	return "", fmt.Errorf("no video ID found in YouTube URL")
}

// extractFromYouTuBeURL extracts video ID from youtu.be URLs
func extractFromYouTuBeURL(parsedURL *url.URL) (string, error) {
	// Short URL: https://youtu.be/VIDEO_ID
	videoID := strings.TrimPrefix(parsedURL.Path, "/")
	videoID = strings.Split(videoID, "?")[0] // Remove query parameters

	if IsValidVideoID(videoID) {
		return videoID, nil
	}

	return "", fmt.Errorf("invalid video ID in short URL: %s", videoID)
}

// IsValidVideoID checks if a string is a valid YouTube video ID
func IsValidVideoID(videoID string) bool {
	// YouTube video IDs are 11 characters long and contain alphanumeric characters, hyphens, and underscores
	if len(videoID) != 11 {
		return false
	}

	match, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{11}$`, videoID)
	return match
}

// NormaliseLanguageCode normalises language codes to match YouTube's format
func NormaliseLanguageCode(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))

	// Handle common variations
	switch lang {
	case "en-gb", "en_gb", "british", "british english":
		return "en-GB"
	case "en-us", "en_us", "american", "american english":
		return "en-US"
	case "en", "english":
		return "en"
	case "fr", "french", "français":
		return "fr"
	case "de", "german", "deutsch":
		return "de"
	case "es", "spanish", "español":
		return "es"
	case "it", "italian", "italiano":
		return "it"
	case "pt", "portuguese", "português":
		return "pt"
	case "ru", "russian", "русский":
		return "ru"
	case "ja", "japanese", "日本語":
		return "ja"
	case "ko", "korean", "한국어":
		return "ko"
	case "zh", "chinese", "中文":
		return "zh"
	case "ar", "arabic", "العربية":
		return "ar"
	case "hi", "hindi", "हिन्दी":
		return "hi"
	default:
		return lang
	}
}

// ValidateFormat checks if the output format is supported
func ValidateFormat(format string) error {
	switch OutputFormat(format) {
	case FormatText, FormatJSON, FormatSRT, FormatVTT:
		return nil
	default:
		return fmt.Errorf("unsupported format: %s (supported: text, json, srt, vtt)", format)
	}
}
