#!/usr/bin/env python3
"""
Table Processing Module for Docling Document Processing

This module handles table extraction and formatting.
"""

from typing import List, Dict, Any
import logging
import pandas as pd

logger = logging.getLogger(__name__)


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

                try:
                    # Use docling's built-in pandas export for better accuracy
                    df = table.export_to_dataframe()

                    if not df.empty:
                        headers = df.columns.tolist()
                        rows = df.values.tolist()

                        table_data["headers"] = headers
                        table_data["rows"] = rows

                        # Generate export formats from the dataframe
                        table_data["markdown"] = df.to_markdown(index=False)
                        table_data["csv"] = df.to_csv(index=False)
                        # Pass caption to to_html if it exists
                        caption = table_data.get("caption")
                        table_data["html"] = df.to_html(index=False, caption=caption if caption else None)

                except Exception as e:
                    logger.warning(f"Could not export table {i+1} to dataframe, falling back to manual extraction. Error: {e}")
                    # Fallback to original logic if export_to_dataframe fails
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
