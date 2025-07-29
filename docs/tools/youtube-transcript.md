# YouTube Transcript Tool

Extract transcripts from YouTube videos with multiple output formats and language support.

## Overview

The YouTube transcript tool provides a robust way to extract captions and transcripts from YouTube videos. It supports multiple extraction methods and output formats, with intelligent fallback mechanisms for maximum reliability.

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `video_url` | string | ✅ | - | YouTube video URL or video ID |
| `language` | string | ❌ | `"auto"` | Preferred language code (e.g., 'en-GB', 'en', 'fr') |
| `format` | string | ❌ | `"text"` | Output format: `text`, `json`, `srt`, `vtt` |
| `include_timestamps` | boolean | ❌ | `true` | Include timing information in output |
| `auto_generated_fallback` | boolean | ❌ | `true` | Allow auto-generated captions if manual ones unavailable |
| `translate_to` | string | ❌ | - | Translate transcript to specified language (requires API key) |

## Output Formats

### Text Format
```
[00:01] Welcome to this tutorial
[00:05] Today we'll learn about...
[00:12] First, let's start with...
```

### JSON Format
```json
{
  "video_id": "xHHlhoRC8W4",
  "title": "Example Video Title",
  "language": "en",
  "is_auto_generated": false,
  "format": "json",
  "segments": [
    {
      "text": "Welcome to this tutorial",
      "start": 1.5,
      "duration": 3.2
    }
  ],
  "metadata": {
    "extraction_method": "youtube_data_api_v3",
    "extracted_at": "2024-01-01T12:00:00Z",
    "available_languages": ["en", "es", "fr"]
  }
}
```

### SRT Format
```
1
00:00:01,500 --> 00:00:04,700
Welcome to this tutorial

2
00:00:05,000 --> 00:00:08,200
Today we'll learn about...
```

### VTT Format
```
WEBVTT

00:00:01.500 --> 00:00:04.700
Welcome to this tutorial

00:00:05.000 --> 00:00:08.200
Today we'll learn about...
```

## Usage Examples

### Basic Text Extraction
```json
{
  "video_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  "format": "text",
  "include_timestamps": false
}
```

### Extract with British English Preference
```json
{
  "video_url": "xHHlhoRC8W4",
  "language": "en-GB",
  "format": "json",
  "include_timestamps": true
}
```

### Generate SRT Subtitles
```json
{
  "video_url": "https://youtu.be/xHHlhoRC8W4",
  "format": "srt"
}
```

## Supported URL Formats

The tool accepts various YouTube URL formats:

- `https://www.youtube.com/watch?v=VIDEO_ID`
- `https://youtu.be/VIDEO_ID`
- `https://m.youtube.com/watch?v=VIDEO_ID`
- `https://www.youtube.com/embed/VIDEO_ID`
- `VIDEO_ID` (raw video ID)

## Language Handling

### Language Preference Order

1. **Specified language** (e.g., `"en-GB"`)
2. **British English** (`"en-GB"`) - default preference
3. **Generic English** (`"en"`)
4. **US English** (`"en-US"`)
5. **First available language**

### Language Codes
Common language codes include:
- `en-GB` - British English
- `en-US` - US English
- `en` - Generic English
- `es` - Spanish
- `fr` - French
- `de` - German
- `ja` - Japanese

## Extraction Methods

The tool uses multiple extraction methods for maximum reliability:

### 1. YouTube Data API v3 (Recommended)
- **Pros**: Official API, reliable, higher rate limits
- **Cons**: Requires API key
- **Setup**: Set `YOUTUBE_API_KEY` environment variable

### 2. Unofficial API (Fallback)
- **Pros**: No API key required
- **Cons**: Subject to rate limiting, may break if YouTube changes their internal APIs

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `YOUTUBE_API_KEY` | - | YouTube Data API v3 key (highly recommended) |
| `YOUTUBE_TRANSCRIPT_CACHE_ENABLED` | `true` | Enable transcript caching |
| `YOUTUBE_TRANSCRIPT_CACHE_TTL_MINUTES` | `60` | Cache TTL in minutes |

### Getting a YouTube API Key

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the "YouTube Data API v3"
4. Create credentials (API key)
5. Set the API key in your environment: `export YOUTUBE_API_KEY=your_api_key_here`

## Error Handling

### Common Errors

#### Rate Limiting (HTTP 429)
```
Error: rate limited by YouTube (HTTP 429) - please try again later or use a different IP address
```

**Solutions:**
- Use a YouTube API key for higher rate limits
- Wait a few minutes before retrying
- Try from a different IP address or network

#### No Captions Available
```
Error: no captions available for this video
```

**Solutions:**
- Check if the video has captions enabled
- Try with `auto_generated_fallback: true`
- Some videos may not have any captions

#### Invalid Video ID
```
Error: failed to extract video ID from URL
```

**Solutions:**
- Ensure the URL is a valid YouTube URL
- Try using just the video ID instead of the full URL

#### API Key Issues
```
Error: API error 403: The request cannot be completed because you have exceeded your quota
```

**Solutions:**
- Check your YouTube API quota in Google Cloud Console
- Wait for quota reset (usually daily)
- Upgrade your quota limits if needed

## Performance Notes

- **With API Key**: Typically 2-5 seconds per video
- **Without API Key**: May encounter rate limiting, especially with multiple requests
- **Caching**: Results are cached for 60 minutes by default to avoid repeated API calls

## Limitations

1. **Rate Limiting**: YouTube may limit requests, especially without an API key
2. **Captions Availability**: Not all videos have captions available
3. **Language Support**: Limited to languages supported by YouTube's captioning system
4. **Video Access**: Cannot extract from private or restricted videos

## Best Practices

1. **Use an API Key**: Significantly improves reliability and rate limits
2. **Cache Results**: Enable caching to avoid repeated requests for the same video
3. **Handle Errors Gracefully**: Implement proper error handling in your application
4. **Respect Rate Limits**: Avoid making too many requests in quick succession
5. **Choose Appropriate Format**: Use JSON for programmatic processing, SRT/VTT for subtitle files

## Examples

### CLI Usage
```bash
# With API key
export YOUTUBE_API_KEY=your_api_key_here

# Extract basic text transcript
echo '{"video_url": "dQw4w9WgXcQ", "format": "text"}' | mcp-devtools call youtube_transcript

# Extract structured JSON with timestamps
echo '{"video_url": "dQw4w9WgXcQ", "format": "json"}' | mcp-devtools call youtube_transcript

# Extract SRT subtitles for a specific language
echo '{"video_url": "dQw4w9WgXcQ", "format": "srt", "language": "en-GB"}' | mcp-devtools call youtube_transcript
```

### Programmatic Usage
```python
import json
import subprocess

def get_youtube_transcript(video_url, format="text", language="auto"):
    payload = {
        "video_url": video_url,
        "format": format,
        "language": language
    }
    
    cmd = ["mcp-devtools", "call", "youtube_transcript"]
    result = subprocess.run(cmd, input=json.dumps(payload), 
                          capture_output=True, text=True)
    
    if result.returncode == 0:
        return json.loads(result.stdout)
    else:
        raise Exception(f"Error: {result.stderr}")

# Usage
transcript = get_youtube_transcript("dQw4w9WgXcQ", "json", "en-GB")
print(transcript["title"])
for segment in transcript["segments"]:
    print(f"[{segment['start']:.1f}s] {segment['text']}")
```

## Troubleshooting

### Tool Not Found
Ensure the YouTube transcript tool is properly installed and registered with the MCP server.

### Connection Timeouts
Check your internet connection and try again. YouTube's servers may be temporarily unavailable.

### Persistent Rate Limiting
- Use a YouTube API key
- Try from a different network
- Wait longer between requests
- Contact support if the issue persists

For additional support, check the main MCP DevTools documentation or file an issue on the project repository.