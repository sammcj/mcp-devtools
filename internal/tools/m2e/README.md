# m2e - American to British English Converter

This tool provides conversion from American English to British English (en-GB) using the excellent [m2e](https://github.com/sammcj/m2e) library.

## Features

- Converts American spellings to British equivalents
- Preserves formatting, capitalisation, and punctuation  
- Code-aware processing to avoid converting code elements
- Supports smart quote normalisation
- Uses both built-in and user-defined dictionaries
- **Default**: Updates files in place
- **Alternative**: Inline text conversion

## Operation Modes

### Default: Update File Mode  
Provide a file path and the tool will update the file in place, returning only a success confirmation.

### Alternative: Inline Mode
Provide text directly and receive the converted text in the response.

## Parameters

- `file_path` (default mode): Fully qualified absolute path to the file to update in place
- `text` (alternative mode): The text to convert and return inline
- `keep_smart_quotes` (optional, default: false): Whether to keep smart quotes and em-dashes as-is (default: false, meaning they will be normalised to standard quotes)

**Note**: Provide either `file_path` OR `text`, not both.

## Usage Examples

### Default: File Update Examples

Update a file in place:
```json
{
  "file_path": "/Users/username/Documents/readme.md"
}
```

Update a file whilst keeping smart quotes:
```json
{
  "file_path": "/Users/username/Documents/readme.md",
  "keep_smart_quotes": true
}
```

### Alternative: Inline Mode Examples

Convert text inline:
```json
{
  "text": "I need to organize my favorite colors."
}
```

Convert whilst keeping smart quotes:
```json
{
  "text": "This "color" is my favorite!",
  "keep_smart_quotes": true
}
```

## Notes

The tool uses the m2e library which includes comprehensive American-to-British spelling dictionaries and sophisticated text processing that preserves code blocks, URLs, and formatting whilst converting natural language text.

The tool will only write the file if changes were made, avoiding unnecessary file modifications.

By default, smart quotes and em-dashes are normalised to standard quotes and hyphens. Use `keep_smart_quotes: true` to preserve them.