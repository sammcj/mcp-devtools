# Document Processing Tool

The Document Processing tool provides intelligent document conversion capabilities for PDF, DOCX, XLSX, and PPTX files using the powerful [Docling](https://docling-project.github.io/docling/) library. It converts documents to structured Markdown while preserving formatting, extracting tables, images, and metadata.

## Features

- **Multi-format Support**: PDF, DOCX, XLSX, PPTX document processing
- **Intelligent Conversion**: Preserves document structure and formatting
- **OCR Support**: Extract text from scanned documents
- **Hardware Acceleration**: Supports MPS (macOS), CUDA, and CPU processing
- **Caching System**: Intelligent caching to avoid reprocessing identical documents
- **Metadata Extraction**: Extracts document metadata (title, author, page count, etc.)
- **Table & Image Extraction**: Preserves tables and images in markdown format
- **Diagram Analysis**: Advanced diagram detection and description using vision models
- **Mermaid Generation**: Convert diagrams to editable Mermaid syntax using external LLM
- **Flexible Processing Modes**: Basic, advanced, OCR-focused, table-focused, and image-focused modes

## Installation

### Prerequisites

Note: mcp-devtools will attempt to install the docling package if it's unavailable.

1. **Python 3.13+** with Docling installed:
   ```bash
   pip install docling
   ```

2. **Optional: Hardware Acceleration**
   - **macOS**: Install PyTorch with MPS support
   - **NVIDIA GPUs**: Install PyTorch with CUDA support
   - **CPU**: Works out of the box

### Configuration

The tool can be configured via environment variables:

```bash
# Python Configuration
export DOCLING_PYTHON_PATH="/path/to/python"  # Auto-detected if not set

# Cache Configuration
export DOCLING_CACHE_DIR="~/.mcp-devtools/docling-cache"
export DOCLING_CACHE_ENABLED="true"

# Hardware Acceleration
export DOCLING_HARDWARE_ACCELERATION="auto"  # auto, mps, cuda, cpu

# Processing Configuration
export DOCLING_TIMEOUT="300"        # 5 minutes
export DOCLING_MAX_FILE_SIZE="100"  # 100 MB

# OCR Configuration
export DOCLING_OCR_LANGUAGES="en,fr,de"

# Vision Model Configuration
export DOCLING_VISION_MODEL="SmolDocling"

# Certificate Configuration (for MITM proxies)
export DOCLING_EXTRA_CA_CERTS="/path/to/mitm-ca-bundle.pem"
```

## Usage

### Basic Usage

```json
{
  "source": "/path/to/document.pdf"
}
```

### Advanced Usage

```json
{
  "source": "/path/to/document.pdf",
  "processing_mode": "advanced",
  "enable_ocr": true,
  "ocr_languages": ["en", "fr"],
  "preserve_images": true,
  "output_format": "markdown",
  "cache_enabled": true,
  "timeout": 600,
  "max_file_size": 200
}
```

### Processing Modes

- **`basic`** (default): Fast processing using code-only parsing
- **`advanced`**: Uses vision models for better structure recognition
- **`ocr`**: Optimised for scanned documents with OCR
- **`tables`**: Focus on accurate table extraction
- **`images`**: Focus on image extraction and preservation

### OCR (Optical Character Recognition)

The tool supports OCR processing for extracting text from scanned documents and images. Understanding when to use OCR is important for optimal results:

#### OCR Disabled (Default)
- **Best for**: Digital documents (native PDFs, Word documents, Excel files)
- **How it works**: Extracts text directly from the document's digital structure
- **Advantages**:
  - Faster processing
  - Perfect text accuracy (no recognition errors)
  - Preserves original formatting and fonts
  - Lower resource usage
- **Limitations**: Cannot process scanned documents or image-based PDFs

#### OCR Enabled
- **Best for**: Scanned documents, image-based PDFs, photos of documents
- **How it works**: Uses computer vision to recognise text from images
- **Advantages**:
  - Can process any document type, including scanned pages
  - Handles handwritten text (with varying accuracy)
  - Works with photos and screenshots of documents
- **Limitations**:
  - Slower processing
  - May introduce text recognition errors
  - Formatting may not be perfectly preserved
  - Higher resource usage

#### When to Use Each Mode

**Use OCR Disabled when:**
- Processing native digital documents (Word, Excel, native PDFs)
- You need perfect text accuracy
- Speed is important
- The document has complex formatting you want to preserve

**Use OCR Enabled when:**
- Processing scanned documents or PDFs created from scans
- Working with image files (PNG, JPEG) containing text
- The document fails to process with OCR disabled
- You need to extract text from photos of documents

#### OCR Language Support

When OCR is enabled, you can specify languages for better recognition accuracy:

```json
{
  "enable_ocr": true,
  "ocr_languages": ["en", "fr", "de", "es"]
}
```

Supported languages include: English (en), French (fr), German (de), Spanish (es), Italian (it), Portuguese (pt), Dutch (nl), Russian (ru), Chinese (zh), Japanese (ja), Korean (ko), and many others.

### Diagram Analysis and Mermaid Generation

The tool supports advanced diagram analysis and conversion to Mermaid syntax using external LLM vision models.

#### Basic Diagram Analysis

Enable diagram detection and description using the built-in SmolDocling vision model:

```json
{
  "source": "/path/to/document.pdf",
  "diagram_description": true,
  "vision_mode": "smoldocling"
}
```

#### LLM-Enhanced Mermaid Generation

For advanced diagram-to-Mermaid conversion, configure external LLM integration:

```bash
# Required environment variables for LLM integration
export DOCLING_LLM_OPENAI_API_BASE="https://api.openai.com/v1"     # Or any OpenAI-compatible endpoint
export DOCLING_LLM_MODEL_NAME="gpt-4-vision-preview"        # Model name for the provider
export DOCLING_LLM_OPENAI_API_KEY="your-api-key-here"              # API key for authentication

# Optional LLM configuration (with defaults)
export DOCLING_LLM_MAX_TOKENS="16384"        # Maximum tokens for LLM response
export DOCLING_LLM_TEMPERATURE="0.1"        # Temperature for LLM inference
export DOCLING_LLM_TIMEOUT="240"             # Timeout for LLM requests in seconds
```

Then enable enhanced diagram generation:

```json
{
  "source": "/path/to/document.pdf",
  "diagram_description": true,
  "generate_diagrams": true,
  "vision_mode": "smoldocling"
}
```

#### Supported LLM Providers

The tool supports any OpenAI compatible API endpoint, e.g: Ollama, LM Studio, OpenRouter, OpenAI, Google etc. via their OpenAI compatible APIs as long as you select a model that supports vision input.

#### Custom Prompts

You can customise the prompts used for diagram analysis:

```bash
# Base prompt for all diagram analysis
export DOCLING_LLM_PROMPT_BASE="You are an expert at analysing diagrams..."

# Type-specific prompts
export DOCLING_LLM_PROMPT_FLOWCHART="This appears to be a flowchart..."
export DOCLING_LLM_PROMPT_ARCHITECTURE="This appears to be an architecture diagram..."
export DOCLING_LLM_PROMPT_CHART="This appears to be a chart or graph..."
export DOCLING_LLM_PROMPT_GENERIC="Analyse this diagram and..."
```

#### Diagram Analysis Features

- **Automatic Detection**: Identifies diagrams, flowcharts, architecture diagrams, and charts
- **Type Classification**: Classifies diagram types with confidence scoring
- **Mermaid Conversion**: Generates valid Mermaid syntax for diagrams
- **Element Extraction**: Extracts text elements and structural components
- **AWS Colour Coding**: Applies consistent colour schemes for architecture diagrams
- **Validation**: Validates generated Mermaid syntax for correctness
- **Fallback Handling**: Gracefully falls back to basic analysis if LLM is unavailable

### Output Formats

- **`markdown`** (default): Returns processed content as Markdown
- **`json`**: Returns metadata only
- **`both`**: Returns both content and detailed metadata

## Response Format

```json
{
  "source": "/path/to/document.pdf",
  "content": "# Document Title\n\nDocument content in markdown...",
  "cache_hit": false,
  "metadata": {
    "title": "Document Title",
    "author": "Author Name",
    "subject": "Document Subject",
    "page_count": 10,
    "word_count": 1500
  },
  "processing_info": {
    "processing_mode": "basic",
    "hardware_acceleration": "mps",
    "ocr_enabled": false,
    "processing_time": "2.5s",
    "cache_key": "abc123...",
    "timestamp": "2025-07-08T17:56:05+10:00"
  }
}
```

## Error Handling

The tool provides detailed error information:

```json
{
  "source": "/path/to/document.pdf",
  "error": "Processing failed: File not found",
  "system_info": {
    "platform": "darwin",
    "python_available": true,
    "docling_available": false,
    "hardware_acceleration": ["cpu", "mps"]
  }
}
```

## Architecture

### Components

1. **DocumentProcessorTool**: Main MCP tool interface
2. **Config**: Configuration management with environment variable support
3. **CacheManager**: Intelligent caching system with TTL support
4. **Python Wrapper**: Subprocess interface to Docling Python library

### File Structure

```
internal/tools/docprocessing/
├── README.md                    # This file
├── document_processor.go        # Main tool implementation
├── types.go                     # Type definitions
├── config.go                    # Configuration management
├── cache.go                     # Caching system
└── scripts/
    └── docling_processor.py     # Python wrapper script
```

### Python Wrapper

The tool uses a Python subprocess wrapper (`scripts/docling_processor.py`) that:

- Handles Docling library integration
- Manages hardware acceleration detection and configuration
- Provides structured JSON output
- Handles errors gracefully
- Supports multiple processing modes

## Performance

### Caching

The tool implements intelligent caching based on:
- Document source (file path/URL)
- Processing parameters (mode, OCR settings, etc.)
- File modification time (for local files)

Cache entries have a 24-hour TTL by default and are stored as JSON files.

### Hardware Acceleration

Processing performance varies by hardware:
- **CPU**: Baseline performance, works everywhere but slow
- **MPS (macOS)**: 2-5x faster on Apple Silicon
- **CUDA**: 3-10x faster on NVIDIA GPUs

## Use With Custom MITM Certs

The document processing tool performs a pip install docling (if it's not found) and if you choose to use the advanced vLLM processing also has to download the SmolDocling model, as such some corporate environments that use MITM privacy-breaking proxies may need additional certs provided. Set the `DOCLING_EXTRA_CA_CERTS` environment variable to point to your certificate bundle:

```bash
export DOCLING_EXTRA_CA_CERTS="/path/to/mitm-ca-bundle.pem" # or a directory
```

### Supported Certificate Formats

The tool supports the following certificate file formats:
- `.pem` - PEM encoded certificates
- `.crt` - Certificate files
- `.cer` - Certificate files
- `.ca-bundle` - Certificate bundles

### How It Works

When `DOCLING_EXTRA_CA_CERTS` is configured, the tool automatically sets the following environment variables for all Python subprocess calls:

- `SSL_CERT_FILE` - For general SSL/TLS connections
- `REQUESTS_CA_BUNDLE` - For Python requests library
- `CURL_CA_BUNDLE` - For curl-based downloads
- `PIP_CERT` - For pip package installations
- `CONDA_SSL_VERIFY` - For conda package installations (if detected and used)

## Troubleshooting

### Common Issues

1. **"Python path is required but not found"**
   - Install Python 3.10+ (ideally 3.13+) and ensure it's in PATH
   - Or set `DOCLING_PYTHON_PATH` environment variable

2. **"Docling not available"**
   - Install Docling: `pip install docling`
   - Verify installation: `python -c "import docling; print('OK')"`

3. **"Processing timeout"**
   - Increase timeout with `DOCLING_TIMEOUT` environment variable
   - Or pass `timeout` parameter in request

4. **"Hardware acceleration not working"**
   - Install appropriate PyTorch version for your hardware
   - Check system compatibility with `python -c "import torch; print(torch.backends.mps.is_available())"`

5. **"Certificate path does not exist"**
   - Verify the path specified in `DOCLING_EXTRA_CA_CERTS` exists
   - Ensure the certificate file or directory is readable
   - Check that certificate files have the correct extensions (.pem, .crt, .cer, .ca-bundle)

### Debug Mode

Enable debug logging by setting the MCP server to debug mode. The tool logs processing steps and performance metrics.

## Examples

### Process a PDF with OCR

```bash
echo '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "process_document",
    "arguments": {
      "source": "/path/to/scanned.pdf",
      "processing_mode": "ocr",
      "enable_ocr": true,
      "ocr_languages": ["en"]
    }
  }
}' | mcp-devtools stdio
```

### Process a DOCX with table focus

```bash
echo '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "process_document",
    "arguments": {
      "source": "/path/to/document.docx",
      "processing_mode": "tables",
      "preserve_images": true
    }
  }
}' | mcp-devtools stdio
```

### Process a PDF with diagram-to-Mermaid conversion

```bash
# First, set up LLM environment variables
export DOCLING_LLM_OPENAI_API_BASE="https://api.openai.com/v1"
export DOCLING_LLM_MODEL_NAME="gpt-4-vision-preview"
export DOCLING_LLM_OPENAI_API_KEY="your-api-key-here"

# Then process the document
echo '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "process_document",
    "arguments": {
      "source": "/path/to/document-with-diagrams.pdf",
      "diagram_description": true,
      "generate_diagrams": true,
      "vision_mode": "smoldocling"
    }
  }
}' | mcp-devtools stdio
```

## License

This tool is part of the mcp-devtools project and follows the same license terms.
