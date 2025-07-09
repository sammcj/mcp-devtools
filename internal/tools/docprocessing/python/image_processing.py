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
                # Since images are now saved in the same directory as the markdown,
                # just use the filename
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
        picture_counter = 0

        # Try to extract images from document using different approaches

        # Method 1: Try to get images from document.pictures if available
        if hasattr(document, 'pictures') and document.pictures:
            for i, picture in enumerate(document.pictures):
                try:
                    picture_counter += 1

                    # Try to get the image data
                    pil_image = None
                    image_data = None

                    # Try different methods to get the image
                    if hasattr(picture, 'get_image'):
                        try:
                            pil_image = picture.get_image(document)
                        except Exception as e:
                            logger.debug(f"Failed to get image using get_image: {e}")

                    if not pil_image and hasattr(picture, 'image'):
                        pil_image = picture.image

                    if not pil_image and hasattr(picture, 'data'):
                        # Try to create PIL image from raw data
                        try:
                            if isinstance(picture.data, bytes):
                                pil_image = io.BytesIO(picture.data)
                                from PIL import Image
                                pil_image = Image.open(pil_image)
                        except Exception as e:
                            logger.debug(f"Failed to create PIL image from data: {e}")

                    if pil_image:
                        # Convert PIL image to base64
                        img_buffer = io.BytesIO()
                        pil_image.save(img_buffer, format='PNG')
                        img_buffer.seek(0)
                        image_data = base64.b64encode(img_buffer.getvalue()).decode('utf-8')

                        # Get image dimensions
                        width, height = pil_image.size

                        # Extract metadata
                        caption = getattr(picture, 'caption', '') or getattr(picture, 'text', '') or f"Picture {picture_counter}"
                        page_number = getattr(picture, 'page_number', None)

                        # Extract bounding box
                        bounding_box = None
                        if hasattr(picture, 'bbox') or hasattr(picture, 'bounding_box'):
                            bbox = getattr(picture, 'bbox', None) or getattr(picture, 'bounding_box', None)
                            if bbox:
                                bounding_box = {
                                    "x": getattr(bbox, 'x', 0.0),
                                    "y": getattr(bbox, 'y', 0.0),
                                    "width": getattr(bbox, 'width', 0.0),
                                    "height": getattr(bbox, 'height', 0.0)
                                }

                        # Try to extract text from the image using OCR
                        extracted_text = []
                        try:
                            extracted_text = extract_text_from_image(pil_image)
                        except Exception as e:
                            logger.debug(f"Failed to extract text from image: {e}")

                        # Save image to file
                        image_filename = f"picture_{picture_counter}.png"
                        image_file_path = save_image_to_file(image_data, image_filename, args)

                        # Create image record
                        image_record = {
                            "id": f"picture_{picture_counter}",
                            "type": "picture",
                            "caption": caption,
                            "alt_text": caption,
                            "format": "PNG",
                            "width": width,
                            "height": height,
                            "size": len(base64.b64decode(image_data)),
                            "file_path": image_file_path,
                            "page_number": page_number,
                            "bounding_box": bounding_box,
                            "extracted_text": extracted_text,
                            "description": f"Extracted image: {caption}" if caption else f"Extracted image {picture_counter}",
                            "recreation_prompt": generate_ai_recreation_prompt("picture", caption, extracted_text)[0] if extracted_text else ""
                        }

                        images.append(image_record)

                except Exception as e:
                    logger.warning(f"Failed to process picture {i}: {e}")
                    continue

        # Method 2: Try to extract from document elements if pictures not available
        elif hasattr(document, 'elements'):
            for element in document.elements:
                try:
                    # Look for image-like elements
                    if hasattr(element, 'type'):
                        element_type = str(element.type).lower()
                        if any(img_type in element_type for img_type in ['image', 'picture', 'figure', 'graphic']):
                            picture_counter += 1

                            # Try to extract image data from element
                            pil_image = None
                            if hasattr(element, 'get_image'):
                                try:
                                    pil_image = element.get_image(document)
                                except Exception as e:
                                    logger.debug(f"Failed to get image from element: {e}")

                            if pil_image:
                                # Convert PIL image to base64
                                img_buffer = io.BytesIO()
                                pil_image.save(img_buffer, format='PNG')
                                img_buffer.seek(0)
                                image_data = base64.b64encode(img_buffer.getvalue()).decode('utf-8')

                                # Get image dimensions
                                width, height = pil_image.size

                                # Extract metadata
                                caption = getattr(element, 'caption', '') or getattr(element, 'text', '') or f"Image {picture_counter}"
                                page_number = getattr(element, 'page_number', None)

                                # Save image to file
                                image_filename = f"image_{picture_counter}.png"
                                image_file_path = save_image_to_file(image_data, image_filename, args)

                                # Create image record
                                image_record = {
                                    "id": f"image_{picture_counter}",
                                    "type": "image",
                                    "caption": caption,
                                    "alt_text": caption,
                                    "format": "PNG",
                                    "width": width,
                                    "height": height,
                                    "size": len(base64.b64decode(image_data)),
                                    "file_path": image_file_path,
                                    "page_number": page_number,
                                    "description": f"Extracted image: {caption}" if caption else f"Extracted image {picture_counter}",
                                    "recreation_prompt": ""
                                }

                                images.append(image_record)

                except Exception as e:
                    logger.debug(f"Failed to process element: {e}")
                    continue

        # Method 3: If no images found, try using pdfimages as fallback
        if not images and args and hasattr(args, 'source'):
            try:
                images = extract_images_with_pdfimages(args.source, args)
            except Exception as e:
                logger.debug(f"pdfimages fallback failed: {e}")

        # Method 4: If still no images found, check if there are image placeholders in the markdown
        # This suggests images exist but we couldn't extract them directly
        if not images:
            try:
                markdown_content = document.export_to_markdown() if hasattr(document, 'export_to_markdown') else ""
                placeholder_count = markdown_content.count("<!-- image -->")

                if placeholder_count > 0:
                    logger.info(f"Found {placeholder_count} image placeholders but could not extract image data")
                    # Create placeholder records for the images we couldn't extract
                    for i in range(placeholder_count):
                        image_record = {
                            "id": f"placeholder_{i+1}",
                            "type": "placeholder",
                            "caption": f"Image {i+1} (could not extract)",
                            "alt_text": f"Image {i+1}",
                            "format": "unknown",
                            "width": 0,
                            "height": 0,
                            "size": 0,
                            "file_path": "",
                            "page_number": None,
                            "description": f"Image placeholder {i+1} - extraction failed",
                            "recreation_prompt": "Image data could not be extracted from the document."
                        }
                        images.append(image_record)
            except Exception as e:
                logger.debug(f"Failed to check for image placeholders: {e}")

        logger.info(f"Successfully extracted {len(images)} images from document")

    except Exception as e:
        logger.warning(f"Failed to extract images: {e}")

    return images


def extract_images_with_pdfimages(source_path: str, args=None) -> List[Dict[str, Any]]:
    """Extract images using pdfimages command line tool as fallback."""
    images = []

    try:
        import subprocess
        import tempfile
        import shutil
        from PIL import Image

        # Check if source is a local file (not URL)
        if source_path.startswith(('http://', 'https://')):
            logger.debug("pdfimages fallback not supported for URLs")
            return images

        # Check if pdfimages is available
        try:
            subprocess.run(['pdfimages', '--help'], capture_output=True, check=True)
        except (subprocess.CalledProcessError, FileNotFoundError):
            logger.debug("pdfimages command not available")
            return images

        # Create temporary directory for extracted images
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_prefix = os.path.join(temp_dir, 'extracted')

            # Run pdfimages to extract images
            try:
                subprocess.run([
                    'pdfimages', '-png', source_path, temp_prefix
                ], capture_output=True, check=True)
            except subprocess.CalledProcessError as e:
                logger.debug(f"pdfimages extraction failed: {e}")
                return images

            # Find all extracted image files
            extracted_files = []
            for filename in os.listdir(temp_dir):
                if filename.startswith('extracted') and filename.endswith('.png'):
                    extracted_files.append(os.path.join(temp_dir, filename))

            # Sort files to maintain order
            extracted_files.sort()

            # Process each extracted image
            for i, image_path in enumerate(extracted_files):
                try:
                    # Load image to get dimensions and convert to base64
                    with Image.open(image_path) as pil_image:
                        width, height = pil_image.size

                        # Convert to base64
                        img_buffer = io.BytesIO()
                        pil_image.save(img_buffer, format='PNG')
                        img_buffer.seek(0)
                        image_data = base64.b64encode(img_buffer.getvalue()).decode('utf-8')

                        # Try to extract text from image using OCR
                        extracted_text = []
                        try:
                            extracted_text = extract_text_from_image(pil_image)
                        except Exception as e:
                            logger.debug(f"Failed to extract text from pdfimages image: {e}")

                        # Save image to final location
                        image_filename = f"picture_{i+1}.png"
                        image_file_path = save_image_to_file(image_data, image_filename, args)

                        # Determine page number (pdfimages doesn't provide this directly)
                        # We'll estimate based on image order
                        estimated_page = (i // 2) + 1  # Rough estimate assuming ~2 images per page

                        # Create image record
                        image_record = {
                            "id": f"extracted_image_{i+1}",
                            "type": "extracted",
                            "caption": f"Extracted Image {i+1}",
                            "alt_text": f"Extracted Image {i+1}",
                            "format": "PNG",
                            "width": width,
                            "height": height,
                            "size": len(base64.b64decode(image_data)),
                            "file_path": image_file_path,
                            "page_number": estimated_page,
                            "bounding_box": None,
                            "extracted_text": extracted_text,
                            "description": f"Image extracted using pdfimages fallback method",
                            "recreation_prompt": generate_ai_recreation_prompt("extracted", f"Extracted Image {i+1}", extracted_text)[0] if extracted_text else ""
                        }

                        images.append(image_record)

                except Exception as e:
                    logger.warning(f"Failed to process extracted image {image_path}: {e}")
                    continue

        logger.info(f"pdfimages fallback extracted {len(images)} images")

    except Exception as e:
        logger.warning(f"pdfimages fallback extraction failed: {e}")

    return images


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
