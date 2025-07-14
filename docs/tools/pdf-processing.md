# PDF Processing Tool

The PDF Processing tool provides fast and efficient text and image extraction from PDF files, converting them to structured Markdown with embedded image references.

## Overview

A lightweight, fast alternative to the full Document Processing tool specifically optimised for PDF files. Perfect when you need quick text extraction without the overhead of advanced analysis features.

## Features

- **Fast Extraction**: Optimised for speed and efficiency
- **Text and Images**: Extract both text content and embedded images
- **Page Ranges**: Process specific pages or page ranges
- **Markdown Output**: Clean, structured markdown format
- **Image Links**: Properly linked images in markdown
- **No Dependencies**: Self-contained, no external requirements
- **Cross-Platform**: Works on macOS and Linux

## When to Use PDF vs Document Processing

### Use PDF Processing When:
- ✅ Working with PDF files only
- ✅ Need fast extraction speed
- ✅ Want simple, lightweight processing
- ✅ Don't need OCR or diagram analysis
- ✅ Processing digital (non-scanned) PDFs

### Use Document Processing When:
- 📄 Working with multiple document formats (DOCX, XLSX, etc.)
- 🔍 Need OCR for scanned documents
- 🎨 Want diagram analysis and Mermaid generation
- ⚙️ Need advanced processing profiles
- 🧠 Require AI-powered content analysis

## Usage Examples

### Basic Usage
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/absolute/path/to/document.pdf"
  }
}
```

### Extract All Content
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/path/to/document.pdf",
    "extract_images": true,
    "output_dir": "/path/to/output"
  }
}
```

### Process Specific Pages
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/path/to/large-document.pdf", 
    "pages": "1-5",
    "extract_images": true
  }
}
```

### Page Range Examples
```json
// First 10 pages
{"pages": "1-10"}

// Specific pages
{"pages": "1,3,5,10"}

// Mixed ranges and pages
{"pages": "1-3,7,10-12"}

// All pages (default)
{"pages": "all"}
```

## Parameters Reference

### Required Parameters
| Parameter | Description | Example |
|-----------|-------------|---------|
| `file_path` | Absolute path to PDF file | `"/Users/john/documents/report.pdf"` |

### Optional Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `output_dir` | string | Same as PDF | Output directory for markdown and images |
| `extract_images` | boolean | `true` | Whether to extract embedded images |
| `pages` | string | `"all"` | Page range to process |

### Page Range Formats
- **All pages**: `"all"` (default)
- **Range**: `"1-5"` (pages 1 through 5)
- **Specific pages**: `"1,3,5"` (pages 1, 3, and 5)  
- **Mixed**: `"1-3,7,10-12"` (pages 1-3, 7, and 10-12)

## Output Structure

### Generated Files
```
/path/to/document.pdf
├── document.md          # Extracted markdown content
└── images/              # Extracted images directory
    ├── image_001.png
    ├── image_002.jpg
    └── ...
```

### Markdown Format
```markdown
# Document Title

## Page 1

Document content from first page...

![Image 1](images/image_001.png)

More content...

## Page 2

Content from second page...
```

## Response Format

### Successful Processing
```json
{
  "success": true,
  "message": "PDF processed successfully",
  "markdown_file": "/path/to/document.md",
  "images_extracted": 5,
  "images_directory": "/path/to/document/images",
  "pages_processed": 10,
  "processing_time": 2.3,
  "file_size": 1024000
}
```

### Error Response
```json
{
  "success": false,
  "error": "File not found: /invalid/path/document.pdf",
  "file_path": "/invalid/path/document.pdf"
}
```

## Common Use Cases

### Quick Document Review
Extract text for quick review or analysis:
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/downloads/research-paper.pdf",
    "extract_images": false
  }
}
```

### Documentation Conversion
Convert PDF documentation to markdown:
```json
{
  "name": "pdf", 
  "arguments": {
    "file_path": "/docs/api-reference.pdf",
    "extract_images": true,
    "output_dir": "/project/docs"
  }
}
```

### Large Document Processing
Process specific sections of large documents:
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/reports/annual-report-2024.pdf",
    "pages": "5-15",
    "extract_images": true
  }
}
```

### Batch Processing Preparation
Extract specific pages for further processing:
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/contracts/agreement.pdf",
    "pages": "1,5,10-12",
    "extract_images": false
  }
}
```

## Performance Characteristics

### Processing Speed
- **Small PDFs** (< 10 pages): 1-3 seconds
- **Medium PDFs** (10-50 pages): 3-15 seconds  
- **Large PDFs** (50+ pages): 15+ seconds

### Factors Affecting Speed
- **Page count**: Linear relationship with processing time
- **Image content**: Images slow down processing
- **Text complexity**: Tables and complex layouts take longer
- **File size**: Large embedded images impact speed

### Memory Usage
- **Text processing**: Low memory usage
- **Image extraction**: Moderate memory for large images
- **Large documents**: Memory usage scales with content

## Comparison with Document Processing

| Feature | PDF Processing | Document Processing |
|---------|---------------|-------------------|
| **Speed** | ⚡ Fast (1-15 seconds) | 🐌 Slower (10-60+ seconds) |
| **File Types** | PDF only | PDF, DOCX, XLSX, PPTX, HTML, CSV, images |
| **Dependencies** | None | Python 3.10+, Docling |
| **OCR Support** | ❌ No | ✅ Yes |
| **Diagram Analysis** | ❌ No | ✅ Yes with AI models |
| **Setup Complexity** | ✅ Simple | ⚙️ Complex |
| **Resource Usage** | 🟢 Low | 🟡 Moderate to High |

## Integration Examples

### Research Workflow
```bash
# 1. Quick PDF extraction
pdf_extract="/path/to/research.pdf"

# 2. Process with think tool
think "I've extracted the research paper content. Let me analyse the key findings and methodology before proceeding with implementation."

# 3. Store key insights  
memory create_entities --data '{"entities": [{"name": "Research_Paper_2024", "type": "document", "observations": ["Novel approach to distributed consensus", "Improves performance by 40%"]}]}'
```

### Documentation Workflow
```bash
# 1. Extract PDF documentation
pdf_extract="/technical-specs/api-guide.pdf" --pages="1-20" 

# 2. Search for additional information
internet_search "REST API best practices 2024"

# 3. Combine insights for implementation
think "The PDF provides specific implementation details, while the search results show current best practices. I'll combine both for the recommended approach."
```

### Content Analysis Workflow
```bash
# 1. Extract content from multiple PDFs
pdf_extract="/reports/q1-report.pdf" --pages="1-5"
pdf_extract="/reports/q2-report.pdf" --pages="1-5"

# 2. Store extracted insights
memory create_entities --namespace="quarterly_reports" --data='{...}'

# 3. Analyse trends
think "Comparing Q1 and Q2 reports, I can see a clear trend in customer acquisition costs and revenue growth patterns."
```

## Error Handling

### Common Errors
- **File not found**: Invalid file path
- **Permission denied**: Insufficient file access rights
- **Corrupted PDF**: Damaged or invalid PDF file
- **Unsupported PDF**: Encrypted or password-protected PDFs
- **Disk space**: Insufficient space for output files

### Error Prevention
1. **Use absolute paths**: Avoid relative path issues
2. **Check file existence**: Verify file exists before processing
3. **Verify permissions**: Ensure read access to PDF and write access to output directory
4. **Test with small files**: Validate setup with simple PDFs first

## Advanced Usage

### Custom Output Organisation
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/source/document.pdf",
    "output_dir": "/organised/content/document-name",
    "extract_images": true,
    "pages": "1-10"
  }
}
```

### Selective Processing for Large Documents
```json
// Process table of contents and summary
{
  "name": "pdf", 
  "arguments": {
    "file_path": "/reports/annual-report.pdf",
    "pages": "1-3,50-55",
    "extract_images": false
  }
}
```

### Image-Only Extraction
```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/presentations/slides.pdf", 
    "extract_images": true,
    "pages": "10-20"
  }
}
```

---

For technical implementation details, see the [PDF Processing source documentation](../../internal/tools/pdf/README.md).