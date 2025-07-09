#!/usr/bin/env python3
"""
Docling Document Processing Wrapper Script

This script provides a command-line interface to Docling for document processing.
It's designed to be called from the Go MCP server as a subprocess.
"""

import argparse
import json
import sys
import hashlib
import gc
from typing import Optional, List, Dict, Any
import logging
import time

# Configure logging to stderr to avoid interfering with JSON output
logging.basicConfig(
    level=logging.WARNING,
    format='%(asctime)s - %(levelname)s - %(message)s',
    stream=sys.stderr
)
logger = logging.getLogger(__name__)

def configure_accelerator():
    """Configure the accelerator device for Docling."""
    try:
        # Try to use MPS (Metal Performance Shaders) on macOS first
        import platform
        if platform.system() == 'Darwin':
            try:
                import torch
                if torch.backends.mps.is_available():
                    # Try to configure Docling settings if available
                    try:
                        from docling.datamodel.settings import settings
                        from docling.utils.accelerator_utils import AcceleratorDevice
                        if hasattr(settings.perf, 'accelerator_device'):
                            settings.perf.accelerator_device = AcceleratorDevice.MPS
                    except ImportError:
                        pass  # Settings not available, but MPS is still detected
                    return "mps"
            except ImportError:
                pass

        # Try CUDA if available
        try:
            import torch
            if torch.cuda.is_available():
                # Try to configure Docling settings if available
                try:
                    from docling.datamodel.settings import settings
                    from docling.utils.accelerator_utils import AcceleratorDevice
                    if hasattr(settings.perf, 'accelerator_device'):
                        settings.perf.accelerator_device = AcceleratorDevice.CUDA
                except ImportError:
                    pass  # Settings not available, but CUDA is still detected
                return "cuda"
        except ImportError:
            pass

        # Fall back to CPU
        try:
            from docling.datamodel.settings import settings
            from docling.utils.accelerator_utils import AcceleratorDevice
            if hasattr(settings.perf, 'accelerator_device'):
                settings.perf.accelerator_device = AcceleratorDevice.CPU
        except ImportError:
            pass  # Settings not available
        return "cpu"

    except Exception as e:
        logger.warning(f"Failed to configure accelerator: {e}")
        return "unknown"

def cleanup_memory():
    """Force garbage collection to free up memory."""
    gc.collect()

def create_smoldocling_converter(format_options):
    """Create a DocumentConverter configured for SmolDocling vision processing."""
    try:
        from docling.document_converter import DocumentConverter

        # For now, use standard converter with enhanced pipeline options
        # SmolDocling integration would require additional configuration
        # This is a placeholder for future SmolDocling-specific setup
        converter = DocumentConverter(format_options=format_options)

        return converter
    except Exception as e:
        logger.warning(f"Failed to create SmolDocling converter, falling back to standard: {e}")
        from docling.document_converter import DocumentConverter
        return DocumentConverter(format_options=format_options)

def get_cache_key(args) -> str:
    """Generate a cache key for the document conversion including all processing parameters."""
    key_data = {
        "source": args.source,
        "processing_mode": args.processing_mode,
        "enable_ocr": args.enable_ocr,
        "ocr_languages": args.ocr_languages or [],
        "preserve_images": args.preserve_images,
        "table_former_mode": getattr(args, 'table_former_mode', 'accurate'),
        "cell_matching": getattr(args, 'cell_matching', None),
        "no_cell_matching": getattr(args, 'no_cell_matching', False),
        "vision_mode": getattr(args, 'vision_mode', 'standard'),
        "diagram_description": getattr(args, 'diagram_description', False),
        "chart_data_extraction": getattr(args, 'chart_data_extraction', False),
        "enable_remote_services": getattr(args, 'enable_remote_services', False),
        "output_format": args.output_format
    }
    key_str = json.dumps(key_data, sort_keys=True)
    return hashlib.md5(key_str.encode()).hexdigest()

def resolve_feature_dependencies(args):
    """Intelligently resolve feature dependencies by auto-enabling required features."""
    # Create a copy of args to avoid modifying the original
    import copy
    resolved_args = copy.copy(args)

    # Track what we've auto-enabled for user feedback
    auto_enabled = []

    # Chart data extraction requires advanced vision processing
    if getattr(args, 'chart_data_extraction', False):
        if getattr(args, 'vision_mode', 'standard') == 'standard':
            resolved_args.vision_mode = 'advanced'
            auto_enabled.append("vision_mode: advanced (required for chart data extraction)")

        # Chart extraction also requires remote services
        if not getattr(args, 'enable_remote_services', False):
            resolved_args.enable_remote_services = True
            auto_enabled.append("enable_remote_services: true (required for chart data extraction)")

    # Diagram description requires advanced vision processing
    if getattr(args, 'diagram_description', False):
        if getattr(args, 'vision_mode', 'standard') == 'standard':
            resolved_args.vision_mode = 'advanced'
            auto_enabled.append("vision_mode: advanced (required for diagram description)")

        # Diagram description also requires remote services
        if not getattr(args, 'enable_remote_services', False):
            resolved_args.enable_remote_services = True
            auto_enabled.append("enable_remote_services: true (required for diagram description)")

    # SmolDocling vision mode requires advanced processing mode
    if getattr(args, 'vision_mode', 'standard') == 'smoldocling':
        if args.processing_mode == 'basic':
            resolved_args.processing_mode = 'advanced'
            auto_enabled.append("processing_mode: advanced (required for SmolDocling vision)")

    # Advanced vision mode requires advanced processing mode
    if getattr(args, 'vision_mode', 'standard') == 'advanced':
        if args.processing_mode == 'basic':
            resolved_args.processing_mode = 'advanced'
            auto_enabled.append("processing_mode: advanced (required for advanced vision)")

    # Table-focused processing with fast mode should enable table structure processing
    if getattr(args, 'table_former_mode', 'accurate') == 'fast':
        if args.processing_mode not in ['tables', 'advanced']:
            resolved_args.processing_mode = 'tables'
            auto_enabled.append("processing_mode: tables (optimised for fast table processing)")

    # Log auto-enabled features for debugging (to stderr, not stdout)
    if auto_enabled:
        logger.info(f"Auto-enabled features: {', '.join(auto_enabled)}")

    return resolved_args

def get_processing_method_description(args) -> str:
    """Generate a concise description of the processing method used."""
    components = []

    # Base processing mode
    if args.enable_ocr:
        components.append("ocr")

    # Vision processing
    vision_mode = getattr(args, 'vision_mode', 'standard')
    if vision_mode != 'standard':
        components.append(f"vision:{vision_mode}")
    elif args.processing_mode in ['advanced', 'images']:
        components.append("vision:standard")

    # Table processing
    if args.processing_mode == 'tables' or getattr(args, 'table_former_mode', 'accurate') == 'fast':
        table_mode = getattr(args, 'table_former_mode', 'accurate')
        components.append(f"tables:{table_mode}")

    # Special features
    if getattr(args, 'diagram_description', False):
        components.append("diagrams")
    if getattr(args, 'chart_data_extraction', False):
        components.append("charts")

    # If no special processing, just return the base mode
    if not components:
        return args.processing_mode

    return f"{args.processing_mode}+{'+'.join(components)}"

def process_document(args) -> Dict[str, Any]:
    """Process a document using Docling."""
    start_time = time.time()

    try:
        # Import Docling components
        from docling.document_converter import DocumentConverter, PdfFormatOption
        from docling.datamodel.base_models import InputFormat
        from docling.datamodel.pipeline_options import (
            PdfPipelineOptions,
            EasyOcrOptions,
            TableFormerMode
        )

        # Apply intelligent feature dependency resolution
        args = resolve_feature_dependencies(args)

        # Configure hardware acceleration
        hardware_acceleration = configure_accelerator()

        # Build pipeline options
        pipeline_options = PdfPipelineOptions()

        # Configure OCR if enabled
        if args.enable_ocr:
            ocr_options = EasyOcrOptions(lang=args.ocr_languages or ["en"])
            pipeline_options.do_ocr = True
            pipeline_options.ocr_options = ocr_options

        # Configure table processing
        if hasattr(args, 'table_former_mode') and args.table_former_mode:
            pipeline_options.do_table_structure = True
            if args.table_former_mode == 'fast':
                pipeline_options.table_structure_options.mode = TableFormerMode.FAST
            else:
                pipeline_options.table_structure_options.mode = TableFormerMode.ACCURATE

        # Configure cell matching
        if hasattr(args, 'cell_matching') and args.cell_matching is not None:
            if args.no_cell_matching:
                pipeline_options.table_structure_options.do_cell_matching = False
            elif args.cell_matching:
                pipeline_options.table_structure_options.do_cell_matching = True

        # Configure remote services if needed for advanced vision processing
        if hasattr(args, 'enable_remote_services') and args.enable_remote_services:
            pipeline_options.enable_remote_services = True

        # Configure vision processing mode
        vision_mode = getattr(args, 'vision_mode', 'standard')

        # Set up format options
        format_options = {
            InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline_options)
        }

        # Create converter with appropriate configuration
        if vision_mode == 'smoldocling':
            # Use SmolDocling pipeline if available
            converter = create_smoldocling_converter(format_options)
        else:
            converter = DocumentConverter(format_options=format_options)

        # Convert the document
        result = converter.convert(args.source)

        # Check for errors - handle different API versions
        has_error = False
        error_message = ""

        # Try different ways to check for errors based on the API version
        if hasattr(result, 'status'):
            if hasattr(result.status, 'is_error'):
                has_error = result.status.is_error
            elif hasattr(result.status, 'error'):
                has_error = result.status.error

        if hasattr(result, 'errors') and result.errors:
            has_error = True
            error_message = str(result.errors)

        if has_error:
            return {
                "success": False,
                "error": f"Conversion failed: {error_message}",
                "processing_time": time.time() - start_time
            }

        # Generate output based on format
        content_output = ""
        structured_json = None

        if args.output_format in ['markdown', 'both']:
            # Export to markdown
            content_output = result.document.export_to_markdown()

        if args.output_format in ['json', 'both']:
            # Export structured JSON
            structured_json = export_structured_json(result.document)

        # Extract metadata
        metadata = extract_metadata(result.document)

        # Extract images if requested
        images = []
        if args.preserve_images:
            images = extract_images(result.document)

        # Extract tables if requested
        tables = []
        if args.processing_mode in ['tables', 'advanced']:
            tables = extract_tables(result.document)

        # Clean up memory
        cleanup_memory()

        processing_time = time.time() - start_time

        response = {
            "success": True,
            "content": content_output,
            "metadata": metadata,
            "images": images,
            "tables": tables,
            "processing_info": {
                "processing_mode": args.processing_mode,
                "processing_method": get_processing_method_description(args),
                "hardware_acceleration": str(hardware_acceleration) if hardware_acceleration else "unknown",
                "ocr_enabled": args.enable_ocr,
                "ocr_languages": args.ocr_languages or [],
                "processing_time": processing_time,
                "timestamp": time.time()
            }
        }

        # Add structured JSON if requested
        if structured_json:
            response["structured_json"] = structured_json

        return response

    except ImportError as e:
        return {
            "success": False,
            "error": f"Docling not available: {str(e)}",
            "processing_time": time.time() - start_time
        }
    except Exception as e:
        logger.exception(f"Error processing document: {args.source}")
        return {
            "success": False,
            "error": f"Processing failed: {str(e)}",
            "processing_time": time.time() - start_time
        }

def extract_metadata(document) -> Dict[str, Any]:
    """Extract metadata from the document."""
    metadata = {}

    try:
        # Try to extract basic metadata
        if hasattr(document, 'meta'):
            meta = document.meta
            if hasattr(meta, 'title') and meta.title:
                metadata['title'] = meta.title
            if hasattr(meta, 'author') and meta.author:
                metadata['author'] = meta.author
            if hasattr(meta, 'subject') and meta.subject:
                metadata['subject'] = meta.subject

        # Count pages if available
        if hasattr(document, 'pages'):
            metadata['page_count'] = len(document.pages)

        # Estimate word count from content
        if hasattr(document, 'export_to_markdown'):
            content = document.export_to_markdown()
            words = len(content.split())
            metadata['word_count'] = words

    except Exception as e:
        logger.warning(f"Failed to extract metadata: {e}")

    return metadata

def extract_images(document) -> List[Dict[str, Any]]:
    """Extract images from the document."""
    images = []

    try:
        # This is a placeholder - actual implementation would depend on
        # Docling's API for image extraction
        pass  # Image extraction not yet implemented
    except Exception as e:
        logger.warning(f"Failed to extract images: {e}")

    return images

def extract_tables(document) -> List[Dict[str, Any]]:
    """Extract tables from the document with multiple export formats."""
    tables = []

    try:
        # Extract tables from Docling document
        if hasattr(document, 'tables') and document.tables:
            for i, table in enumerate(document.tables):
                table_data = {
                    "id": f"table_{i+1}",
                    "page_number": getattr(table, 'page_number', None),
                    "caption": getattr(table, 'caption', ''),
                    "headers": [],
                    "rows": [],
                    "markdown": "",
                    "csv": "",
                    "html": ""
                }

                # Extract table structure
                if hasattr(table, 'data') and table.data:
                    # Convert table data to structured format
                    table_rows = []
                    headers = []

                    # Handle different table data formats
                    if isinstance(table.data, list):
                        # List of rows format
                        for row_idx, row in enumerate(table.data):
                            if isinstance(row, list):
                                # List of cells
                                row_data = [str(cell) if cell is not None else "" for cell in row]
                            elif hasattr(row, 'cells'):
                                # Row object with cells
                                row_data = [str(cell.text) if hasattr(cell, 'text') else str(cell) for cell in row.cells]
                            else:
                                # Fallback: convert to string
                                row_data = [str(row)]

                            if row_idx == 0 and not headers:
                                # First row might be headers
                                headers = row_data
                            else:
                                table_rows.append(row_data)

                    # If no headers were detected, create generic ones
                    if not headers and table_rows:
                        headers = [f"Column {i+1}" for i in range(len(table_rows[0]))]

                    table_data["headers"] = headers
                    table_data["rows"] = table_rows

                    # Generate export formats
                    table_data["markdown"] = generate_table_markdown(headers, table_rows)
                    table_data["csv"] = generate_table_csv(headers, table_rows)
                    table_data["html"] = generate_table_html(headers, table_rows, table_data["caption"])

                # Add bounding box if available
                if hasattr(table, 'bbox') or hasattr(table, 'bounding_box'):
                    bbox = getattr(table, 'bbox', None) or getattr(table, 'bounding_box', None)
                    if bbox:
                        table_data["bounding_box"] = {
                            "x": getattr(bbox, 'x', 0),
                            "y": getattr(bbox, 'y', 0),
                            "width": getattr(bbox, 'width', 0),
                            "height": getattr(bbox, 'height', 0)
                        }

                tables.append(table_data)

        # Alternative: Extract from document elements if tables attribute not available
        elif hasattr(document, 'elements'):
            table_count = 0
            for element in document.elements:
                if hasattr(element, 'type') and element.type == 'table':
                    table_count += 1
                    table_data = extract_table_from_element(element, table_count)
                    if table_data:
                        tables.append(table_data)

    except Exception as e:
        logger.warning(f"Failed to extract tables: {e}")

    return tables

def extract_table_from_element(element, table_id: int) -> Dict[str, Any]:
    """Extract table data from a document element."""
    try:
        table_data = {
            "id": f"table_{table_id}",
            "page_number": getattr(element, 'page_number', None),
            "caption": getattr(element, 'caption', ''),
            "headers": [],
            "rows": [],
            "markdown": "",
            "csv": "",
            "html": ""
        }

        # Extract table content from element
        if hasattr(element, 'content') and element.content:
            # Try to parse table content
            content = element.content
            if isinstance(content, str):
                # Parse markdown-style table
                lines = content.strip().split('\n')
                if len(lines) >= 2:
                    # First line as headers
                    headers = [cell.strip() for cell in lines[0].split('|') if cell.strip()]

                    # Skip separator line (usually contains dashes)
                    data_lines = lines[2:] if len(lines) > 2 else []

                    rows = []
                    for line in data_lines:
                        if '|' in line:
                            row = [cell.strip() for cell in line.split('|') if cell.strip()]
                            if row:
                                rows.append(row)

                    table_data["headers"] = headers
                    table_data["rows"] = rows

                    # Generate export formats
                    table_data["markdown"] = generate_table_markdown(headers, rows)
                    table_data["csv"] = generate_table_csv(headers, rows)
                    table_data["html"] = generate_table_html(headers, rows, table_data["caption"])

        return table_data

    except Exception as e:
        logger.warning(f"Failed to extract table from element: {e}")
        return None

def generate_table_markdown(headers: List[str], rows: List[List[str]]) -> str:
    """Generate markdown table format."""
    if not headers and not rows:
        return ""

    try:
        lines = []

        # Add headers
        if headers:
            lines.append("| " + " | ".join(headers) + " |")
            lines.append("| " + " | ".join(["---"] * len(headers)) + " |")

        # Add rows
        for row in rows:
            # Ensure row has same number of columns as headers
            padded_row = row + [""] * (len(headers) - len(row)) if headers else row
            lines.append("| " + " | ".join(padded_row) + " |")

        return "\n".join(lines)

    except Exception as e:
        logger.warning(f"Failed to generate markdown table: {e}")
        return ""

def generate_table_csv(headers: List[str], rows: List[List[str]]) -> str:
    """Generate CSV table format."""
    if not headers and not rows:
        return ""

    try:
        import csv
        import io

        output = io.StringIO()
        writer = csv.writer(output, quoting=csv.QUOTE_MINIMAL)

        # Write headers
        if headers:
            writer.writerow(headers)

        # Write rows
        for row in rows:
            # Ensure row has same number of columns as headers
            padded_row = row + [""] * (len(headers) - len(row)) if headers else row
            writer.writerow(padded_row)

        return output.getvalue().strip()

    except Exception as e:
        logger.warning(f"Failed to generate CSV table: {e}")
        return ""

def generate_table_html(headers: List[str], rows: List[List[str]], caption: str = "") -> str:
    """Generate HTML table format."""
    if not headers and not rows:
        return ""

    try:
        html_parts = ["<table>"]

        # Add caption if provided
        if caption:
            html_parts.append(f"  <caption>{escape_html(caption)}</caption>")

        # Add headers
        if headers:
            html_parts.append("  <thead>")
            html_parts.append("    <tr>")
            for header in headers:
                html_parts.append(f"      <th>{escape_html(header)}</th>")
            html_parts.append("    </tr>")
            html_parts.append("  </thead>")

        # Add rows
        if rows:
            html_parts.append("  <tbody>")
            for row in rows:
                html_parts.append("    <tr>")
                # Ensure row has same number of columns as headers
                padded_row = row + [""] * (len(headers) - len(row)) if headers else row
                for cell in padded_row:
                    html_parts.append(f"      <td>{escape_html(cell)}</td>")
                html_parts.append("    </tr>")
            html_parts.append("  </tbody>")

        html_parts.append("</table>")

        return "\n".join(html_parts)

    except Exception as e:
        logger.warning(f"Failed to generate HTML table: {e}")
        return ""

def escape_html(text: str) -> str:
    """Escape HTML special characters."""
    if not isinstance(text, str):
        text = str(text)

    return (text
            .replace("&", "&amp;")
            .replace("<", "&lt;")
            .replace(">", "&gt;")
            .replace('"', "&quot;")
            .replace("'", "&#x27;"))

def export_structured_json(document) -> Dict[str, Any]:
    """Export document as structured JSON with full document hierarchy."""
    try:
        structured_doc = {
            "document_type": "structured_document",
            "version": "1.0",
            "metadata": {},
            "pages": [],
            "elements": [],
            "tables": [],
            "images": [],
            "structure": {
                "headings": [],
                "paragraphs": [],
                "lists": [],
                "sections": []
            }
        }

        # Extract basic metadata
        if hasattr(document, 'meta'):
            meta = document.meta
            structured_doc["metadata"] = {
                "title": getattr(meta, 'title', ''),
                "author": getattr(meta, 'author', ''),
                "subject": getattr(meta, 'subject', ''),
                "creator": getattr(meta, 'creator', ''),
                "producer": getattr(meta, 'producer', ''),
                "creation_date": getattr(meta, 'creation_date', None),
                "modified_date": getattr(meta, 'modified_date', None)
            }

        # Extract page information
        if hasattr(document, 'pages'):
            for i, page in enumerate(document.pages):
                page_data = {
                    "page_number": i + 1,
                    "width": getattr(page, 'width', 0),
                    "height": getattr(page, 'height', 0),
                    "elements": []
                }

                # Extract elements from page if available
                if hasattr(page, 'elements'):
                    for element in page.elements:
                        element_data = extract_element_data(element, i + 1)
                        if element_data:
                            page_data["elements"].append(element_data)

                structured_doc["pages"].append(page_data)

        # Extract document-level elements
        if hasattr(document, 'elements'):
            for element in document.elements:
                element_data = extract_element_data(element)
                if element_data:
                    structured_doc["elements"].append(element_data)

                    # Categorise elements by type
                    element_type = element_data.get("type", "unknown")
                    if element_type == "heading":
                        structured_doc["structure"]["headings"].append(element_data)
                    elif element_type == "paragraph":
                        structured_doc["structure"]["paragraphs"].append(element_data)
                    elif element_type == "list":
                        structured_doc["structure"]["lists"].append(element_data)
                    elif element_type == "table":
                        structured_doc["tables"].append(element_data)
                    elif element_type == "image":
                        structured_doc["images"].append(element_data)

        # Extract tables with structured data
        if hasattr(document, 'tables'):
            for i, table in enumerate(document.tables):
                table_data = {
                    "id": f"table_{i+1}",
                    "type": "table",
                    "page_number": getattr(table, 'page_number', None),
                    "caption": getattr(table, 'caption', ''),
                    "structure": extract_table_structure(table),
                    "bounding_box": extract_bounding_box(table)
                }
                structured_doc["tables"].append(table_data)

        # Add document statistics
        structured_doc["statistics"] = {
            "total_pages": len(structured_doc["pages"]),
            "total_elements": len(structured_doc["elements"]),
            "total_tables": len(structured_doc["tables"]),
            "total_images": len(structured_doc["images"]),
            "total_headings": len(structured_doc["structure"]["headings"]),
            "total_paragraphs": len(structured_doc["structure"]["paragraphs"]),
            "total_lists": len(structured_doc["structure"]["lists"])
        }

        return structured_doc

    except Exception as e:
        logger.warning(f"Failed to export structured JSON: {e}")
        return {
            "document_type": "structured_document",
            "version": "1.0",
            "error": f"Failed to extract structure: {str(e)}",
            "metadata": {},
            "pages": [],
            "elements": []
        }

def extract_element_data(element, page_number: int = None) -> Dict[str, Any]:
    """Extract structured data from a document element."""
    try:
        element_data = {
            "type": getattr(element, 'type', 'unknown'),
            "content": getattr(element, 'content', ''),
            "text": getattr(element, 'text', ''),
            "page_number": page_number or getattr(element, 'page_number', None),
            "bounding_box": extract_bounding_box(element),
            "properties": {}
        }

        # Extract type-specific properties
        element_type = element_data["type"]

        if element_type == "heading":
            element_data["properties"] = {
                "level": getattr(element, 'level', 1),
                "text": element_data["text"] or element_data["content"]
            }
        elif element_type == "paragraph":
            element_data["properties"] = {
                "text": element_data["text"] or element_data["content"],
                "word_count": len((element_data["text"] or element_data["content"]).split())
            }
        elif element_type == "list":
            element_data["properties"] = {
                "list_type": getattr(element, 'list_type', 'unordered'),
                "items": getattr(element, 'items', [])
            }
        elif element_type == "table":
            element_data["properties"] = {
                "rows": getattr(element, 'rows', 0),
                "columns": getattr(element, 'columns', 0),
                "caption": getattr(element, 'caption', '')
            }
        elif element_type == "image":
            element_data["properties"] = {
                "alt_text": getattr(element, 'alt_text', ''),
                "caption": getattr(element, 'caption', ''),
                "format": getattr(element, 'format', ''),
                "width": getattr(element, 'width', 0),
                "height": getattr(element, 'height', 0)
            }

        # Extract confidence scores if available
        if hasattr(element, 'confidence'):
            element_data["confidence"] = element.confidence

        return element_data

    except Exception as e:
        logger.warning(f"Failed to extract element data: {e}")
        return None

def extract_table_structure(table) -> Dict[str, Any]:
    """Extract structured table data."""
    try:
        structure = {
            "headers": [],
            "rows": [],
            "row_count": 0,
            "column_count": 0
        }

        if hasattr(table, 'data') and table.data:
            if isinstance(table.data, list):
                structure["row_count"] = len(table.data)

                for row_idx, row in enumerate(table.data):
                    if isinstance(row, list):
                        row_data = [str(cell) if cell is not None else "" for cell in row]
                    elif hasattr(row, 'cells'):
                        row_data = [str(cell.text) if hasattr(cell, 'text') else str(cell) for cell in row.cells]
                    else:
                        row_data = [str(row)]

                    if row_idx == 0:
                        structure["headers"] = row_data
                        structure["column_count"] = len(row_data)
                    else:
                        structure["rows"].append(row_data)

        return structure

    except Exception as e:
        logger.warning(f"Failed to extract table structure: {e}")
        return {"headers": [], "rows": [], "row_count": 0, "column_count": 0}

def extract_bounding_box(element) -> Dict[str, float]:
    """Extract bounding box information from an element."""
    try:
        bbox = getattr(element, 'bbox', None) or getattr(element, 'bounding_box', None)
        if bbox:
            return {
                "x": getattr(bbox, 'x', 0.0),
                "y": getattr(bbox, 'y', 0.0),
                "width": getattr(bbox, 'width', 0.0),
                "height": getattr(bbox, 'height', 0.0)
            }
        return {"x": 0.0, "y": 0.0, "width": 0.0, "height": 0.0}

    except Exception as e:
        logger.warning(f"Failed to extract bounding box: {e}")
        return {"x": 0.0, "y": 0.0, "width": 0.0, "height": 0.0}

def get_system_info() -> Dict[str, Any]:
    """Get system information for diagnostics."""
    import platform

    info = {
        "platform": platform.system(),
        "architecture": platform.machine(),
        "python_version": platform.python_version(),
    }

    # Check Docling availability and version
    try:
        import docling
        info["docling_available"] = True
        if hasattr(docling, '__version__'):
            info["docling_version"] = docling.__version__
    except ImportError:
        info["docling_available"] = False

    # Check hardware acceleration availability
    acceleration_available = []

    # Check MPS (macOS)
    if platform.system() == 'Darwin':
        try:
            import torch
            if torch.backends.mps.is_available():
                acceleration_available.append("mps")
        except ImportError:
            pass

    # Check CUDA
    try:
        import torch
        if torch.cuda.is_available():
            acceleration_available.append("cuda")
    except ImportError:
        pass

    # CPU is always available
    acceleration_available.append("cpu")

    info["hardware_acceleration_available"] = acceleration_available

    return info

def main():
    """Main entry point for the script."""
    parser = argparse.ArgumentParser(description='Docling Document Processing Wrapper')

    # Command selection
    subparsers = parser.add_subparsers(dest='command', help='Available commands')

    # Process document command
    process_parser = subparsers.add_parser('process', help='Process a document')
    process_parser.add_argument('source', help='Document source (file path or URL)')
    process_parser.add_argument('--processing-mode', default='basic',
                               choices=['basic', 'advanced', 'ocr', 'tables', 'images'],
                               help='Processing mode')
    process_parser.add_argument('--enable-ocr', action='store_true', help='Enable OCR processing')
    process_parser.add_argument('--ocr-languages', nargs='+', default=['en'],
                               help='OCR language codes')
    process_parser.add_argument('--preserve-images', action='store_true',
                               help='Extract and preserve images')
    process_parser.add_argument('--output-format', default='markdown',
                               choices=['markdown', 'json', 'both'],
                               help='Output format')
    process_parser.add_argument('--table-former-mode', default='accurate',
                               choices=['fast', 'accurate'],
                               help='TableFormer processing mode for table structure recognition')
    process_parser.add_argument('--cell-matching', action='store_true', default=None,
                               help='Use PDF cells for table matching (default)')
    process_parser.add_argument('--no-cell-matching', action='store_true',
                               help='Use predicted text cells for table matching')
    process_parser.add_argument('--vision-mode', default='standard',
                               choices=['standard', 'smoldocling', 'advanced'],
                               help='Vision processing mode for enhanced document understanding')
    process_parser.add_argument('--diagram-description', action='store_true',
                               help='Enable diagram and chart description using vision models')
    process_parser.add_argument('--chart-data-extraction', action='store_true',
                               help='Enable data extraction from charts and graphs')
    process_parser.add_argument('--enable-remote-services', action='store_true',
                               help='Allow communication with external vision model services')

    # System info command
    info_parser = subparsers.add_parser('info', help='Get system information')

    args = parser.parse_args()

    if args.command == 'process':
        result = process_document(args)
    elif args.command == 'info':
        result = get_system_info()
    else:
        parser.print_help()
        sys.exit(1)

    # Output result as JSON
    print(json.dumps(result, indent=2))

if __name__ == '__main__':
    main()
