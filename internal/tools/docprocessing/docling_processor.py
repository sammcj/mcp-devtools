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
        from docling.datamodel.settings import settings
        from docling.utils.accelerator_utils import AcceleratorDevice

        # Check if the accelerator_device attribute exists
        if hasattr(settings.perf, 'accelerator_device'):
            # Try to use MPS (Metal Performance Shaders) on macOS first
            import platform
            if platform.system() == 'Darwin':
                try:
                    import torch
                    if torch.backends.mps.is_available():
                        settings.perf.accelerator_device = AcceleratorDevice.MPS
                        # MPS acceleration configured
                        return AcceleratorDevice.MPS
                except ImportError:
                    pass

            # Try CUDA if available
            try:
                import torch
                if torch.cuda.is_available():
                    settings.perf.accelerator_device = AcceleratorDevice.CUDA
                    # CUDA acceleration configured
                    return AcceleratorDevice.CUDA
            except ImportError:
                pass

            # Fall back to CPU
            settings.perf.accelerator_device = AcceleratorDevice.CPU
            # CPU processing configured
            return AcceleratorDevice.CPU
        else:
            # Accelerator device configuration not supported in this version of Docling
            return None

    except Exception as e:
        logger.warning(f"Failed to configure accelerator: {e}")
        return None

def cleanup_memory():
    """Force garbage collection to free up memory."""
    gc.collect()

def get_cache_key(source: str, processing_mode: str, enable_ocr: bool, ocr_languages: Optional[List[str]], preserve_images: bool) -> str:
    """Generate a cache key for the document conversion."""
    key_data = {
        "source": source,
        "processing_mode": processing_mode,
        "enable_ocr": enable_ocr,
        "ocr_languages": ocr_languages or [],
        "preserve_images": preserve_images
    }
    key_str = json.dumps(key_data, sort_keys=True)
    return hashlib.md5(key_str.encode()).hexdigest()

def process_document(args) -> Dict[str, Any]:
    """Process a document using Docling."""
    start_time = time.time()

    try:
        # Import Docling components
        from docling.document_converter import DocumentConverter, PdfFormatOption
        from docling.datamodel.base_models import InputFormat
        from docling.datamodel.pipeline_options import (
            PdfPipelineOptions,
            EasyOcrOptions
        )

        # Configure hardware acceleration
        hardware_acceleration = configure_accelerator()

        # Configure OCR if enabled
        format_options = {}
        if args.enable_ocr:
            ocr_options = EasyOcrOptions(lang=args.ocr_languages or ["en"])
            pipeline_options = PdfPipelineOptions(
                do_ocr=True,
                ocr_options=ocr_options
            )
            format_options = {
                InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline_options)
            }

        # Create converter
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

        # Export to markdown
        markdown_output = result.document.export_to_markdown()

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

        return {
            "success": True,
            "content": markdown_output,
            "metadata": metadata,
            "images": images,
            "tables": tables,
            "processing_info": {
                "processing_mode": args.processing_mode,
                "hardware_acceleration": str(hardware_acceleration) if hardware_acceleration else "unknown",
                "ocr_enabled": args.enable_ocr,
                "ocr_languages": args.ocr_languages or [],
                "processing_time": processing_time,
                "timestamp": time.time()
            }
        }

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
    """Extract tables from the document."""
    tables = []

    try:
        # This is a placeholder - actual implementation would depend on
        # Docling's API for table extraction
        pass  # Table extraction not yet implemented
    except Exception as e:
        logger.warning(f"Failed to extract tables: {e}")

    return tables

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
