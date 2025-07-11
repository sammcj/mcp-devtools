# PDF Processing Tool

This tool provides PDF text and image extraction capabilities using the [pdfcpu](https://github.com/pdfcpu/pdfcpu) library. It processes PDF files to extract content while attempting to retain formatting and accurate layout, creating markdown files with embedded image references.

## Features

- **Text Extraction**: Extracts text content from PDF pages using pdfcpu's content extraction
- **Image Extraction**: Extracts embedded images from PDFs with proper naming and organisation
- **Markdown Output**: Generates well-formatted markdown with preserved structure
- **Multi-page Support**: Process all pages or specific page ranges
- **Automatic Linking**: Links extracted images in the correct locations within the markdown
- **Flexible Output**: Choose output directory or use the same directory as the source PDF

## Usage

### Basic Usage

```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/absolute/path/to/document.pdf"
  }
}
```

### Advanced Usage

```json
{
  "name": "pdf",
  "arguments": {
    "file_path": "/absolute/path/to/document.pdf",
    "output_dir": "/absolute/path/to/output",
    "extract_images": true,
    "pages": "1-5"
  }
}
```

## Parameters

- **`file_path`** (required): Absolute file path to the PDF document to process
- **`output_dir`** (optional): Output directory for markdown and images (defaults to same directory as PDF)
- **`extract_images`** (optional): Whether to extract images from the PDF (default: true)
- **`pages`** (optional): Page range to process. Options:
  - `"all"` - Process all pages (default)
  - `"1-5"` - Process pages 1 through 5
  - `"1,3,5"` - Process pages 1, 3, and 5
  - `"1-3,7,10-12"` - Process pages 1-3, 7, and 10-12

## Output

The tool creates:

1. **Markdown File**: `{basename}.md` in the output directory containing:
   - Document title and metadata
   - Page-by-page content with headers
   - Embedded image references where appropriate

2. **Image Directory**: `{basename}_images/` containing:
   - All extracted images with descriptive names
   - Images organised by page number

3. **JSON Response**: Contains:
   - Path to generated markdown file
   - List of extracted image paths
   - Processing statistics

## Example Output Structure

```
/output/directory/
├── document.md                 # Generated markdown
└── document_images/           # Extracted images
    ├── document_page_1_img_1.jpg
    ├── document_page_1_img_2.png
    ├── document_page_2_img_1.jpg
    └── ...
```

## Limitations

- **Text Quality**: Text extraction quality depends on the PDF structure. Complex layouts may not be perfectly preserved.
- **Font Dependencies**: Some text rendering may be affected by missing font information.
- **Table Extraction**: Tables are extracted as raw text without structure preservation.
- **Scanned PDFs**: This tool does not perform OCR on scanned documents.

## Technical Details

The tool uses pdfcpu's content extraction capabilities:

- `api.ExtractContentFile()` for raw page content
- `api.ExtractImagesFile()` for image extraction  
- `api.PageCountFile()` for page counting

The extracted content is processed to:
1. Remove PDF-specific commands and directives
2. Extract readable text from PDF text operations
3. Format content as markdown with appropriate headers
4. Link extracted images in context

## Error Handling

The tool provides detailed error messages for:
- Invalid file paths or missing files
- Unsupported file formats
- Invalid page ranges
- Extraction failures

Failed pages are noted in the output with appropriate error messages, allowing partial processing to continue.

## Dependencies

- [pdfcpu](https://github.com/pdfcpu/pdfcpu) v0.11.0+
- Go standard library packages for file operations and text processing