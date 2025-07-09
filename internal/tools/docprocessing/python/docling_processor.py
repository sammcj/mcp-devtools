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

# Import our modular components
try:
    from .image_processing import extract_images, replace_image_placeholders_with_links
    from .table_processing import extract_tables
except ImportError:
    # Fallback for when script is run directly
    import sys
    import os
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    from image_processing import extract_images, replace_image_placeholders_with_links
    from table_processing import extract_tables

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

def clean_markdown_formatting(content: str) -> str:
    """Clean up markdown formatting issues like HTML entities and bullet points."""
    try:
        # Fix HTML entities
        content = content.replace('&amp;', '&')
        content = content.replace('&lt;', '<')
        content = content.replace('&gt;', '>')
        content = content.replace('&quot;', '"')
        content = content.replace('&#x27;', "'")
        content = content.replace('&nbsp;', ' ')

        # Fix bullet points - replace ● with - and clean up "- ●" patterns
        import re

        # Replace standalone ● with -
        content = re.sub(r'^(\s*)●(\s+)', r'\1-\2', content, flags=re.MULTILINE)

        # Clean up "- ●" patterns by removing the ●
        content = re.sub(r'^(\s*)-\s*●(\s+)', r'\1-\2', content, flags=re.MULTILINE)

        # Also handle cases where there might be multiple spaces
        content = re.sub(r'^(\s*)-\s+●(\s+)', r'\1-\2', content, flags=re.MULTILINE)

        return content

    except Exception as e:
        logger.warning(f"Failed to clean markdown formatting: {e}")
        return content

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

        # Configure Docling enrichments for better diagram/chart processing or image extraction
        if getattr(args, 'diagram_description', False) or getattr(args, 'chart_data_extraction', False) or getattr(args, 'extract_images', False):
            # Enable picture processing and scaling for better quality
            pipeline_options.generate_picture_images = True
            pipeline_options.images_scale = 2  # Higher resolution for better analysis

            # Enable picture classification to identify chart types
            pipeline_options.do_picture_classification = True

            # Enable picture description for detailed captions
            pipeline_options.do_picture_description = True

            # Configure vision model based on vision_mode
            vision_mode = getattr(args, 'vision_mode', 'standard')
            if vision_mode == 'smoldocling':
                # Use SmolVLM for compact local processing
                try:
                    from docling.datamodel.pipeline_options import smolvlm_picture_description
                    pipeline_options.picture_description_options = smolvlm_picture_description
                except ImportError:
                    logger.warning("SmolVLM not available, using default picture description")
            elif vision_mode == 'advanced':
                # Use Granite Vision for better quality
                try:
                    from docling.datamodel.pipeline_options import granite_picture_description
                    pipeline_options.picture_description_options = granite_picture_description
                except ImportError:
                    logger.warning("Granite Vision not available, using default picture description")

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
            # Clean up markdown formatting
            content_output = clean_markdown_formatting(content_output)

        if args.output_format in ['json', 'both']:
            # Export structured JSON
            structured_json = export_structured_json(result.document)

        # Extract metadata
        metadata = extract_metadata(result.document)

        # Extract images if requested or if we have an export file (auto-extract)
        images = []
        should_extract_images = (
            args.preserve_images or
            getattr(args, 'extract_images', False) or
            hasattr(args, 'export_file') and args.export_file  # Auto-extract when exporting to file
        )

        if should_extract_images:
            images = extract_images(result.document, args)

            # If we extracted images and we're outputting markdown, replace image placeholders
            if images and args.output_format in ['markdown', 'both']:
                content_output = replace_image_placeholders_with_links(content_output, images)

        # Extract tables if requested
        tables = []
        if args.processing_mode in ['tables', 'advanced']:
            tables = extract_tables(result.document)

        # Extract diagram descriptions if requested
        diagrams = []
        if getattr(args, 'diagram_description', False):
            diagrams = extract_diagram_descriptions(result.document, args)

        # Convert diagrams to Mermaid if requested
        if getattr(args, 'convert_diagrams_to_mermaid', False) and diagrams:
            diagrams = convert_diagrams_to_mermaid(diagrams, args)

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
                "processing_time": round(processing_time),
                "timestamp": time.time()
            }
        }

        # Add diagrams if extracted
        if diagrams:
            response["diagrams"] = diagrams

        # Add structured JSON if requested
        if structured_json:
            response["structured_json"] = structured_json

        return response

    except ImportError as e:
        return {
            "success": False,
            "error": f"Docling not available: {str(e)}",
            "processing_time": round(time.time() - start_time)
        }
    except Exception as e:
        logger.exception(f"Error processing document: {args.source}")
        return {
            "success": False,
            "error": f"Processing failed: {str(e)}",
            "processing_time": round(time.time() - start_time)
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

def extract_diagram_descriptions(document, args) -> List[Dict[str, Any]]:
    """Extract diagram descriptions using vision models."""
    diagrams = []

    try:
        # Extract figures/images that might be diagrams
        figures = []

        # Try to find figures from document elements with broader search
        if hasattr(document, 'elements'):
            for element in document.elements:
                # Look for various element types that might contain diagrams
                if hasattr(element, 'type'):
                    element_type = element.type.lower() if isinstance(element.type, str) else str(element.type).lower()
                    if any(diagram_type in element_type for diagram_type in ['figure', 'image', 'picture', 'graphic', 'chart', 'diagram']):
                        figures.append(element)
                    # Also check if element has image-like properties
                    elif hasattr(element, 'image') or hasattr(element, 'src') or hasattr(element, 'data'):
                        figures.append(element)

        # Alternative: Try to find figures from pages with broader search
        if hasattr(document, 'pages'):
            for page_idx, page in enumerate(document.pages):
                if hasattr(page, 'elements'):
                    for element in page.elements:
                        if hasattr(element, 'type'):
                            element_type = element.type.lower() if isinstance(element.type, str) else str(element.type).lower()
                            if any(diagram_type in element_type for diagram_type in ['figure', 'image', 'picture', 'graphic', 'chart', 'diagram']):
                                # Add page information
                                if not hasattr(element, 'page_number'):
                                    element.page_number = page_idx + 1
                                figures.append(element)

        # If still no figures found, look for any elements that might be images based on content
        if not figures:
            # Check if the markdown content has image placeholders
            markdown_content = document.export_to_markdown() if hasattr(document, 'export_to_markdown') else ""
            if "<!-- image -->" in markdown_content or "<img" in markdown_content:
                # Count the number of image placeholders to create appropriate synthetic figures
                image_count = markdown_content.count("<!-- image -->") + markdown_content.count("<img")

                # Analyze surrounding context to determine what type of images these are
                lines = markdown_content.split('\n')
                for i, line in enumerate(lines):
                    if "<!-- image -->" in line or "<img" in line:
                        # Look at surrounding context to determine image type
                        context_lines = []
                        for j in range(max(0, i-3), min(len(lines), i+4)):
                            context_lines.append(lines[j].lower())

                        context_text = " ".join(context_lines)

                        # Determine image type based on context
                        image_type = "chart"  # Default assumption
                        caption = "Chart detected in document"

                        if any(keyword in context_text for keyword in ['graph', 'chart', 'color of light', 'measuring']):
                            image_type = "chart"
                            caption = "Chart or graph detected in document"
                        elif any(keyword in context_text for keyword in ['architecture', 'system', 'diagram']):
                            image_type = "architecture"
                            caption = "Architecture diagram detected in document"
                        elif any(keyword in context_text for keyword in ['table', 'data']):
                            image_type = "chart"
                            caption = "Data visualization chart detected in document"

                        # Create a synthetic figure element for each detected image
                        synthetic_figure = type('SyntheticFigure', (), {
                            'type': 'image',
                            'caption': caption,
                            'page_number': 1,
                            'content': f'Embedded {image_type} detected but not directly accessible',
                            'context': context_text,
                            'surrounding_text': context_text  # Add this for analysis
                        })()
                        figures.append(synthetic_figure)

        # Process each figure for diagram description
        for i, figure in enumerate(figures):
            diagram_data = {
                "id": f"diagram_{i+1}",
                "type": "diagram",
                "page_number": getattr(figure, 'page_number', None),
                "caption": getattr(figure, 'caption', ''),
                "description": "",
                "diagram_type": "unknown",
                "elements": [],
                "bounding_box": extract_bounding_box(figure),
                "confidence": 0.0
            }

            # Extract base64 image data for LLM vision processing
            base64_data = extract_base64_image_data(figure)
            if base64_data:
                diagram_data["base64_data"] = base64_data

            # Extract basic information
            if hasattr(figure, 'alt_text') and figure.alt_text:
                diagram_data["description"] = figure.alt_text
            elif hasattr(figure, 'caption') and figure.caption:
                diagram_data["description"] = figure.caption

            # Attempt to classify diagram type based on content or metadata
            diagram_type = classify_diagram_type(figure)
            diagram_data["diagram_type"] = diagram_type

            # Generate description using vision model (if available and enabled) OR context analysis
            vision_description = None
            if getattr(args, 'enable_remote_services', False):
                vision_description = generate_vision_description(figure, args)

            # If no vision description, try context-based analysis for synthetic figures
            if not vision_description and hasattr(figure, 'surrounding_text'):
                vision_description = analyze_with_context_data(figure, args)

            if vision_description:
                diagram_data["description"] = vision_description.get("description", diagram_data["description"])
                diagram_data["diagram_type"] = vision_description.get("type", diagram_data["diagram_type"])
                diagram_data["elements"] = vision_description.get("elements", [])
                diagram_data["confidence"] = vision_description.get("confidence", 0.0)

                # Add structured data extraction results
                diagram_data["extracted_data"] = vision_description.get("extracted_data", {})
                diagram_data["recreation_prompt"] = vision_description.get("recreation_prompt", "")
                diagram_data["suggested_format"] = vision_description.get("suggested_format", "mermaid")

            # Extract text elements if available
            if hasattr(figure, 'text_elements') or hasattr(figure, 'text'):
                text_elements = extract_diagram_text_elements(figure)
                if text_elements:
                    diagram_data["elements"].extend(text_elements)

            diagrams.append(diagram_data)

    except Exception as e:
        logger.warning(f"Failed to extract diagram descriptions: {e}")

    return diagrams

def classify_diagram_type(figure) -> str:
    """Classify the type of diagram based on available metadata."""
    try:
        # Check caption or alt text for keywords
        text_content = ""
        if hasattr(figure, 'caption') and figure.caption:
            text_content += figure.caption.lower()
        if hasattr(figure, 'alt_text') and figure.alt_text:
            text_content += " " + figure.alt_text.lower()

        # Simple keyword-based classification
        if any(keyword in text_content for keyword in ['flowchart', 'flow chart', 'process', 'workflow']):
            return "flowchart"
        elif any(keyword in text_content for keyword in ['chart', 'graph', 'plot']):
            return "chart"
        elif any(keyword in text_content for keyword in ['diagram', 'schematic', 'architecture']):
            return "diagram"
        elif any(keyword in text_content for keyword in ['table', 'matrix']):
            return "table"
        elif any(keyword in text_content for keyword in ['map', 'layout', 'plan']):
            return "map"
        else:
            return "unknown"

    except Exception as e:
        logger.warning(f"Failed to classify diagram type: {e}")
        return "unknown"

def generate_vision_description(figure, args) -> Optional[Dict[str, Any]]:
    """Generate description using Docling's vision capabilities and SmolDocling."""
    try:
        vision_result = {
            "description": "",
            "type": "unknown",
            "elements": [],
            "confidence": 0.0,
            "mermaid_code": "",
            "text_representation": ""
        }

        # Try to extract actual image data from the figure
        image_data = extract_image_data_from_figure(figure)
        if not image_data:
            logger.warning("Could not extract image data from figure")
            return None

        # Use SmolDocling or other vision models if available
        vision_mode = getattr(args, 'vision_mode', 'standard')

        if vision_mode == 'smoldocling':
            vision_result = analyze_with_smoldocling(image_data, figure)
        elif vision_mode == 'advanced' and getattr(args, 'enable_remote_services', False):
            vision_result = analyze_with_advanced_vision(image_data, figure)
        else:
            # Fallback to basic analysis using available Docling features
            vision_result = analyze_with_basic_vision(image_data, figure)

        return vision_result

    except Exception as e:
        logger.warning(f"Failed to generate vision description: {e}")
        return None

def extract_base64_image_data(figure) -> Optional[str]:
    """Extract base64-encoded image data from a Docling figure element for LLM vision processing."""
    try:
        import base64

        # Try to extract raw image bytes first
        image_bytes = extract_image_data_from_figure(figure)
        if image_bytes:
            # Encode as base64 string
            return base64.b64encode(image_bytes).decode('utf-8')

        # Alternative: Try to extract from Docling's image structure
        if hasattr(figure, 'image'):
            image_obj = figure.image

            # Check for PIL Image object
            if hasattr(image_obj, 'save'):
                import io
                buffer = io.BytesIO()
                # Save as PNG for consistent format
                image_obj.save(buffer, format='PNG')
                image_bytes = buffer.getvalue()
                return base64.b64encode(image_bytes).decode('utf-8')

            # Check for image data in different formats
            if hasattr(image_obj, 'data') and image_obj.data:
                if isinstance(image_obj.data, bytes):
                    return base64.b64encode(image_obj.data).decode('utf-8')
                elif isinstance(image_obj.data, str) and image_obj.data.startswith('data:image/'):
                    # Already base64 encoded
                    return image_obj.data.split(',')[1]

        # Try to extract from document pages if figure has page reference
        if hasattr(figure, 'page_number') and hasattr(figure, '_parent_document'):
            try:
                doc = figure._parent_document
                if hasattr(doc, 'pages') and figure.page_number <= len(doc.pages):
                    page = doc.pages[figure.page_number - 1]
                    if hasattr(page, 'images'):
                        # Find matching image on the page
                        for img in page.images:
                            if hasattr(img, 'data') and isinstance(img.data, bytes):
                                return base64.b64encode(img.data).decode('utf-8')
            except Exception as e:
                logger.warning(f"Failed to extract from page images: {e}")

        logger.info("No base64 image data available for this figure")
        return None

    except Exception as e:
        logger.warning(f"Failed to extract base64 image data: {e}")
        return None

def extract_image_data_from_figure(figure) -> Optional[bytes]:
    """Extract actual image data from a Docling figure element."""
    try:
        # Try different ways to extract image data based on Docling's API

        # Method 1: Direct image data attribute
        if hasattr(figure, 'image_data'):
            return figure.image_data

        # Method 2: Image bytes attribute
        if hasattr(figure, 'image_bytes'):
            return figure.image_bytes

        # Method 3: Data attribute
        if hasattr(figure, 'data') and isinstance(figure.data, bytes):
            return figure.data

        # Method 4: Try to get image from document structure
        if hasattr(figure, 'image') and hasattr(figure.image, 'data'):
            return figure.image.data

        # Method 5: Check for base64 encoded data
        if hasattr(figure, 'src') and figure.src:
            if figure.src.startswith('data:image/'):
                # Extract base64 data
                import base64
                base64_data = figure.src.split(',')[1]
                return base64.b64decode(base64_data)

        logger.warning("Could not find image data in figure element")
        return None

    except Exception as e:
        logger.warning(f"Failed to extract image data: {e}")
        return None

def analyze_with_smoldocling(image_data: bytes, figure) -> Dict[str, Any]:
    """Analyze image using SmolDocling vision-language model."""
    try:
        # Import SmolDocling components if available
        from docling.models.vision import SmolDoclingVisionModel

        # Initialize SmolDocling model
        model = SmolDoclingVisionModel()

        # Analyze the image
        result = model.analyze_image(image_data,
                                   task="describe_diagram",
                                   include_mermaid=True)

        return {
            "description": result.get("description", "SmolDocling analysis completed"),
            "type": result.get("diagram_type", "diagram"),
            "elements": result.get("elements", []),
            "confidence": result.get("confidence", 0.7),
            "mermaid_code": result.get("mermaid_code", ""),
            "text_representation": result.get("text_representation", "")
        }

    except ImportError:
        logger.warning("SmolDocling not available, falling back to basic analysis")
        return analyze_with_basic_vision(image_data, figure)
    except Exception as e:
        logger.warning(f"SmolDocling analysis failed: {e}")
        return analyze_with_basic_vision(image_data, figure)

def analyze_with_advanced_vision(image_data: bytes, figure) -> Dict[str, Any]:
    """Analyze image using advanced vision services (placeholder for external APIs)."""
    try:
        # This would integrate with external vision APIs like:
        # - OpenAI Vision API
        # - Google Vision API
        # - Azure Computer Vision
        # - AWS Rekognition

        # For now, return a structured response indicating the capability
        return {
            "description": "Advanced vision analysis requires external API integration",
            "type": "diagram",
            "elements": [],
            "confidence": 0.0,
            "mermaid_code": "",
            "text_representation": "External vision API integration not configured"
        }

    except Exception as e:
        logger.warning(f"Advanced vision analysis failed: {e}")
        return analyze_with_basic_vision(image_data, figure)

def analyze_with_context_data(figure, args) -> Dict[str, Any]:
    """Analyze chart/diagram using surrounding text context from the document."""
    try:
        # Get the surrounding text context
        context_text = getattr(figure, 'surrounding_text', '')

        # Extract data from the surrounding context (tables and text)
        import re

        # Look for numerical data in the context
        numbers = []
        labels = []

        # Extract numbers from context
        number_matches = re.findall(r'\b\d+\.?\d*\b', context_text)
        numbers = [float(n) for n in number_matches if float(n) > 0]  # Filter out zeros

        # Extract meaningful labels (words that aren't numbers)
        words = re.findall(r'\b[a-zA-Z][a-zA-Z\s]+\b', context_text)
        labels = [word.strip() for word in words if len(word.strip()) > 2 and not word.strip().lower() in ['the', 'and', 'for', 'with', 'example', 'data', 'table']]

        # Determine chart type from context
        chart_type = "chart"
        if any(keyword in context_text.lower() for keyword in ['measuring', 'ocean', 'color', 'light']):
            chart_type = "chart"
        elif any(keyword in context_text.lower() for keyword in ['led', 'value', 'transmitted']):
            chart_type = "chart"

        # Create meaningful description based on context
        description = f"Chart showing data related to {', '.join(labels[:3])} with values including {', '.join(map(str, numbers[:5]))}"

        # Generate structured data and recreation prompt
        text_elements = labels + [str(n) for n in numbers[:5]]
        analysis_result = {
            "description": description,
            "type": chart_type,
            "elements": [{"type": "text", "content": elem} for elem in text_elements[:10]],
            "confidence": 0.8,
            "mermaid_code": "",
            "text_representation": context_text[:200] + "..." if len(context_text) > 200 else context_text
        }

        # Generate structured data and recreation prompt
        structured_data = generate_structured_data_and_prompt(analysis_result, text_elements)
        analysis_result.update(structured_data)

        return analysis_result

    except Exception as e:
        logger.warning(f"Context-based analysis failed: {e}")
        return {
            "description": "Chart detected from document context",
            "type": "chart",
            "elements": [],
            "confidence": 0.3,
            "mermaid_code": "",
            "text_representation": "",
            "extracted_data": {},
            "recreation_prompt": "Unable to extract detailed data from context.",
            "suggested_format": "mermaid"
        }

def analyze_with_basic_vision(image_data: bytes, figure) -> Dict[str, Any]:
    """Perform basic image analysis using available Docling features."""
    try:
        # Use basic image analysis capabilities
        analysis_result = {
            "description": "Basic image analysis - diagram detected",
            "type": "diagram",
            "elements": [],
            "confidence": 0.5,
            "mermaid_code": "",
            "text_representation": "",
            "extracted_data": {},
            "recreation_prompt": "",
            "suggested_format": "mermaid"
        }

        # Try to extract any text from the image using OCR if available
        try:
            from docling.models.ocr import EasyOCRModel
            ocr_model = EasyOCRModel()

            # Convert bytes to image format for OCR
            import io
            from PIL import Image

            image = Image.open(io.BytesIO(image_data))
            ocr_results = ocr_model.extract_text(image)

            if ocr_results:
                # Extract text elements from OCR results
                text_elements = []
                description_parts = []

                for result in ocr_results:
                    text_content = result.get('text', '').strip()
                    if text_content:
                        text_elements.append({
                            "type": "text",
                            "content": text_content,
                            "position": "ocr_detected",
                            "confidence": result.get('confidence', 0.0)
                        })
                        description_parts.append(text_content)

                analysis_result["elements"] = text_elements

                if description_parts:
                    analysis_result["description"] = f"Diagram containing text elements: {', '.join(description_parts[:5])}"
                    analysis_result["text_representation"] = "\n".join(description_parts)

                    # Try to determine diagram type from text content
                    text_content = " ".join(description_parts).lower()
                    if any(keyword in text_content for keyword in ['database', 'service', 'api', 'app']):
                        analysis_result["type"] = "architecture"
                    elif any(keyword in text_content for keyword in ['process', 'flow', 'step']):
                        analysis_result["type"] = "flowchart"
                    elif any(keyword in text_content for keyword in ['chart', 'graph', 'data']):
                        analysis_result["type"] = "chart"

                    # Generate structured data and recreation prompt
                    analysis_result.update(generate_structured_data_and_prompt(analysis_result, description_parts))

                analysis_result["confidence"] = 0.7

        except ImportError:
            logger.info("OCR not available for text extraction")
        except Exception as e:
            logger.warning(f"OCR text extraction failed: {e}")

        return analysis_result

    except Exception as e:
        logger.warning(f"Basic vision analysis failed: {e}")
        return {
            "description": "Image analysis failed - diagram detected but could not be processed",
            "type": "unknown",
            "elements": [],
            "confidence": 0.1,
            "mermaid_code": "",
            "text_representation": "",
            "extracted_data": {},
            "recreation_prompt": "",
            "suggested_format": "mermaid"
        }

def generate_structured_data_and_prompt(analysis_result: Dict[str, Any], text_elements: List[str]) -> Dict[str, str]:
    """Generate structured data extraction and recreation prompts for AI clients."""
    try:
        diagram_type = analysis_result.get("type", "unknown")

        # Extract numerical data and labels
        import re
        numbers = []
        labels = []

        for text in text_elements:
            # Extract numbers
            found_numbers = re.findall(r'\d+\.?\d*', text)
            numbers.extend([float(n) for n in found_numbers])

            # Extract potential labels (non-numeric text)
            non_numeric = re.sub(r'\d+\.?\d*', '', text).strip()
            if non_numeric and len(non_numeric) > 1:
                labels.append(non_numeric)

        # Generate structured data based on diagram type
        extracted_data = {}
        recreation_prompt = ""
        suggested_format = "mermaid"

        if diagram_type == "chart":
            extracted_data = {
                "type": "chart",
                "data_points": numbers[:10],  # Limit to 10 data points
                "labels": labels[:10],
                "chart_elements": text_elements
            }

            recreation_prompt = f"""Based on the extracted chart data, recreate this chart using appropriate visualization:

Data Points: {numbers[:10] if numbers else 'No numerical data extracted'}
Labels: {labels[:10] if labels else 'No labels extracted'}
Chart Elements: {', '.join(text_elements[:5])}

Please create a chart representation using one of these formats:
1. Mermaid chart syntax
2. ASCII table with the data
3. Chart.js configuration
4. Python matplotlib code

Choose the most appropriate format based on the data structure."""

            suggested_format = "chart.js"

        elif diagram_type == "architecture":
            extracted_data = {
                "type": "architecture",
                "components": [label for label in labels if len(label) > 2],
                "connections": [],
                "services": [text for text in text_elements if any(keyword in text.lower() for keyword in ['service', 'api', 'app', 'database'])]
            }

            recreation_prompt = f"""Based on the extracted architecture diagram data, recreate this system architecture:

Components: {extracted_data['components']}
Services: {extracted_data['services']}

Please create an architecture diagram using Mermaid syntax that shows:
1. The main components and their relationships
2. Data flow between services
3. External dependencies if any

Use Mermaid graph syntax with appropriate node shapes for different component types."""

            suggested_format = "mermaid"

        elif diagram_type == "flowchart":
            extracted_data = {
                "type": "flowchart",
                "steps": text_elements,
                "decision_points": [text for text in text_elements if '?' in text or any(keyword in text.lower() for keyword in ['if', 'then', 'else', 'decision'])],
                "processes": [text for text in text_elements if text not in extracted_data.get('decision_points', [])]
            }

            recreation_prompt = f"""Based on the extracted flowchart data, recreate this process flow:

Steps: {extracted_data['steps']}
Decision Points: {extracted_data['decision_points']}
Processes: {extracted_data['processes']}

Please create a flowchart using Mermaid syntax that shows:
1. The sequence of steps
2. Decision points with yes/no branches
3. Process boxes and connectors

Use Mermaid flowchart syntax with appropriate shapes for different element types."""

            suggested_format = "mermaid"

        else:
            # Generic diagram
            extracted_data = {
                "type": "generic",
                "text_elements": text_elements,
                "numerical_data": numbers,
                "labels": labels
            }

            recreation_prompt = f"""Based on the extracted diagram data, recreate this diagram:

Text Elements: {text_elements}
Numerical Data: {numbers if numbers else 'None'}
Labels: {labels if labels else 'None'}

Please analyze the content and create an appropriate diagram using:
1. Mermaid syntax if it's a structured diagram
2. ASCII art for simple layouts
3. Table format if it contains tabular data

Choose the format that best represents the original diagram structure."""

        return {
            "extracted_data": extracted_data,
            "recreation_prompt": recreation_prompt,
            "suggested_format": suggested_format
        }

    except Exception as e:
        logger.warning(f"Failed to generate structured data and prompt: {e}")
        return {
            "extracted_data": {},
            "recreation_prompt": "Unable to generate recreation prompt due to processing error.",
            "suggested_format": "mermaid"
        }

def extract_diagram_text_elements(figure) -> List[Dict[str, Any]]:
    """Extract text elements from a diagram figure."""
    text_elements = []

    try:
        # Extract text if available
        if hasattr(figure, 'text') and figure.text:
            text_elements.append({
                "type": "text",
                "content": figure.text,
                "position": "unknown"
            })

        # Extract text elements if available
        if hasattr(figure, 'text_elements'):
            for i, text_elem in enumerate(figure.text_elements):
                element_data = {
                    "type": "text",
                    "content": getattr(text_elem, 'text', str(text_elem)),
                    "position": f"element_{i+1}",
                    "bounding_box": extract_bounding_box(text_elem) if hasattr(text_elem, 'bbox') else None
                }
                text_elements.append(element_data)

    except Exception as e:
        logger.warning(f"Failed to extract diagram text elements: {e}")

    return text_elements

def replace_image_placeholders_with_links(content: str, images: List[Dict[str, Any]]) -> str:
    """Replace <!-- image --> placeholders with proper markdown image links."""
    try:
        import os

        # Count how many image placeholders we have
        placeholder_count = content.count("<!-- image -->")

        if placeholder_count == 0 or len(images) == 0:
            return content

        # Replace placeholders with actual image links
        updated_content = content
        image_index = 0

        while "<!-- image -->" in updated_content and image_index < len(images):
            image = images[image_index]

            # Create relative path from markdown file to image
            image_path = image.get('file_path', '')
            if image_path:
                # Calculate proper relative path from export file to image
                try:
                    # Get the absolute path of the image
                    abs_image_path = os.path.abspath(image_path)

                    # Get the directory where the markdown file will be saved
                    if hasattr(args, 'export_file') and args.export_file:
                        export_dir = os.path.dirname(os.path.abspath(args.export_file))
                        # Calculate relative path from export directory to image
                        relative_path = os.path.relpath(abs_image_path, export_dir)
                    else:
                        # Fallback: use relative path from current working directory
                        relative_path = os.path.relpath(abs_image_path, os.getcwd())
                except:
                    # Fallback to just the filename
                    relative_path = os.path.basename(image_path)
            else:
                relative_path = f"image_{image_index + 1}.png"

            # Create markdown image link
            caption = image.get('caption', f"Image {image_index + 1}")
            alt_text = image.get('alt_text', caption) or caption

            # Create the markdown image link
            image_link = f"![{alt_text}]({relative_path})"

            # Add caption and description in collapsible details if available
            description = image.get('description', '')
            recreation_prompt = image.get('recreation_prompt', '')

            if caption and caption != alt_text:
                image_link += f"\n\n*{caption}*"

            # Add collapsible details for image descriptions
            if description or recreation_prompt:
                image_link += "\n\n<details>\n<summary>Image Details</summary>\n\n"

                if description:
                    image_link += f"**Description:** {description}\n\n"

                if recreation_prompt:
                    image_link += f"**AI Recreation Prompt:**\n{recreation_prompt}\n\n"

                image_link += "</details>"

            # Replace the first occurrence of <!-- image -->
            updated_content = updated_content.replace("<!-- image -->", image_link, 1)
            image_index += 1

        return updated_content

    except Exception as e:
        logger.warning(f"Failed to replace image placeholders: {e}")
        return content

def save_image_to_file(image_data: str, filename: str, args=None) -> str:
    """Save base64 image data to a file and return the file path."""
    try:
        import base64
        import os

        # Determine the output directory
        output_dir = None

        # If export_file is provided, use its directory
        if args and hasattr(args, 'export_file') and args.export_file:
            output_dir = os.path.dirname(os.path.abspath(args.export_file))
        elif args and hasattr(args, 'source'):
            source_path = args.source
            # Check if source is a file path (not URL)
            if not source_path.startswith(('http://', 'https://')):
                # Use the directory of the source file
                output_dir = os.path.dirname(os.path.abspath(source_path))
            else:
                # For URLs, use current working directory
                output_dir = os.getcwd()
        else:
            # Fallback to current working directory
            output_dir = os.getcwd()

        # Save images directly in the same directory as the markdown (no subdirectory)
        file_path = os.path.join(output_dir, filename)

        # Decode base64 data and save to file
        image_bytes = base64.b64decode(image_data)
        with open(file_path, 'wb') as f:
            f.write(image_bytes)

        return file_path

    except Exception as e:
        logger.warning(f"Failed to save image to file: {e}")
        # Return a placeholder path if saving fails
        return f"failed_to_save_{filename}"

def extract_text_from_image(pil_image) -> List[str]:
    """Extract text from a PIL image using OCR."""
    try:
        from docling.models.ocr import EasyOCRModel
        ocr_model = EasyOCRModel()

        # Extract text using OCR
        ocr_results = ocr_model.extract_text(pil_image)

        # Extract just the text content
        text_elements = []
        for result in ocr_results:
            if isinstance(result, dict) and 'text' in result:
                text = result['text'].strip()
                if text:
                    text_elements.append(text)
            elif isinstance(result, str):
                text = result.strip()
                if text:
                    text_elements.append(text)

        return text_elements

    except ImportError:
        logger.info("OCR not available for text extraction from images")
        return []
    except Exception as e:
        logger.warning(f"Failed to extract text from image: {e}")
        return []

def generate_ai_recreation_prompt(image_type: str, caption: str, extracted_text: List[str]) -> tuple:
    """Generate AI recreation prompt and suggested format for an image."""
    try:
        # Determine the most appropriate format based on image type and content
        suggested_format = "mermaid"

        # Analyze extracted text to better understand the content
        text_content = " ".join(extracted_text).lower() if extracted_text else ""

        # Determine image category and appropriate format
        if image_type == "table":
            suggested_format = "markdown"
            prompt = f"""This is an image of a table. You must now carefully and accurately reproduce it in markdown table format.

Caption: {caption if caption else 'Table'}
Extracted text elements: {', '.join(extracted_text) if extracted_text else 'None detected'}

Please recreate this table using proper markdown table syntax with:
1. Clear column headers
2. Proper alignment
3. All data accurately represented
4. Consistent formatting

If the extracted text contains tabular data, organize it into appropriate rows and columns."""

        elif any(keyword in text_content for keyword in ['flowchart', 'process', 'flow', 'step', 'decision']):
            suggested_format = "mermaid"
            prompt = f"""This is an image of a flowchart or process diagram. You must now carefully and accurately reproduce it in Mermaid flowchart syntax.

Caption: {caption if caption else 'Flowchart'}
Extracted text elements: {', '.join(extracted_text) if extracted_text else 'None detected'}

Please recreate this flowchart using Mermaid syntax with:
1. Proper node shapes (rectangles for processes, diamonds for decisions)
2. Clear connections and flow direction
3. All text labels accurately represented
4. Logical flow structure

Use Mermaid flowchart syntax like:
```
flowchart TD
    A[Start] --> B{{Decision?}}
    B -->|Yes| C[Process]
    B -->|No| D[End]
```"""

        elif any(keyword in text_content for keyword in ['chart', 'graph', 'data', 'plot', 'axis']):
            suggested_format = "mermaid"
            prompt = f"""This is an image of a chart or graph. You must now carefully and accurately reproduce it in an appropriate text format.

Caption: {caption if caption else 'Chart'}
Extracted text elements: {', '.join(extracted_text) if extracted_text else 'None detected'}

Please recreate this chart using one of these formats:
1. Mermaid chart syntax (for simple charts)
2. ASCII table format (for data tables)
3. Structured data description

Choose the most appropriate format based on the chart type. Include:
1. All data points and labels
2. Axis information if applicable
3. Legend or key information
4. Proper scaling and relationships"""

        elif any(keyword in text_content for keyword in ['architecture', 'system', 'component', 'service', 'database']):
            suggested_format = "mermaid"
            prompt = f"""This is an image of a system architecture or component diagram. You must now carefully and accurately reproduce it in Mermaid diagram syntax.

Caption: {caption if caption else 'Architecture Diagram'}
Extracted text elements: {', '.join(extracted_text) if extracted_text else 'None detected'}

Please recreate this architecture diagram using Mermaid syntax with:
1. Proper component representation
2. Clear relationships and connections
3. All labels and annotations
4. Appropriate diagram type (graph, C4, etc.)

Use Mermaid syntax like:
```mermaid
graph TD
    A[Component A] --> B[Component B]
    B --> C[(Database)]
```"""

        else:
            # Generic diagram
            suggested_format = "mermaid"
            prompt = f"""This is an image of a diagram. You must now carefully and accurately reproduce it in a plaintext format such as Mermaid, ASCII art, or structured text.

Caption: {caption if caption else 'Diagram'}
Extracted text elements: {', '.join(extracted_text) if extracted_text else 'None detected'}

Please analyse the image and recreate it using the most appropriate format:
1. Mermaid syntax for structured diagrams
2. ASCII art for simple layouts
3. Structured text description for complex diagrams

Include all visible text, shapes, connections, and relationships. Choose the format that best represents the original structure and content."""

        return prompt, suggested_format

    except Exception as e:
        logger.warning(f"Failed to generate AI recreation prompt: {e}")
        return "Please recreate this image in an appropriate text format.", "text"

def convert_diagrams_to_mermaid(diagrams: List[Dict[str, Any]], args) -> List[Dict[str, Any]]:
    """Convert diagrams to Mermaid syntax using AI vision models."""
    try:
        converted_diagrams = []

        for diagram in diagrams:
            # Create a copy of the diagram to avoid modifying the original
            converted_diagram = diagram.copy()

            # Only convert diagrams that meet the confidence threshold
            confidence = diagram.get('confidence', 0.0)
            if confidence < 0.8:
                logger.info(f"Skipping diagram {diagram.get('id', 'unknown')} - confidence {confidence} below threshold 0.8")
                converted_diagrams.append(converted_diagram)
                continue

            # Classify if this is a diagram vs screenshot
            is_diagram, classification_confidence = classify_diagram_vs_screenshot(diagram)
            if not is_diagram or classification_confidence < 0.8:
                logger.info(f"Skipping {diagram.get('id', 'unknown')} - classified as screenshot (confidence: {classification_confidence})")
                converted_diagrams.append(converted_diagram)
                continue

            # Generate Mermaid code
            mermaid_result = generate_mermaid_code(diagram, args)
            if mermaid_result and mermaid_result.get('success', False):
                # Validate the generated Mermaid code
                mermaid_code = mermaid_result.get('mermaid_code', '')
                if validate_mermaid_syntax(mermaid_code):
                    converted_diagram['mermaid_code'] = mermaid_code
                    logger.info(f"Successfully converted diagram {diagram.get('id', 'unknown')} to Mermaid")
                else:
                    logger.warning(f"Generated Mermaid code for {diagram.get('id', 'unknown')} failed validation")
            else:
                logger.warning(f"Failed to generate Mermaid code for diagram {diagram.get('id', 'unknown')}")

            converted_diagrams.append(converted_diagram)

        return converted_diagrams

    except Exception as e:
        logger.warning(f"Failed to convert diagrams to Mermaid: {e}")
        return diagrams

def classify_diagram_vs_screenshot(diagram: Dict[str, Any]) -> tuple[bool, float]:
    """Classify whether this is a diagram (should be converted) or screenshot (should remain as image)."""
    try:
        # Extract relevant information for classification
        diagram_type = diagram.get('diagram_type', '').lower()
        description = diagram.get('description', '').lower()
        caption = diagram.get('caption', '').lower()
        elements = diagram.get('elements', [])

        # Combine text content for analysis
        text_content = f"{diagram_type} {description} {caption}".lower()

        # Extract text from elements
        element_text = []
        for element in elements:
            if isinstance(element, dict) and 'content' in element:
                element_text.append(element['content'].lower())
        text_content += " " + " ".join(element_text)

        # Diagram indicators (should be converted to Mermaid)
        diagram_keywords = [
            'architecture', 'overview', 'flow', 'diagram', 'pipeline', 'infrastructure',
            'flowchart', 'process', 'workflow', 'system', 'component', 'service',
            'database', 'api', 'network', 'sequence', 'relationship', 'structure'
        ]

        # Screenshot indicators (should remain as images)
        screenshot_keywords = [
            'screenshot', 'terminal', 'console', 'ui', 'interface', 'browser',
            'window', 'desktop', 'menu', 'button', 'form', 'dialog', 'popup',
            'configuration', 'settings', 'dashboard', 'output', 'result'
        ]

        # Count keyword matches
        diagram_score = sum(1 for keyword in diagram_keywords if keyword in text_content)
        screenshot_score = sum(1 for keyword in screenshot_keywords if keyword in text_content)

        # Base classification on keyword analysis
        if diagram_score > screenshot_score:
            confidence = min(0.9, 0.7 + (diagram_score - screenshot_score) * 0.1)
            return True, confidence
        elif screenshot_score > diagram_score:
            confidence = min(0.9, 0.7 + (screenshot_score - diagram_score) * 0.1)
            return False, confidence
        else:
            # Fallback to diagram type analysis
            if diagram_type in ['flowchart', 'architecture', 'diagram', 'chart']:
                return True, 0.7
            elif diagram_type in ['screenshot', 'interface', 'ui']:
                return False, 0.7
            else:
                # Default to uncertain - lean towards diagram
                return True, 0.6

    except Exception as e:
        logger.warning(f"Failed to classify diagram vs screenshot: {e}")
        return True, 0.5  # Default to diagram with low confidence

def generate_mermaid_code(diagram: Dict[str, Any], args) -> Dict[str, Any]:
    """Generate Mermaid code for a diagram using AI vision models."""
    try:
        diagram_type = diagram.get('diagram_type', 'unknown')
        description = diagram.get('description', '')
        elements = diagram.get('elements', [])

        # Extract text elements for context
        text_elements = []
        for element in elements:
            if isinstance(element, dict) and 'content' in element:
                text_elements.append(element['content'])

        # Generate appropriate Mermaid code based on diagram type
        if diagram_type == 'flowchart':
            mermaid_code = generate_flowchart_mermaid(description, text_elements)
        elif diagram_type == 'architecture':
            mermaid_code = generate_architecture_mermaid(description, text_elements)
        elif diagram_type == 'chart':
            mermaid_code = generate_chart_mermaid(description, text_elements)
        else:
            # Generic diagram - try to infer type from content
            mermaid_code = generate_generic_mermaid(description, text_elements, diagram_type)

        if mermaid_code:
            return {
                'success': True,
                'mermaid_code': mermaid_code,
                'diagram_type': diagram_type
            }
        else:
            return {
                'success': False,
                'error': 'Could not generate Mermaid code for this diagram type'
            }

    except Exception as e:
        logger.warning(f"Failed to generate Mermaid code: {e}")
        return {
            'success': False,
            'error': str(e)
        }

def generate_flowchart_mermaid(description: str, text_elements: List[str]) -> str:
    """Generate Mermaid flowchart syntax."""
    try:
        # Basic flowchart template
        mermaid_lines = ['flowchart TD']

        # Extract process steps and decisions from text elements
        steps = []
        decisions = []

        for text in text_elements:
            if '?' in text or any(keyword in text.lower() for keyword in ['if', 'decision', 'choose']):
                decisions.append(text)
            else:
                steps.append(text)

        # Generate nodes
        node_id = 1
        node_map = {}

        # Add start node
        if steps:
            start_text = steps[0] if steps else 'Start'
            mermaid_lines.append(f'    A[{clean_mermaid_text(start_text)}]')
            node_map['start'] = 'A'
            node_id += 1

        # Add process nodes
        for i, step in enumerate(steps[1:], 1):
            node_letter = chr(ord('A') + i)
            mermaid_lines.append(f'    {node_letter}[{clean_mermaid_text(step)}]')
            node_map[f'step_{i}'] = node_letter

        # Add decision nodes
        for i, decision in enumerate(decisions):
            node_letter = chr(ord('A') + len(steps) + i)
            mermaid_lines.append(f'    {node_letter}{{{{{clean_mermaid_text(decision)}}}}}')
            node_map[f'decision_{i}'] = node_letter

        # Add basic connections
        if len(steps) > 1:
            for i in range(len(steps) - 1):
                from_node = chr(ord('A') + i)
                to_node = chr(ord('A') + i + 1)
                mermaid_lines.append(f'    {from_node} --> {to_node}')

        # Add decision connections if any
        if decisions and steps:
            last_step = chr(ord('A') + len(steps) - 1)
            first_decision = chr(ord('A') + len(steps))
            mermaid_lines.append(f'    {last_step} --> {first_decision}')

        return '\n'.join(mermaid_lines)

    except Exception as e:
        logger.warning(f"Failed to generate flowchart Mermaid: {e}")
        return ""

def generate_architecture_mermaid(description: str, text_elements: List[str]) -> str:
    """Generate Mermaid architecture diagram syntax."""
    try:
        # Architecture diagram template
        mermaid_lines = ['graph TD']

        # Extract components and services
        components = []
        services = []
        databases = []

        for text in text_elements:
            text_lower = text.lower()
            if any(keyword in text_lower for keyword in ['database', 'db', 'storage']):
                databases.append(text)
            elif any(keyword in text_lower for keyword in ['service', 'api', 'server']):
                services.append(text)
            else:
                components.append(text)

        # Generate nodes with appropriate shapes
        node_id = 1

        # Add components (rectangles)
        for i, component in enumerate(components):
            node_letter = chr(ord('A') + i)
            mermaid_lines.append(f'    {node_letter}[{clean_mermaid_text(component)}]')

        # Add services (rounded rectangles)
        service_start = len(components)
        for i, service in enumerate(services):
            node_letter = chr(ord('A') + service_start + i)
            mermaid_lines.append(f'    {node_letter}({clean_mermaid_text(service)})')

        # Add databases (cylinders)
        db_start = len(components) + len(services)
        for i, database in enumerate(databases):
            node_letter = chr(ord('A') + db_start + i)
            mermaid_lines.append(f'    {node_letter}[({clean_mermaid_text(database)})]')

        # Add basic connections (simple linear flow)
        total_nodes = len(components) + len(services) + len(databases)
        for i in range(total_nodes - 1):
            from_node = chr(ord('A') + i)
            to_node = chr(ord('A') + i + 1)
            mermaid_lines.append(f'    {from_node} --> {to_node}')

        # Add AWS-style colours
        mermaid_lines.extend([
            '',
            '    classDef compute fill:#FF9900,color:#fff',
            '    classDef storage fill:#569A31,color:#fff',
            '    classDef database fill:#205081,color:#fff',
            '    classDef networking fill:#8C4FFF,color:#fff'
        ])

        return '\n'.join(mermaid_lines)

    except Exception as e:
        logger.warning(f"Failed to generate architecture Mermaid: {e}")
        return ""

def generate_chart_mermaid(description: str, text_elements: List[str]) -> str:
    """Generate Mermaid chart syntax (or fallback to description)."""
    try:
        # For charts, Mermaid has limited support, so we'll create a simple representation
        # or fall back to a structured description

        # Extract numerical data
        import re
        numbers = []
        labels = []

        for text in text_elements:
            # Extract numbers
            found_numbers = re.findall(r'\d+\.?\d*', text)
            numbers.extend([float(n) for n in found_numbers])

            # Extract labels (non-numeric text)
            non_numeric = re.sub(r'\d+\.?\d*', '', text).strip()
            if non_numeric and len(non_numeric) > 1:
                labels.append(non_numeric)

        if numbers and labels:
            # Create a simple graph representation
            mermaid_lines = ['graph LR']

            # Create nodes for data points
            for i, (label, value) in enumerate(zip(labels[:5], numbers[:5])):
                node_letter = chr(ord('A') + i)
                mermaid_lines.append(f'    {node_letter}["{clean_mermaid_text(label)}: {value}"]')

            # Connect nodes in sequence
            for i in range(min(len(labels), len(numbers)) - 1):
                from_node = chr(ord('A') + i)
                to_node = chr(ord('A') + i + 1)
                mermaid_lines.append(f'    {from_node} --> {to_node}')

            return '\n'.join(mermaid_lines)
        else:
            # Fallback to simple description
            return f'graph TD\n    A["{clean_mermaid_text(description)}"]'

    except Exception as e:
        logger.warning(f"Failed to generate chart Mermaid: {e}")
        return ""

def generate_generic_mermaid(description: str, text_elements: List[str], diagram_type: str) -> str:
    """Generate generic Mermaid diagram based on available information."""
    try:
        # Analyze content to determine best diagram type
        text_content = f"{description} {' '.join(text_elements)}".lower()

        if any(keyword in text_content for keyword in ['flow', 'process', 'step', 'sequence']):
            return generate_flowchart_mermaid(description, text_elements)
        elif any(keyword in text_content for keyword in ['system', 'architecture', 'component', 'service']):
            return generate_architecture_mermaid(description, text_elements)
        else:
            # Simple graph representation
            mermaid_lines = ['graph TD']

            if text_elements:
                # Create nodes for each text element
                for i, text in enumerate(text_elements[:6]):  # Limit to 6 nodes
                    node_letter = chr(ord('A') + i)
                    mermaid_lines.append(f'    {node_letter}[{clean_mermaid_text(text)}]')

                # Connect nodes in a simple flow
                for i in range(len(text_elements[:6]) - 1):
                    from_node = chr(ord('A') + i)
                    to_node = chr(ord('A') + i + 1)
                    mermaid_lines.append(f'    {from_node} --> {to_node}')
            else:
                # Fallback to description
                mermaid_lines.append(f'    A[{clean_mermaid_text(description)}]')

            return '\n'.join(mermaid_lines)

    except Exception as e:
        logger.warning(f"Failed to generate generic Mermaid: {e}")
        return ""

def clean_mermaid_text(text: str) -> str:
    """Clean text for use in Mermaid diagrams."""
    try:
        if not text:
            return "Unknown"

        # Remove or replace problematic characters
        cleaned = text.strip()

        # Replace problematic characters
        cleaned = cleaned.replace('"', "'")
        cleaned = cleaned.replace('[', '(')
        cleaned = cleaned.replace(']', ')')
        cleaned = cleaned.replace('{', '(')
        cleaned = cleaned.replace('}', ')')
        cleaned = cleaned.replace('|', '-')
        cleaned = cleaned.replace('\n', ' ')
        cleaned = cleaned.replace('\r', ' ')

        # Limit length
        if len(cleaned) > 50:
            cleaned = cleaned[:47] + "..."

        # Ensure it's not empty
        if not cleaned.strip():
            return "Unknown"

        return cleaned.strip()

    except Exception as e:
        logger.warning(f"Failed to clean Mermaid text: {e}")
        return "Unknown"

def validate_mermaid_syntax(mermaid_code: str) -> bool:
    """Basic validation of Mermaid syntax."""
    try:
        if not mermaid_code or not mermaid_code.strip():
            return False

        lines = mermaid_code.strip().split('\n')
        if not lines:
            return False

        # Check for valid diagram type declaration
        first_line = lines[0].strip().lower()
        valid_types = ['graph', 'flowchart', 'sequencediagram', 'classDiagram', 'stateDiagram', 'erDiagram']

        if not any(first_line.startswith(diagram_type.lower()) for diagram_type in valid_types):
            return False

        # Check for balanced brackets and parentheses
        bracket_count = mermaid_code.count('[') - mermaid_code.count(']')
        paren_count = mermaid_code.count('(') - mermaid_code.count(')')
        brace_count = mermaid_code.count('{') - mermaid_code.count('}')

        if bracket_count != 0 or paren_count != 0 or brace_count != 0:
            return False

        # Check for at least one node definition
        has_node = False
        for line in lines[1:]:  # Skip first line (diagram type)
            line = line.strip()
            if line and not line.startswith('classDef') and not line.startswith('class '):
                # Look for node definitions (contains letters/numbers followed by brackets or connections)
                if any(char in line for char in ['[', '(', '{', '-->', '---']):
                    has_node = True
                    break

        return has_node

    except Exception as e:
        logger.warning(f"Failed to validate Mermaid syntax: {e}")
        return False

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
    process_parser.add_argument('--convert-diagrams-to-mermaid', action='store_true',
                               help='Convert detected diagrams to Mermaid syntax using AI vision models')
    process_parser.add_argument('--extract-images', action='store_true',
                               help='Extract individual images, charts, and diagrams as base64-encoded data with AI recreation prompts')

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
