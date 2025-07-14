# Document Processing Tool

The Document Processing tool provides intelligent document conversion capabilities for PDF, DOCX, XLSX, PPTX, HTML, CSV, PNG, and JPG files using the powerful [Docling](https://docling-project.github.io/docling/) library.

## Overview

Convert documents to structured Markdown while preserving formatting, extracting tables, images, and metadata. The tool offers processing profiles for different use cases, from simple text extraction to advanced diagram analysis with AI models.

**Note**: This tool is experimental and actively developed.

## Features

- **Multi-format Support**: PDF, DOCX, XLSX, PPTX, HTML, CSV, PNG, JPG
- **Processing Profiles**: Simplified interface with preset configurations
- **Intelligent Conversion**: Preserves document structure and formatting
- **OCR Support**: Extract text from scanned documents
- **Hardware Acceleration**: Supports MPS (macOS), CUDA, and CPU processing
- **Caching System**: Avoids reprocessing identical documents
- **Metadata Extraction**: Document metadata (title, author, page count, etc.)
- **Table & Image Extraction**: Preserves tables and images in markdown
- **Diagram Analysis**: Advanced diagram detection using vision models
- **Mermaid Generation**: Convert diagrams to editable Mermaid syntax
- **Auto-Save**: Automatically saves processed content to files

## Quick Start

First ensure docling is installed in the environment you'll be running the MCP Server from:

```shell
pip install -U pip docling
```

### Simple Usage (Recommended)
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf"
  }
}
```

This uses the default `text-and-image` profile and saves to `/path/to/document.md`.

## Processing Profiles

### `basic` - Fast Text Extraction
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf",
    "profile": "basic"
  }
}
```
- Text extraction only
- Fastest processing
- No image or diagram analysis
- **Best for**: Simple text documents, quick content extraction

### `text-and-image` - Balanced Processing (Default)
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf",
    "profile": "text-and-image"
  }
}
```
- Text and image extraction
- Table processing
- Good balance of speed and features
- **Best for**: Most document types, general use

### `scanned` - OCR Processing
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/scanned-document.pdf",
    "profile": "scanned"
  }
}
```
- Optimised for scanned documents
- OCR enabled by default
- **Best for**: Image-based PDFs, scanned documents

### `llm-smoldocling` - Vision Enhancement
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf",
    "profile": "llm-smoldocling"
  }
}
```
- Enhanced with SmolDocling vision model
- Diagram detection and description
- Chart data extraction
- No external LLM required
- **Best for**: Documents with diagrams and charts

### `llm-external` - Advanced Diagram Processing
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf",
    "profile": "llm-external"
  }
}
```
- Full diagram-to-Mermaid conversion
- Requires LLM environment variables
- Most advanced processing capabilities
- **Best for**: Complex documents with many diagrams
- **Requires**: LLM configuration (see setup below)

## Output Options

### Save to File (Default)
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf"
  }
}
```
- Saves to `/path/to/document.md`
- Images saved in same directory
- Returns success message with file path

### Custom Save Location
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf",
    "save_to": "/custom/path/output.md"
  }
}
```

### Return Content Inline
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf",
    "return_inline_only": true
  }
}
```

## Setup and Configuration

### Prerequisites
- **Python 3.10+** (ideally 3.13+)
- **Docling** (auto-installed if missing)

The tool will attempt to install Docling automatically if not found.

### Environment Variables

#### Python Configuration
```bash
DOCLING_PYTHON_PATH="/path/to/python"  # Auto-detected if not set
```

#### Cache Configuration
```bash
DOCLING_CACHE_DIR="~/.mcp-devtools/docling-cache"
DOCLING_CACHE_ENABLED="true"
```

#### Hardware Acceleration
```bash
DOCLING_HARDWARE_ACCELERATION="auto"  # auto, mps, cuda, cpu
```

#### Processing Configuration
```bash
DOCLING_TIMEOUT="300"        # 5 minutes
DOCLING_MAX_FILE_SIZE="100"  # 100 MB
```

#### OCR Configuration
```bash
DOCLING_OCR_LANGUAGES="en,fr,de"
```

#### LLM Configuration (for `llm-external` profile)
```bash
DOCLING_VLM_API_URL="http://localhost:11434/v1"     # OpenAI-compatible endpoint
DOCLING_VLM_MODEL="mistral-small3.2:24b"           # Vision-capable model
DOCLING_VLM_API_KEY="your-api-key-here"            # API key
```

### Corporate Network Setup
For environments with MITM proxies:
```bash
DOCLING_EXTRA_CA_CERTS="/path/to/mitm-ca-bundle.pem"
```

## OCR (Optical Character Recognition)

### When to Use OCR

**OCR Disabled (Default):**
- **Best for**: Digital documents (native PDFs, Word documents)
- **Advantages**: Faster, perfect accuracy, preserves formatting
- **How it works**: Extracts text directly from document structure

**OCR Enabled (`scanned` profile):**
- **Best for**: Scanned documents, image-based PDFs, photos
- **Advantages**: Processes any document type, handles handwritten text
- **How it works**: Uses computer vision to recognise text from images

### OCR Language Support
```json
{
  "name": "process_document",
  "arguments": {
    "profile": "scanned",
    "ocr_languages": ["en", "fr", "de", "es"]
  }
}
```

Supported languages: English (en), French (fr), German (de), Spanish (es), Italian (it), Portuguese (pt), Dutch (nl), Russian (ru), Chinese (zh), Japanese (ja), Korean (ko), and many others.

## Diagram Analysis and Mermaid Generation

### Basic Diagram Analysis
The `llm-smoldocling` profile uses built-in vision models:
- Automatic diagram detection
- Type classification with confidence scores
- Element extraction
- No external services required

### Advanced Mermaid Generation
The `llm-external` profile converts diagrams to Mermaid syntax:

#### Supported LLM Providers
- **Ollama** (local): `http://localhost:11434/v1`
- **LM Studio** (local): `http://localhost:1234/v1`
- **OpenAI**: `https://api.openai.com/v1`
- **OpenRouter**: `https://openrouter.ai/api/v1`

#### LLM Configuration
```bash
export DOCLING_VLM_API_URL="http://localhost:11434/v1"
export DOCLING_VLM_MODEL="llava:latest"
export DOCLING_VLM_API_KEY="your-api-key"
export DOCLING_LLM_MAX_TOKENS="16384"
export DOCLING_LLM_TEMPERATURE="0.1"
export DOCLING_LLM_TIMEOUT="240"
```

#### Diagram Features
- **Automatic Detection**: Identifies flowcharts, architecture diagrams, charts
- **Mermaid Conversion**: Generates valid Mermaid syntax
- **AWS Colour Coding**: Consistent colour schemes for architecture diagrams
- **Validation**: Validates generated Mermaid syntax
- **Fallback Handling**: Graceful degradation if LLM unavailable

## Response Examples

### File Save Response
```json
{
  "success": true,
  "message": "Content successfully exported to file",
  "save_path": "/path/to/document.md",
  "source": "/path/to/document.pdf",
  "cache_hit": false,
  "metadata": {
    "file_size": 15420,
    "document_title": "Document Title",
    "document_author": "Author Name",
    "page_count": 10,
    "word_count": 1500
  },
  "processing_info": {
    "processing_mode": "advanced",
    "processing_method": "advanced+vision:standard",
    "hardware_acceleration": "mps",
    "ocr_enabled": false,
    "processing_time": 2.5,
    "timestamp": "2025-07-09T22:12:15+10:00"
  }
}
```

### Inline Content Response
```json
{
  "source": "/path/to/document.pdf",
  "content": "# Document Title\n\nDocument content in markdown...",
  "cache_hit": false,
  "metadata": {
    "title": "Document Title",
    "author": "Author Name",
    "page_count": 10
  },
  "images": [
    {
      "id": "image_1",
      "type": "picture",
      "caption": "Figure 1",
      "file_path": "/path/to/extracted/image_1.png"
    }
  ],
  "diagrams": [
    {
      "id": "diagram_1",
      "type": "flowchart",
      "description": "Process flow diagram showing...",
      "mermaid_code": "flowchart TD\n    A[Start] --> B[Process]\n    B --> C[End]",
      "confidence": 0.95
    }
  ]
}
```

## Performance

### Profile Performance (Typical Document)
- **`basic`**: 1-3 seconds
- **`text-and-image`**: 3-10 seconds
- **`scanned`**: 10-30 seconds
- **`llm-smoldocling`**: 5-15 seconds
- **`llm-external`**: 15-60 seconds

### Hardware Impact
- **CPU**: Baseline performance
- **MPS (macOS)**: 2-5x faster on Apple Silicon
- **CUDA**: 3-10x faster on NVIDIA GPUs

### Caching
Intelligent caching based on:
- Document source and modification time
- Processing parameters and profile
- 24-hour TTL by default

## Common Use Cases

### Research Document Analysis
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/research-paper.pdf",
    "profile": "llm-smoldocling"
  }
}
```

### Scanned Document Digitisation
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/scanned-invoice.pdf",
    "profile": "scanned"
  }
}
```

### Architecture Documentation
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/architecture-doc.pdf",
    "profile": "llm-external"
  }
}
```

### Quick Text Extraction
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/simple-doc.pdf",
    "profile": "basic"
  }
}
```

## Troubleshooting

### Common Issues

**"Python path is required but not found"**
- Install Python 3.10+ and ensure it's in PATH
- Or set `DOCLING_PYTHON_PATH` environment variable

**"Docling not available"**
- Install: `pip install docling`
- Verify: `python -c "import docling; print('OK')"`

**"Processing timeout"**
- Increase `DOCLING_TIMEOUT` environment variable
- Use faster profile (`basic` instead of `llm-external`)

**"Hardware acceleration not working"**
- Install appropriate PyTorch version
- Check: `python -c "import torch; print(torch.backends.mps.is_available())"`

**"LLM external profile not available"**
- Set all `DOCLING_LLM_*` environment variables
- Verify LLM endpoint accessibility
- Ensure model supports vision input

### Debug Mode
```json
{
  "name": "process_document",
  "arguments": {
    "source": "/path/to/document.pdf",
    "debug": true
  }
}
```

---

For technical implementation details, see the [Document Processing source documentation](../../internal/tools/docprocessing/README.md).
