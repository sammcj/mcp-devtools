# Document Processing Tool

The Document Processing tool provides intelligent document conversion capabilities for PDF, DOCX, XLSX, PPTX, HTML, CSV, PNG, and JPG files using the powerful [Docling](https://docling-project.github.io/docling/) library. It converts documents to structured Markdown while preserving formatting, extracting tables, images, and metadata.

## Features

- **Multi-format Support**: PDF, DOCX, XLSX, PPTX, HTML, CSV, PNG, JPG document processing
- **Processing Profiles**: Simplified interface with preset configurations for common use cases
- **Intelligent Conversion**: Preserves document structure and formatting
- **OCR Support**: Extract text from scanned documents
- **Hardware Acceleration**: Supports MPS (macOS), CUDA, and CPU processing
- **Caching System**: Intelligent caching to avoid reprocessing identical documents
- **Metadata Extraction**: Extracts document metadata (title, author, page count, etc.)
- **Table & Image Extraction**: Preserves tables and images in markdown format
- **Diagram Analysis**: Advanced diagram detection and description using vision models
- **Mermaid Generation**: Convert diagrams to editable Mermaid syntax by using an external LLM provider
- **Auto-Save**: Automatically saves processed content to files by default

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
DOCLING_PYTHON_PATH="/path/to/python"  # Auto-detected if not set

# Cache Configuration
DOCLING_CACHE_DIR="~/.mcp-devtools/docling-cache"
DOCLING_CACHE_ENABLED="true"

# Hardware Acceleration
DOCLING_HARDWARE_ACCELERATION="auto"  # auto, mps, cuda, cpu

# Processing Configuration
DOCLING_TIMEOUT="300"        # 5 minutes
DOCLING_MAX_FILE_SIZE="100"  # 100 MB

# OCR Configuration
DOCLING_OCR_LANGUAGES="en,fr,de"

# Vision Model Configuration
DOCLING_VISION_MODEL="SmolDocling"

# Certificate Configuration (for MITM proxies)
DOCLING_EXTRA_CA_CERTS="/path/to/mitm-ca-bundle.pem"

# LLM Configuration (for advanced diagram processing)
DOCLING_LLM_OPENAI_API_BASE="http://localhost:11434/v1"
DOCLING_LLM_MODEL_NAME="mistral-small-3.2-24b-2506-ud:q6_k_xl"
DOCLING_LLM_OPENAI_API_KEY="your-api-key-here"
```

## Usage

### Simple Usage (Recommended)

The tool now features a simplified interface using processing profiles that automatically configure all necessary parameters:

```json
{
  "source": "/path/to/document.pdf"
}
```

This uses the default `text-and-image` profile and automatically saves the processed content to `/path/to/document.md`.

### Processing Profiles

Choose from preset profiles that configure multiple parameters automatically:

#### `basic` - Fast Text Extraction
```json
{
  "source": "/path/to/document.pdf",
  "profile": "basic"
}
```
- Text extraction only
- Fastest processing
- No image or diagram analysis

#### `text-and-image` - Balanced Processing (Default)
```json
{
  "source": "/path/to/document.pdf",
  "profile": "text-and-image"
}
```
- Text and image extraction
- Table processing
- Good balance of speed and features

#### `scanned` - OCR Processing
```json
{
  "source": "/path/to/scanned-document.pdf",
  "profile": "scanned"
}
```
- Optimised for scanned documents
- OCR enabled by default
- Best for image-based PDFs

#### `llm-smoldocling` - Vision Enhancement
```json
{
  "source": "/path/to/document.pdf",
  "profile": "llm-smoldocling"
}
```
- Enhanced with SmolDocling vision model
- Diagram detection and description
- Chart data extraction
- No external LLM required
- Slower than `text-and-image`

#### `llm-external` - Advanced Diagram Processing
```json
{
  "source": "/path/to/document.pdf",
  "profile": "llm-external"
}
```
- Full diagram-to-Mermaid conversion
- Requires LLM environment variables
- Most advanced processing capabilities
- Slower processing time
- Best for documents with diagrams and charts
- Only available when `DOCLING_LLM_*` environment variables are configured

### Output Control

#### Save to File (Default)
```json
{
  "source": "/path/to/document.pdf"
}
```
- Automatically saves to `/path/to/document.md` and if images are extracted, they will be saved in the same directory
- Returns success message with file path

#### Custom Save Location
```json
{
  "source": "/path/to/document.pdf",
  "save_to": "/custom/path/output.md"
}
```
- Saves to specified location
- Must be an absolute path

#### Return Content Inline
```json
{
  "source": "/path/to/document.pdf",
  "inline": true
}
```
- Returns content in the response
- No file is saved

## OCR (Optical Character Recognition)

The tool supports OCR processing for extracting text from scanned documents and images. Understanding when to use OCR is important for optimal results:

### OCR Disabled (Default for most profiles)
- **Best for**: Digital documents (native PDFs, Word documents, Excel files)
- **How it works**: Extracts text directly from the document's digital structure
- **Advantages**:
  - Faster processing
  - Perfect text accuracy (no recognition errors)
  - Preserves original formatting and fonts
  - Lower resource usage
- **Limitations**: Cannot process scanned documents or image-based PDFs

### OCR Enabled (Default for `scanned` profile)
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

### OCR Language Support

When using the `scanned` profile or enabling OCR manually, you can specify languages:

```json
{
  "profile": "scanned",
  "ocr_languages": ["en", "fr", "de", "es"]
}
```

Supported languages include: English (en), French (fr), German (de), Spanish (es), Italian (it), Portuguese (pt), Dutch (nl), Russian (ru), Chinese (zh), Japanese (ja), Korean (ko), and many others.

## Diagram Analysis and Mermaid Generation

### Basic Diagram Analysis (`llm-smoldocling` profile)

Uses the built-in SmolDocling vision model for diagram detection and description:

```json
{
  "source": "/path/to/document.pdf",
  "profile": "llm-smoldocling"
}
```

### Advanced Mermaid Generation (`llm-external` profile)

For diagram-to-Mermaid conversion, first configure external LLM integration:

```bash
# Required environment variables
export DOCLING_LLM_OPENAI_API_BASE="http://localhost:11434/v1"   # Any OpenAI-compatible endpoint
export DOCLING_LLM_MODEL_NAME="llava:latest"                     # Vision-capable model
export DOCLING_LLM_OPENAI_API_KEY="your-api-key-here"            # API key

# Optional configuration
export DOCLING_LLM_MAX_TOKENS="16384"        # Maximum tokens for LLM response
export DOCLING_LLM_TEMPERATURE="0.1"         # Temperature for LLM inference
export DOCLING_LLM_TIMEOUT="240"             # Timeout for LLM requests in seconds
```

Then use the `llm-external` profile:

```json
{
  "source": "/path/to/document.pdf",
  "profile": "llm-external"
}
```

### Supported LLM Providers

The tool supports any OpenAI-compatible API endpoint, e.g:
- **Ollama** (local): `http://localhost:11434/v1`
- **LM Studio** (local): `http://localhost:1234/v1`
- **OpenAI**: `https://api.openai.com/v1`
- **OpenRouter**: `https://openrouter.ai/api/v1`

Ensure you select a model that supports vision input (e.g., `mistral-small-3.2-24b-2506-ud:q6_k_xl`, `gpt-4-vision-preview`, `claude-3-sonnet`).

### Diagram Analysis Features

- **Automatic Detection**: Identifies diagrams, flowcharts, architecture diagrams, and charts
- **Type Classification**: Classifies diagram types with confidence scoring
- **Mermaid Conversion**: Generates valid Mermaid syntax for diagrams
- **Element Extraction**: Extracts text elements and structural components
- **AWS Colour Coding**: Applies consistent colour schemes for architecture diagrams
- **Validation**: Validates generated Mermaid syntax for correctness
- **Fallback Handling**: Gracefully falls back to basic analysis if LLM is unavailable

## Response Format

### File Save Response (Default)
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
    "subject": "Document Subject",
    "page_count": 10,
    "word_count": 1500
  },
  "images": [
    {
      "id": "image_1",
      "type": "picture",
      "caption": "Figure 1",
      "file_path": "/path/to/extracted/image_1.png",
      "width": 800,
      "height": 600
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
  ],
  "processing_info": {
    "processing_mode": "advanced",
    "processing_method": "advanced+vision:smoldocling+llm:enhanced",
    "hardware_acceleration": "mps",
    "ocr_enabled": false,
    "processing_time": 8.2,
    "timestamp": "2025-07-09T22:12:15+10:00"
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

1. **DocumentProcessorTool**: Main MCP tool interface with simplified profile system
2. **Config**: Configuration management with environment variable support
3. **CacheManager**: Intelligent caching system with TTL support
4. **LLMClient**: External LLM integration for advanced diagram processing
5. **Python Wrapper**: Subprocess interface to Docling Python library

### File Structure

```
internal/tools/docprocessing/
├── README.md                    # This file
├── document_processor.go        # Main tool implementation
├── types.go                     # Type definitions including profiles
├── config.go                    # Configuration management
├── cache.go                     # Caching system
├── llm_client.go               # LLM integration for diagram processing
└── python/
    ├── docling_processor.py     # Main Python wrapper script
    ├── image_processing.py      # Image extraction and processing
    └── table_processing.py      # Table extraction and formatting
```

### Processing Profiles Implementation

The profile system automatically configures multiple parameters:

- **Profile Selection**: Chooses appropriate processing mode, vision settings, and features
- **Dependency Resolution**: Automatically enables required services and parameters
- **Environment Awareness**: `llm-external` profile only available when LLM is configured
- **Backward Compatibility**: Individual parameters still work for advanced users

## Performance

### Caching

The tool implements intelligent caching based on:
- Document source (file path/URL)
- Processing parameters (profile, mode, OCR settings, etc.)
- File modification time (for local files)

Cache entries have a 24-hour TTL by default and are stored as JSON files.

### Hardware Acceleration

Processing performance varies by hardware:
- **CPU**: Baseline performance, works everywhere
- **MPS (macOS)**: 2-5x faster on Apple Silicon
- **CUDA**: 3-10x faster on NVIDIA GPUs

### Profile Performance Comparison

- **`basic`**: Fastest, 1-5 seconds for typical documents
- **`text-and-image`**: Moderate, 5-15 seconds with image extraction
- **`scanned`**: Slower, 10-30 seconds with OCR processing
- **`llm-smoldocling`**: Moderate, 10-20 seconds with vision analysis
- **`llm-external`**: Slowest, 15-60 seconds with full LLM processing

## Use With Custom MITM Certs

The document processing tool performs a pip install docling (if it's not found) and downloads models, so corporate environments with MITM proxies may need additional certs. Set the `DOCLING_EXTRA_CA_CERTS` environment variable:

```bash
export DOCLING_EXTRA_CA_CERTS="/path/to/mitm-ca-bundle.pem"
```

### Supported Certificate Formats

- `.pem` - PEM encoded certificates
- `.crt` - Certificate files
- `.cer` - Certificate files
- `.ca-bundle` - Certificate bundles

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

5. **"LLM external profile not available"**
   - Ensure all `DOCLING_LLM_*` environment variables are set
   - Verify LLM endpoint is accessible
   - Check model supports vision input

6. **"Certificate path does not exist"**
   - Verify the path specified in `DOCLING_EXTRA_CA_CERTS` exists
   - Ensure the certificate file or directory is readable

### Debug Mode

Enable debug mode to see detailed processing information:

```json
{
  "source": "/path/to/document.pdf",
  "debug": true
}
```


## Potential Future Enhancements

### Document Structure Enhancement
- **Reading Order Detection**: Improve paragraph and section ordering algorithms
- **Metadata Extraction**: Enhanced title, author, reference detection using NLP
- **Language Detection**: Automatic document language identification with confidence scores
- **Figure-Caption Matching**: Automatic association of figures with their captions using proximity and semantic analysis

### Processing Pipeline Options
- **Batch Processing**: Support for processing multiple documents efficiently with shared model loading
- **Resource Limits**: Configurable page limits, file size limits, CPU thread limits for enterprise deployment
- **Remote Services**: Optional integration with cloud-based OCR or vision services (Azure, AWS, GCP)
- **Custom Model Pipelines**: Extensible architecture for adding new models via plugin system

### Advanced Output Formats
- **Custom Chunking**: Integration with HybridChunker for RAG applications
- **Semantic Markup**: Add semantic tags for better downstream processing

### Diagram/Chart Processing (External Integration)
- **External Service Integration**: Use services like "Diagram to Mermaid Converter" APIs
- **Vision Model Integration**: Potentially add support for using an external LLM API for diagram processing
- **OCR + Pattern Recognition**: Extract text from diagrams and attempt to reconstruct logical structure
- **Flowchart Recognition**: Specific support for flowchart-to-Mermaid conversion

### Performance and Scalability
- **Streaming Processing**: Support for processing large documents in chunks
- **Distributed Processing**: Support for processing across multiple nodes

### Quality and Accuracy Improvements
- **Confidence Scoring**: Add confidence scores for all extracted elements
- **Quality Metrics**: Implement quality assessment for extracted content
- **Error Recovery**: Better handling of corrupted or unusual document formats

#### Smart Defaults and Auto-Detection
- **Language Detection**: Auto-detect instead of requiring `ocr_languages`
- **Processing Mode**: Auto-select based on document analysis
- **Table Processing**: Always use optimal settings


## License

This tool is part of the mcp-devtools project and follows the same license terms.
