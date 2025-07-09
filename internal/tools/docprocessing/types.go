package docprocessing

import (
	"time"
)

// ProcessingMode defines the type of document processing to perform
type ProcessingMode string

const (
	ProcessingModeBasic    ProcessingMode = "basic"    // Fast, code-only processing
	ProcessingModeAdvanced ProcessingMode = "advanced" // Vision model with layout preservation
	ProcessingModeOCR      ProcessingMode = "ocr"      // OCR for scanned documents
	ProcessingModeTables   ProcessingMode = "tables"   // Table extraction focus
	ProcessingModeImages   ProcessingMode = "images"   // Image extraction focus
)

// TableFormerMode defines the TableFormer processing mode for table structure recognition
type TableFormerMode string

const (
	TableFormerModeFast     TableFormerMode = "fast"     // Faster but less accurate table processing
	TableFormerModeAccurate TableFormerMode = "accurate" // More accurate but slower table processing (default)
)

// VisionProcessingMode defines the vision model processing mode for enhanced document understanding
type VisionProcessingMode string

const (
	VisionModeStandard    VisionProcessingMode = "standard"    // Standard vision processing
	VisionModeSmolDocling VisionProcessingMode = "smoldocling" // Compact vision-language model (256M parameters)
	VisionModeAdvanced    VisionProcessingMode = "advanced"    // Advanced vision processing with remote services
)

// OutputFormat defines the output format for processed documents
type OutputFormat string

const (
	OutputFormatMarkdown OutputFormat = "markdown" // Markdown output (default)
	OutputFormatJSON     OutputFormat = "json"     // JSON metadata
	OutputFormatBoth     OutputFormat = "both"     // Both markdown and JSON
)

// HardwareAcceleration defines the hardware acceleration mode
type HardwareAcceleration string

const (
	HardwareAccelerationAuto HardwareAcceleration = "auto" // Auto-detect best option
	HardwareAccelerationMPS  HardwareAcceleration = "mps"  // Metal Performance Shaders (macOS)
	HardwareAccelerationCUDA HardwareAcceleration = "cuda" // CUDA (NVIDIA GPUs)
	HardwareAccelerationCPU  HardwareAcceleration = "cpu"  // CPU-only processing
)

// DocumentProcessingRequest represents the input parameters for document processing
type DocumentProcessingRequest struct {
	Source                   string               `json:"source"`                                // File path, URL, or base64 content
	ProcessingMode           ProcessingMode       `json:"processing_mode,omitempty"`             // Processing mode (default: basic)
	OutputFormat             OutputFormat         `json:"output_format,omitempty"`               // Output format (default: markdown)
	EnableOCR                bool                 `json:"enable_ocr,omitempty"`                  // Enable OCR processing
	OCRLanguages             []string             `json:"ocr_languages,omitempty"`               // OCR language codes
	PreserveImages           bool                 `json:"preserve_images,omitempty"`             // Extract and preserve images
	CacheEnabled             *bool                `json:"cache_enabled,omitempty"`               // Override global cache setting
	Timeout                  *int                 `json:"timeout,omitempty"`                     // Processing timeout in seconds
	MaxFileSize              *int                 `json:"max_file_size,omitempty"`               // Maximum file size in MB
	ExportFile               string               `json:"export_file,omitempty"`                 // Optional fully qualified path to save the converted content
	ClearFileCache           bool                 `json:"clear_file_cache,omitempty"`            // Force clear all cache entries for this source file before processing
	TableFormerMode          TableFormerMode      `json:"table_former_mode,omitempty"`           // TableFormer processing mode for table structure recognition
	CellMatching             *bool                `json:"cell_matching,omitempty"`               // Control table cell matching (true: use PDF cells, false: use predicted cells)
	VisionMode               VisionProcessingMode `json:"vision_mode,omitempty"`                 // Vision processing mode for enhanced document understanding
	DiagramDescription       bool                 `json:"diagram_description,omitempty"`         // Enable diagram and chart description using vision models
	ChartDataExtraction      bool                 `json:"chart_data_extraction,omitempty"`       // Enable data extraction from charts and graphs
	EnableRemoteServices     bool                 `json:"enable_remote_services,omitempty"`      // Allow communication with external vision model services
	ConvertDiagramsToMermaid bool                 `json:"convert_diagrams_to_mermaid,omitempty"` // Convert detected diagrams to Mermaid syntax using AI vision models
	GenerateDiagrams         bool                 `json:"generate_diagrams,omitempty"`           // Generate enhanced diagram analysis using external LLM (requires DOCLING_LLM_OPENAI_API_BASE, DOCLING_LLM_MODEL_NAME, DOCLING_LLM_OPENAI_API_KEY environment variables)
	ExtractImages            bool                 `json:"extract_images,omitempty"`              // Extract individual images, charts, and diagrams as base64-encoded data with AI recreation prompts
}

// DocumentProcessingResponse represents the output from document processing
type DocumentProcessingResponse struct {
	Source         string             `json:"source"`             // Original source
	Content        string             `json:"content"`            // Processed content (markdown)
	Metadata       *DocumentMetadata  `json:"metadata,omitempty"` // Document metadata
	Images         []ExtractedImage   `json:"images,omitempty"`   // Extracted images
	Tables         []ExtractedTable   `json:"tables,omitempty"`   // Extracted tables
	Diagrams       []ExtractedDiagram `json:"diagrams,omitempty"` // Extracted diagrams
	ProcessingInfo ProcessingInfo     `json:"processing_info"`    // Processing information
	CacheHit       bool               `json:"cache_hit"`          // Whether result came from cache
	Error          string             `json:"error,omitempty"`    // Error message if processing failed
}

// DocumentMetadata contains metadata about the processed document
type DocumentMetadata struct {
	Title        string            `json:"title,omitempty"`         // Document title
	Author       string            `json:"author,omitempty"`        // Document author
	Subject      string            `json:"subject,omitempty"`       // Document subject
	Creator      string            `json:"creator,omitempty"`       // Document creator
	Producer     string            `json:"producer,omitempty"`      // Document producer
	CreationDate *time.Time        `json:"creation_date,omitempty"` // Creation date
	ModifiedDate *time.Time        `json:"modified_date,omitempty"` // Last modified date
	PageCount    int               `json:"page_count,omitempty"`    // Number of pages
	WordCount    int               `json:"word_count,omitempty"`    // Estimated word count
	Language     string            `json:"language,omitempty"`      // Detected language
	Format       string            `json:"format"`                  // Original document format
	FileSize     int64             `json:"file_size,omitempty"`     // File size in bytes
	Properties   map[string]string `json:"properties,omitempty"`    // Additional properties
}

// ExtractedImage represents an image extracted from the document
type ExtractedImage struct {
	ID            string       `json:"id"`                       // Unique image identifier
	Type          string       `json:"type"`                     // Type of image (picture, table, chart, diagram)
	Caption       string       `json:"caption,omitempty"`        // Image caption if available
	AltText       string       `json:"alt_text,omitempty"`       // Alternative text
	Format        string       `json:"format"`                   // Image format (PNG, JPEG, etc.)
	Width         int          `json:"width,omitempty"`          // Image width in pixels
	Height        int          `json:"height,omitempty"`         // Image height in pixels
	Size          int64        `json:"size,omitempty"`           // Image size in bytes
	FilePath      string       `json:"file_path,omitempty"`      // Path to saved image file
	PageNumber    int          `json:"page_number,omitempty"`    // Page number where image appears
	BoundingBox   *BoundingBox `json:"bounding_box,omitempty"`   // Position on page
	ExtractedText []string     `json:"extracted_text,omitempty"` // Text elements extracted from the image
}

// ExtractedTable represents a table extracted from the document
type ExtractedTable struct {
	ID          string       `json:"id"`                     // Unique table identifier
	Caption     string       `json:"caption,omitempty"`      // Table caption if available
	Headers     []string     `json:"headers,omitempty"`      // Column headers
	Rows        [][]string   `json:"rows"`                   // Table data rows
	PageNumber  int          `json:"page_number,omitempty"`  // Page number where table appears
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"` // Position on page
	Markdown    string       `json:"markdown,omitempty"`     // Markdown representation
	CSV         string       `json:"csv,omitempty"`          // CSV representation
}

// ExtractedDiagram represents a diagram extracted from the document
type ExtractedDiagram struct {
	ID          string                 `json:"id"`                     // Unique diagram identifier
	Type        string                 `json:"type"`                   // Type of diagram (flowchart, chart, diagram, etc.)
	Caption     string                 `json:"caption,omitempty"`      // Diagram caption if available
	Description string                 `json:"description,omitempty"`  // Generated description of the diagram
	DiagramType string                 `json:"diagram_type,omitempty"` // Classified diagram type (flowchart, chart, etc.)
	MermaidCode string                 `json:"mermaid_code,omitempty"` // Generated Mermaid syntax for the diagram
	Base64Data  string                 `json:"base64_data,omitempty"`  // Base64-encoded image data for LLM vision processing
	Elements    []DiagramElement       `json:"elements,omitempty"`     // Text elements within the diagram
	PageNumber  int                    `json:"page_number,omitempty"`  // Page number where diagram appears
	BoundingBox *BoundingBox           `json:"bounding_box,omitempty"` // Position on page
	Confidence  float64                `json:"confidence,omitempty"`   // Confidence score for diagram analysis
	Properties  map[string]interface{} `json:"properties,omitempty"`   // Additional diagram-specific properties
}

// DiagramElement represents a text or structural element within a diagram
type DiagramElement struct {
	Type        string       `json:"type"`                   // Element type (text, shape, connector, etc.)
	Content     string       `json:"content,omitempty"`      // Text content of the element
	Position    string       `json:"position,omitempty"`     // Position description within diagram
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"` // Position within the diagram
}

// BoundingBox represents the position and size of an element on a page
type BoundingBox struct {
	X      float64 `json:"x"`      // X coordinate (left)
	Y      float64 `json:"y"`      // Y coordinate (top)
	Width  float64 `json:"width"`  // Width
	Height float64 `json:"height"` // Height
}

// ProcessingInfo contains information about the processing operation
type ProcessingInfo struct {
	ProcessingMode       ProcessingMode       `json:"processing_mode"`           // Mode used for processing
	ProcessingMethod     string               `json:"processing_method"`         // Concise description of processing method used
	HardwareAcceleration HardwareAcceleration `json:"hardware_acceleration"`     // Hardware acceleration used
	VisionModel          string               `json:"vision_model,omitempty"`    // Vision model used (if any)
	OCREnabled           bool                 `json:"ocr_enabled"`               // Whether OCR was enabled
	OCRLanguages         []string             `json:"ocr_languages,omitempty"`   // OCR languages used
	ProcessingTime       float64              `json:"processing_time"`           // Time taken to process in seconds
	PythonVersion        string               `json:"python_version,omitempty"`  // Python version used
	DoclingVersion       string               `json:"docling_version,omitempty"` // Docling version used
	CacheKey             string               `json:"cache_key,omitempty"`       // Cache key used
	Timestamp            time.Time            `json:"timestamp"`                 // Processing timestamp
}

// BatchProcessingRequest represents a request to process multiple documents
type BatchProcessingRequest struct {
	Sources        []string       `json:"sources"`                   // Multiple document sources
	ProcessingMode ProcessingMode `json:"processing_mode,omitempty"` // Processing mode for all documents
	OutputFormat   OutputFormat   `json:"output_format,omitempty"`   // Output format for all documents
	EnableOCR      bool           `json:"enable_ocr,omitempty"`      // Enable OCR for all documents
	OCRLanguages   []string       `json:"ocr_languages,omitempty"`   // OCR languages for all documents
	PreserveImages bool           `json:"preserve_images,omitempty"` // Extract images from all documents
	CacheEnabled   *bool          `json:"cache_enabled,omitempty"`   // Cache setting for all documents
	Timeout        *int           `json:"timeout,omitempty"`         // Timeout for each document
	MaxConcurrency int            `json:"max_concurrency,omitempty"` // Maximum concurrent processing
}

// BatchProcessingResponse represents the response from batch processing
type BatchProcessingResponse struct {
	Results   []DocumentProcessingResponse `json:"results"`    // Individual processing results
	Summary   BatchSummary                 `json:"summary"`    // Batch processing summary
	TotalTime time.Duration                `json:"total_time"` // Total processing time
	Timestamp time.Time                    `json:"timestamp"`  // Batch processing timestamp
}

// BatchSummary provides summary statistics for batch processing
type BatchSummary struct {
	TotalDocuments  int `json:"total_documents"`  // Total number of documents
	SuccessfulCount int `json:"successful_count"` // Number of successfully processed documents
	FailedCount     int `json:"failed_count"`     // Number of failed documents
	CacheHitCount   int `json:"cache_hit_count"`  // Number of cache hits
	TotalPages      int `json:"total_pages"`      // Total pages processed
	TotalWords      int `json:"total_words"`      // Total words processed
	TotalImages     int `json:"total_images"`     // Total images extracted
	TotalTables     int `json:"total_tables"`     // Total tables extracted
}

// SystemInfo represents system information for diagnostics
type SystemInfo struct {
	Platform             string                 `json:"platform"`                        // Operating system
	Architecture         string                 `json:"architecture"`                    // CPU architecture
	PythonPath           string                 `json:"python_path,omitempty"`           // Path to Python executable
	PythonVersion        string                 `json:"python_version,omitempty"`        // Python version
	DoclingVersion       string                 `json:"docling_version,omitempty"`       // Docling version
	DoclingAvailable     bool                   `json:"docling_available"`               // Whether Docling is available
	HardwareAcceleration []HardwareAcceleration `json:"hardware_acceleration_available"` // Available acceleration options
	CacheDirectory       string                 `json:"cache_directory,omitempty"`       // Cache directory path
	CacheEnabled         bool                   `json:"cache_enabled"`                   // Whether caching is enabled
	MaxFileSize          int                    `json:"max_file_size"`                   // Maximum file size in MB
	DefaultTimeout       int                    `json:"default_timeout"`                 // Default timeout in seconds
}

// ErrorInfo represents detailed error information
type ErrorInfo struct {
	Code        string            `json:"code"`              // Error code
	Message     string            `json:"message"`           // Error message
	Details     string            `json:"details,omitempty"` // Additional error details
	Source      string            `json:"source,omitempty"`  // Source that caused the error
	Timestamp   time.Time         `json:"timestamp"`         // When the error occurred
	Context     map[string]string `json:"context,omitempty"` // Additional context
	Recoverable bool              `json:"recoverable"`       // Whether the error is recoverable
}
