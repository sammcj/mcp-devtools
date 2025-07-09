#!/usr/bin/env python3
"""
Image Processing Module for Docling Document Processing

This module handles image extraction, processing, and file operations.
"""

import base64
import io
import os
from typing import List, Dict, Any, Optional
import logging

logger = logging.getLogger(__name__)


def save_image_to_file(image_data: str, filename: str, args=None) -> str:
    """Save base64 image data to a file and return the file path."""
    try:
        # Determine the output directory
        # Default to same directory as source file if no export path provided
        output_dir = None

        if args and hasattr(args, 'source'):
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

        # Create images subdirectory
        images_dir = os.path.join(output_dir, 'extracted_images')
        os.makedirs(images_dir, exist_ok=True)

        # Create full file path
        file_path = os.path.join(images_dir, filename)

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


def replace_image_placeholders_with_links(content: str, images: List[Dict[str, Any]]) -> str:
    """Replace <!-- image --> placeholders with proper markdown image links."""
    try:
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
                # Get just the filename and subdirectory for relative path
                # e.g., if image is at /path/to/doc/extracted_images/picture_1.png
                # and markdown might be at /path/to/doc/output.md
                # we want: extracted_images/picture_1.png
                try:
                    image_filename = os.path.basename(image_path)
                    image_dir = os.path.basename(os.path.dirname(image_path))
                    relative_path = f"{image_dir}/{image_filename}"
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

            # Add caption if it exists and is different from alt text
            if caption and caption != alt_text:
                image_link += f"\n\n*{caption}*"

            # Replace the first occurrence of <!-- image -->
            updated_content = updated_content.replace("<!-- image -->", image_link, 1)
            image_index += 1

        return updated_content

    except Exception as e:
        logger.warning(f"Failed to replace image placeholders: {e}")
        return content


def extract_images(document, args=None) -> List[Dict[str, Any]]:
    """Extract individual images, charts, and diagrams from the document."""
    images = []

    try:
        # Import required modules for image processing
        from docling_core.types.doc import PictureItem, TableItem

        picture_counter = 0
        table_counter = 0

        # Extract images from document elements using Docling's iterate_items
        if hasattr(document, 'iterate_items'):
            for element, level in document.iterate_items():
                image_data = None
                image_type = "unknown"
                caption = ""
                page_number = None
                bounding_box = None
                extracted_text = []

                # Handle PictureItem (figures, charts, diagrams)
                if isinstance(element, PictureItem):
                    picture_counter += 1
                    image_type = "picture"

                    # Extract image data
                    try:
                        # Get the image using Docling's get_image method
                        pil_image = element.get_image(document)
                        if pil_image:
                            # Convert PIL image to base64
                            img_buffer = io.BytesIO()
                            pil_image.save(img_buffer, format='PNG')
                            img_buffer.seek(0)
                            image_data = base64.b64encode(img_buffer.getvalue()).decode('utf-8')

                            # Get image dimensions
                            width, height = pil_image.size
                        else:
                            continue  # Skip if no image data
                    except Exception as e:
                        logger.warning(f"Failed to extract image data from PictureItem: {e}")
                        continue

                    # Extract metadata
                    caption = getattr(element, 'caption', '') or getattr(element, 'text', '')
                    page_number = getattr(element, 'page_number', None)

                    # Extract bounding box
                    if hasattr(element, 'bbox') or hasattr(element, 'bounding_box'):
                        bbox = getattr(element, 'bbox', None) or getattr(element, 'bounding_box', None)
                        if bbox:
                            bounding_box = {
                                "x": getattr(bbox, 'x', 0.0),
                                "y": getattr(bbox, 'y', 0.0),
                                "width": getattr(bbox, 'width', 0.0),
                                "height": getattr(bbox, 'height', 0.0)
                            }

                    # Try to extract text from the image using OCR
                    try:
                        extracted_text = extract_text_from_image(pil_image)
                    except Exception as e:
                        logger.warning(f"Failed to extract text from image: {e}")

                # Handle TableItem (table images)
                elif isinstance(element, TableItem):
                    table_counter += 1
                    image_type = "table"

                    # Extract table image data
                    try:
                        # Get the table image using Docling's get_image method
                        pil_image = element.get_image(document)
                        if pil_image:
                            # Convert PIL image to base64
                            img_buffer = io.BytesIO()
                            pil_image.save(img_buffer, format='PNG')
                            img_buffer.seek(0)
                            image_data = base64.b64encode(img_buffer.getvalue()).decode('utf-8')

                            # Get image dimensions
                            width, height = pil_image.size
                        else:
                            continue  # Skip if no image data
                    except Exception as e:
                        logger.warning(f"Failed to extract image data from TableItem: {e}")
                        continue

                    # Extract metadata
                    caption = getattr(element, 'caption', '') or f"Table {table_counter}"
                    page_number = getattr(element, 'page_number', None)

                    # Extract bounding box
                    if hasattr(element, 'bbox') or hasattr(element, 'bounding_box'):
                        bbox = getattr(element, 'bbox', None) or getattr(element, 'bounding_box', None)
                        if bbox:
                            bounding_box = {
                                "x": getattr(bbox, 'x', 0.0),
                                "y": getattr(bbox, 'y', 0.0),
                                "width": getattr(bbox, 'width', 0.0),
                                "height": getattr(bbox, 'height', 0.0)
                            }

                    # Try to extract text from the table image
                    try:
                        extracted_text = extract_text_from_image(pil_image)
                    except Exception as e:
                        logger.warning(f"Failed to extract text from table image: {e}")

                # If we have image data, save it to file and create the image record
                if image_data:
                    # Save image to file
                    image_filename = f"{image_type}_{picture_counter if image_type == 'picture' else table_counter}.png"
                    image_file_path = save_image_to_file(image_data, image_filename, args)

                    # Create image record with file path instead of base64 data
                    image_record = {
                        "id": f"{image_type}_{picture_counter if image_type == 'picture' else table_counter}",
                        "type": image_type,
                        "caption": caption,
                        "format": "PNG",
                        "width": width,
                        "height": height,
                        "size": len(base64.b64decode(image_data)),
                        "file_path": image_file_path,
                        "page_number": page_number,
                        "bounding_box": bounding_box,
                        "extracted_text": extracted_text
                    }

                    images.append(image_record)

    except ImportError as e:
        logger.warning(f"Required modules not available for image extraction: {e}")
    except Exception as e:
        logger.warning(f"Failed to extract images: {e}")

    return images
