# American to English Conversion Tool

The American to English conversion tool converts American English text to International / British English spelling, helping maintain consistent International English standards in documentation and content.

## Overview

This tool automatically converts American English spellings to their British English equivalents, ensuring consistent international spelling standards. Perfect for documentation, content creation, and maintaining British English conventions across projects.

## Features

- **File Processing**: Convert entire files in place
- **Inline Conversion**: Convert text snippets and return results
- **Smart Quote Handling**: Optionally normalise smart quotes and em-dashes
- **Comprehensive Coverage**: Handles common American/British spelling differences
- **Preserve Formatting**: Maintains original text structure and formatting

## Usage

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### File Processing (In-Place)
Convert an entire file and save changes:
```json
{
  "name": "murican_to_english",
  "arguments": {
    "file_path": "/absolute/path/to/document.md"
  }
}
```

### Inline Text Conversion
Convert text and return the result:
```json
{
  "name": "murican_to_english",
  "arguments": {
    "text": "The color of the aluminum center was optimized for organization."
  }
}
```

### With Smart Quote Normalisation
```json
{
  "name": "murican_to_english",
  "arguments": {
    "text": "The company's "modernization" efforts were successful.",
    "keep_smart_quotes": false
  }
}
```

## Parameters Reference

### Core Parameters

| Parameter           | Type    | Required | Description                                      |
|---------------------|---------|----------|--------------------------------------------------|
| `file_path`         | string  | ❌        | Absolute path to file to convert in place        |
| `text`              | string  | ❌        | Text to convert and return inline                |
| `keep_smart_quotes` | boolean | ❌        | Keep smart quotes and em-dashes (default: false) |

**Note**: Either `file_path` or `text` must be provided, but not both.

## Configuration

### Environment Variables

The American to English tool supports the following configuration options:

- **`M2E_MAX_LENGTH`**: Maximum length for text input in characters
  - **Default**: `40000`
  - **Description**: Controls the maximum length of text that can be processed (applies to both inline text and file content)
  - **Example**: `M2E_MAX_LENGTH=100000` allows processing text up to 100,000 characters

### Security Features

- **Input Length Validation**: Prevents processing of excessively large text that could impact performance
- **Resource Protection**: Configurable limits help maintain system stability while handling legitimate large documents
- **Error Handling**: Clear feedback when text exceeds configured limits
- **File Size Limits**: Both inline text and file content are subject to the same length restrictions

## Common Spelling Conversions

### Word Endings

| American | British  | Example                     |
|----------|----------|-----------------------------|
| -ize     | -ise     | organize → organise         |
| -ization | -isation | organization → organisation |
| -yze     | -yse     | analyze → analyse           |
| -or      | -our     | color → colour              |
| -er      | -re      | center → centre             |

### Specific Words

| American | British         |
|----------|-----------------|
| aluminum | aluminium       |
| defense  | defence         |
| license  | licence (noun)  |
| practice | practise (verb) |
| gray     | grey            |
| tire     | tyre            |
| curb     | kerb            |
| mom      | mum             |

### Double Letters

| American  | British    |
|-----------|------------|
| modeling  | modelling  |
| traveling | travelling |
| canceled  | cancelled  |
| labeled   | labelled   |

## Response Formats

### File Processing Response
```json
{
  "success": true,
  "message": "File converted successfully from American to British English",
  "file_path": "/path/to/document.md",
  "conversions_made": 15,
  "examples": [
    "color → colour",
    "organization → organisation",
    "analyze → analyse"
  ]
}
```

### Inline Conversion Response
```json
{
  "original_text": "The color of the aluminum center was optimized for organization.",
  "converted_text": "The colour of the aluminium centre was optimised for organisation.",
  "conversions_made": 4,
  "conversions": [
    {"position": 4, "from": "color", "to": "colour"},
    {"position": 17, "from": "aluminum", "to": "aluminium"},
    {"position": 26, "from": "center", "to": "centre"},
    {"position": 36, "from": "optimized", "to": "optimised"},
    {"position": 50, "from": "organization", "to": "organisation"}
  ]
}
```

### Error Response
```json
{
  "success": false,
  "error": "File not found: /invalid/path/document.md",
  "file_path": "/invalid/path/document.md"
}
```

## Common Use Cases

### Documentation Standardisation
Convert American documentation to British English:
```json
{
  "name": "murican_to_english",
  "arguments": {
    "file_path": "/docs/api-reference.md"
  }
}
```

### Content Editing
Convert text snippets during content creation:
```json
{
  "name": "murican_to_english",
  "arguments": {
    "text": "We're organizing a customization workshop to optimize our organization's color scheme."
  }
}
```

### Batch Processing Setup
Process multiple files systematically:
```json
// Process README
{
  "name": "murican_to_english",
  "arguments": {
    "file_path": "/project/README.md"
  }
}

// Process documentation
{
  "name": "murican_to_english",
  "arguments": {
    "file_path": "/project/docs/user-guide.md"
  }
}
```

### Quote Normalisation
Handle smart quotes and punctuation:
```json
{
  "name": "murican_to_english",
  "arguments": {
    "text": "The company's "modernization" efforts—including color changes—were successful.",
    "keep_smart_quotes": false
  }
}
```

## Workflow Integration

### Documentation Review Workflow
```bash
# 1. Convert documentation to British English
murican_to_english --file_path="/docs/user-guide.md"

# 2. Review changes and verify consistency
think "The conversion changed 23 American spellings to British equivalents. I should review the changes to ensure they maintain the intended meaning and consistency."

# 3. Store conversion decisions for future reference
memory create_entities --namespace="style_guide" --data='{"entities": [{"name": "British_English_Standards", "observations": ["Use -ise endings", "Colour not color", "Centre not center"]}]}'
```

### Content Creation Workflow
```bash
# 1. Create content with mixed spelling
# (Content written in American English)

# 2. Convert to British standard
murican_to_english --text="The organization will analyze the color optimization."

# 3. Use converted text in final document
think "The converted text maintains meaning while following British English conventions. I'll use this standardised version in the final documentation."
```

### Project Standardisation Workflow
```bash
# 1. Identify files needing conversion
# (Use find command to locate markdown/text files)

# 2. Convert key documentation files
murican_to_english --file_path="/project/README.md"
murican_to_english --file_path="/project/CONTRIBUTING.md"

# 3. Update style guide
memory add_observations --data='{"observations": [{"entityName": "Project_Style_Guide", "contents": ["Use British English spelling throughout", "Applied automatic conversion to existing docs"]}]}'
```

## Smart Quote Handling

### Default Behaviour (Normalise Quotes)
```json
{
  "name": "murican_to_english",
  "arguments": {
    "text": "The company's "modernization" efforts were successful."
  }
}
```
**Result**: `The company's "modernisation" efforts were successful.`

### Preserve Smart Quotes
```json
{
  "name": "murican_to_english",
  "arguments": {
    "text": "The company's "modernization" efforts were successful.",
    "keep_smart_quotes": true
  }
}
```
**Result**: `The company's "modernisation" efforts were successful.`

## File Types Supported

### Text-Based Files
- **Markdown**: `.md`, `.markdown`
- **Text**: `.txt`, `.text`
- **Documentation**: `.rst`, `.asciidoc`
- **Code Comments**: Most programming language files
- **Configuration**: `.yaml`, `.yml`, `.json` (string values)

### Recommended Use
- ✅ Documentation files
- ✅ README files
- ✅ User guides and manuals
- ✅ Blog posts and articles
- ✅ Configuration descriptions
- ⚠️ Code files (review changes carefully)
- ❌ Binary files (not supported)

## Best Practices

### Before Converting Files
1. **Backup important files**: Make copies before in-place conversion
2. **Review file types**: Ensure files are text-based
3. **Test with samples**: Try small sections first
4. **Check encoding**: Ensure UTF-8 encoding for best results

### After Conversion
1. **Review changes**: Check that conversions maintain intended meaning
2. **Test functionality**: Ensure converted content still works correctly
3. **Update style guides**: Document spelling standards for future content
4. **Train team members**: Ensure team understands British English conventions

### Context Considerations
- **Technical terms**: Some technical terms may have specific spellings
- **Brand names**: Don't convert proper nouns or brand names
- **Code examples**: Be careful with code snippets and variable names
- **User content**: Consider whether user-generated content should be converted

## Error Handling

### File Errors
- **File not found**: Check file path is absolute and correct
- **Permission denied**: Ensure write access to target file
- **File content too large**: File content exceeds maximum length limit (default: 40,000 characters, configurable via `M2E_MAX_LENGTH`)
- **Encoding issues**: Ensure files are UTF-8 encoded

### Text Errors
- **Empty input**: Provide non-empty text for conversion
- **Text too long**: Text exceeds maximum length limit (default: 40,000 characters, configurable via `M2E_MAX_LENGTH`)
- **Encoding problems**: Ensure text is properly encoded
- **Special characters**: Some characters may not convert correctly

## Integration Examples

### Automated Documentation Pipeline
```bash
# 1. Generate documentation (potentially with American spelling)
# 2. Convert to British English
murican_to_english --file_path="/generated/api-docs.md"

# 3. Verify conversion quality
think "The API documentation has been converted to British English. I should check that technical terms and code examples weren't incorrectly modified."

# 4. Commit standardised documentation
# (git commit with British English content)
```

### Content Review Process
```bash
# 1. Review content for American spellings
murican_to_english --text="Draft content with analyze, color, and organization."

# 2. Apply corrections and review
think "The conversion changed 'analyze' to 'analyse', 'color' to 'colour', and 'organization' to 'organisation'. These changes maintain meaning while following British conventions."

# 3. Update content standards
memory add_observations --data='{"observations": [{"entityName": "Content_Standards", "contents": ["Enforce British English spelling", "Review conversions for accuracy"]}]}'
```

---

For technical implementation details, see the [American to English source documentation](../../internal/tools/m2e/README.md).
