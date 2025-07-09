# Docling Enhancement Development Plan

## Overview

This document outlines the development plan for enhancing the `process_document` tool in the MCP DevTools project with advanced Docling capabilities. The enhancements focus on improved vision processing, advanced table handling, and expanded output formats based on research into Docling's latest capabilities.

## Background & Research Findings

### Key Docling Capabilities Discovered

From the [Docling Technical Report](https://arxiv.org/abs/2408.09869) and [usage documentation](https://docling-project.github.io/docling/usage/), we learned:

1. **Architecture**: Docling uses a linear pipeline with PDF backends → AI models → assembly/post-processing
2. **AI Models**:
   - Layout Analysis: RT-DETR-based model trained on DocLayNet dataset
   - Table Structure Recognition: TableFormer model with advanced capabilities
   - OCR: EasyOCR integration with multi-language support
3. **Hardware Acceleration**: Supports CPU, CUDA, and MPS (Apple Silicon)
4. **Extensibility**: Custom model pipelines can be implemented via BaseModelPipeline
5. **SmolDocling**: A compact vision-language model (256M parameters) available via CLI and API

### Current Implementation Status

Our current Python wrapper (`docling_processor.py`) implements:
- Basic document conversion with configurable processing modes
- OCR support with language selection
- Hardware acceleration detection (MPS, CUDA, CPU) (The user wants the tool to work on macOS (Apple Silicon) and Linux - we do not want to support Windows)
- Basic metadata extraction
- Placeholder functions for image and table extraction

### Limitations Identified

1. **Diagram/Chart Processing**: Docling doesn't have native diagram-to-text conversion (e.g., to Mermaid)
2. **Vision Processing**: Limited to basic image detection, no advanced vision model integration
3. **Table Processing**: Basic extraction without advanced TableFormer options
4. **Output Formats**: Currently only supports Markdown export

## Project Goals

Enhance the `process_document` tool with:

1. **Enhanced Vision Processing** - Optional SmolDocling integration, diagram description, chart recognition
2. **Advanced Table Processing** - Optional TableFormer mode selection, cell matching control, multiple export formats
3. **Output Format Enhancements** - Optional structured JSON and Doctags format support

- We MUST ensure we leverage **GPU Acceleration** where ever possible (both for Apple Silicon and Linux) to ensure the best performance possible.
- All tool parameters must have clear, concise descriptions / annotations as this is how the AI agents using the tool will understand how to use it.

## Implementation Status

### ✅ COMPLETED - All Core Infrastructure and Features

#### Phase 4: Go Integration and Core Infrastructure - ✅ COMPLETED
- [x] Update `DocumentProcessingRequest` struct with new parameters
- [x] Add new processing modes to enum types (`TableFormerMode`, `VisionProcessingMode`)
- [x] Update tool definition with all new parameters including `export_file`
- [x] Implement parameter validation
- [x] Add new command line arguments for all parameters
- [x] Implement parameter parsing and validation
- [x] Add pipeline configuration for new features
- [x] Maintain backward compatibility

#### Enhanced Cache System - ✅ COMPLETED
- [x] Cache keys now include ALL processing parameters for accurate cache hits/misses
- [x] Different processing parameters create separate cache entries
- [x] Same parameters return cached results
- [x] Verified working with comprehensive testing

#### Processing Method Reporting - ✅ COMPLETED
- [x] Added `processing_method` field to response with concise descriptions
- [x] Examples: `"basic"`, `"basic+vision:smoldocling"`, `"basic+vision:advanced+charts"`
- [x] Clear indication of what processing features were used

#### Hardware Acceleration Detection - ✅ COMPLETED
- [x] Fixed MPS acceleration detection on Apple Silicon
- [x] Returns `"mps"` on Apple Silicon, `"cuda"` on NVIDIA GPUs, `"cpu"` for fallback
- [x] Automatic detection with no configuration required
- [x] Reported in `processing_info.hardware_acceleration` field

#### Export File Functionality - ✅ COMPLETED
- [x] Added `export_file` parameter for saving content to files
- [x] Automatically creates directories if they don't exist
- [x] Returns success message with export path instead of content
- [x] Verified working: tested with `/tmp/test_export.md`

#### Intelligent Feature Dependency Resolution - ✅ COMPLETED
- [x] Chart data extraction auto-enables: `vision_mode: "advanced"` + `enable_remote_services: true`
- [x] Diagram description auto-enables: `vision_mode: "advanced"` + `enable_remote_services: true`
- [x] SmolDocling vision auto-enables: `processing_mode: "advanced"` (when needed)
- [x] Table processing optimisation: `table_former_mode: "fast"` → `processing_mode: "tables"`
- [x] Comprehensive dependency resolution with user-friendly experience

### ✅ COMPLETED - All Core Parameters
- [x] `table_former_mode`: "fast" or "accurate" TableFormer processing
- [x] `cell_matching`: Control PDF vs predicted cell matching
- [x] `vision_mode`: "standard", "smoldocling", or "advanced"
- [x] `diagram_description`: Enable diagram/chart description
- [x] `chart_data_extraction`: Enable chart data extraction
- [x] `enable_remote_services`: Allow external vision services
- [x] `export_file`: Optional fully qualified path to save converted content

### 🔄 REMAINING IMPLEMENTATION TASKS

### Phase 1: Enhanced Vision Processing

#### Task 1.1: SmolDocling Integration - 🔄 FOUNDATION COMPLETE
- [x] Research SmolDocling CLI and API integration methods
- [x] Add SmolDocling as processing mode option in Go types
- [x] Implement SmolDocling pipeline foundation in Python wrapper
- [ ] Add SmolDocling-specific configuration options
- [ ] Test SmolDocling performance vs standard pipeline

**Technical Notes:**
- SmolDocling can be invoked via: `docling --pipeline vlm --vlm-model smoldocling`
- Uses MLX acceleration on Apple Silicon, CUDA on Linux
- 256M parameter compact vision-language model

#### Task 1.2: Diagram Description Implementation
- [ ] Add diagram detection and description capability
- [ ] Implement vision model integration for chart/diagram analysis
- [ ] Add structured output for diagram descriptions
- [ ] Create diagram-specific metadata extraction

**Technical Approach:**
- Use Docling's `PictureDescriptionApiOptions` for vision model API calls
- Requires `enable_remote_services=True` configuration
- Extract detected figures and process with vision models

#### Task 1.3: Chart/Graph Recognition and Data Extraction
- [ ] Implement chart type detection (bar, line, pie, etc.)
- [ ] Add data extraction from simple charts
- [ ] Create structured output format for chart data
- [ ] Add chart-to-table conversion capability

### Phase 2: Advanced Table Processing

#### Task 2.1: TableFormer Mode Selection
- [ ] Add `TableFormerMode` enum to Go types (`FAST`, `ACCURATE`)
- [ ] Implement mode selection in Python wrapper
- [ ] Update tool definition with new parameter
- [ ] Add mode selection to processing options

**Technical Implementation:**
```python
from docling.datamodel.pipeline_options import TableFormerMode
pipeline_options.table_structure_options.mode = TableFormerMode.ACCURATE
```

#### Task 2.2: Cell Matching Control
- [ ] Add `do_cell_matching` boolean parameter to Go types
- [ ] Implement cell matching control in Python wrapper
- [ ] Update tool definition and documentation
- [ ] Test quality differences between matching modes

**Technical Implementation:**
```python
pipeline_options.table_structure_options.do_cell_matching = False  # uses predicted text cells
```

#### Task 2.3: Table Export Formats
- [ ] Implement CSV export for extracted tables
- [ ] Add HTML table export capability
- [ ] Create structured JSON format for tables
- [ ] Add table metadata (headers, spans, etc.)

### Phase 3: Output Format Enhancements

#### Task 3.1: Structured JSON Export
- [ ] Implement full document structure JSON export
- [ ] Preserve all document elements and metadata
- [ ] Add hierarchical structure representation
- [ ] Ensure lossless serialisation capability

**Technical Notes:**
- Use Docling's native JSON serialisation
- Preserve bounding boxes, page numbers, element types
- Include processing metadata and confidence scores
- Never log to stdout or stderr when the MCP server is running in stdio mode as this will break MCP
- Always use British English spelling for all code, comments and documentation

#### Task 3.2: Doctags Format Support
- [ ] Research Doctags format specification
- [ ] Implement Doctags export in Python wrapper
- [ ] Add Doctags as output format option
- [ ] Test Doctags compatibility with downstream tools

### Phase 4: Integration and Testing

#### Task 4.1: Go Integration Updates
- [ ] Update `DocumentProcessingRequest` struct with new parameters
- [ ] Add new processing modes to enum types
- [ ] Update tool definition with all new parameters
- [ ] Implement parameter validation

#### Task 4.2: Python Wrapper Enhancements
- [ ] Refactor processing pipeline for new capabilities
- [ ] Implement proper error handling for new features
- [ ] Add comprehensive logging for debugging
- [ ] Optimise memory usage for large documents

#### Task 4.3: Testing and Validation
- [ ] Create test documents with various diagram types - you will need to stop and ask the user to provide a PDF with various diagrams and charts in it for you to test with.
- [ ] Test all table processing modes and formats
- [ ] Validate output format compatibility
- [ ] Performance testing with new features
- [ ] Create integration test

## Technical Architecture Changes

### New Go Types Required

```go
// TableFormer processing modes
type TableFormerMode string
const (
    TableFormerModeFast     TableFormerMode = "fast"
    TableFormerModeAccurate TableFormerMode = "accurate"
)

// Vision processing modes
type VisionProcessingMode string
const (
    VisionModeStandard   VisionProcessingMode = "standard"
    VisionModeSmolDocling VisionProcessingMode = "smoldocling"
    VisionModeAdvanced   VisionProcessingMode = "advanced"
)

// Additional request parameters
type DocumentProcessingRequest struct {
    // ... existing fields ...
    TableFormerMode      TableFormerMode      `json:"table_former_mode,omitempty"`
    CellMatching         *bool                `json:"cell_matching,omitempty"`
    VisionMode           VisionProcessingMode `json:"vision_mode,omitempty"`
    DiagramDescription   bool                 `json:"diagram_description,omitempty"`
    ChartDataExtraction  bool                 `json:"chart_data_extraction,omitempty"`
    EnableRemoteServices bool                 `json:"enable_remote_services,omitempty"`
}
```

### Python Wrapper Architecture

```python
# New processing pipeline structure
class EnhancedDoclingProcessor:
    def __init__(self, config):
        self.vision_processor = VisionProcessor()
        self.table_processor = AdvancedTableProcessor()
        self.output_formatter = MultiFormatOutputter()

    def process_with_vision(self, document, vision_mode):
        # SmolDocling or advanced vision processing
        pass

    def process_tables_advanced(self, document, mode, cell_matching):
        # Advanced table processing with TableFormer options
        pass

    def export_multiple_formats(self, document, formats):
        # Support for JSON, Doctags, etc.
        pass
```

## Dependencies and Requirements

### Python Dependencies (New)

Note: Always check and use the latest package versions available by using the tools you have available.

```
torch>=1.9.0  # For SmolDocling and advanced vision
transformers>=4.20.0  # For vision models
pillow>=8.0.0  # For image processing
pandas>=1.3.0  # For table data manipulation
```

### System Requirements
- **Memory**: Minimum 8GB RAM for vision processing
- **Storage**: Additional 2-4GB for vision model weights
- **Hardware**: GPU / Apple Silicon highly recommended for optimal performance

## Success Criteria

1. **Vision Processing**: Successfully extract and describe diagrams/charts from test documents
2. **Table Processing**: Demonstrate improved table extraction quality with new modes
3. **Output Formats**: Generate valid JSON and Doctags output for all document types
4. **Performance**: Maintain reasonable processing times (< 2x current baseline)
5. **Compatibility**: Maintain backward compatibility with existing API

## Risk Assessment

### High Risk
- SmolDocling integration complexity and performance impact
- Vision model API costs and reliability
- Memory usage with multiple large models

### Medium Risk
- TableFormer mode compatibility across Docling versions
- Output format standardisation and validation
- Performance degradation with new features

### Low Risk
- Go type additions and parameter handling
- Basic table export format implementation
- Documentation and testing updates

---

## Potential Future Enhancements

DO NOT IMPLEMENT THESE NOW, BUT KEEP THEM IN MIND FOR FUTURE ENHANCEMENTS

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
