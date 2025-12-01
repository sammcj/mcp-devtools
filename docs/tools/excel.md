# Excel Tool

Excel (xlsx) file manipulation tool providing workbook, worksheet, data, formatting, chart, pivot table, formula, and validation operations.

## Configuration

**Disabled by default** - requires explicit enablement:

```json
{
  "mcpServers": {
    "dev-tools": {
      "env": {
        "ENABLE_ADDITIONAL_TOOLS": "excel",
        "EXCEL_FILES_PATH": "/path/to/excel/files"
      }
    }
  }
}
```

### Environment Variables

- `ENABLE_ADDITIONAL_TOOLS` - Must include `excel` to enable this tool
- `EXCEL_FILES_PATH` - (Only required when running the server is running in HTTP mode) Allows you to set a base directory for Excel files in HTTP mode (default: `~/.mcp-devtools/excel/`)

### Transport Modes

- **STDIO mode**: Use absolute file paths
- **HTTP mode**: Use relative paths from `EXCEL_FILES_PATH`

## Functions

### Workbook Operations

#### `create_workbook`
Create a new Excel workbook.

**Parameters:**
- `filepath` (required): Path to Excel file
- `options.initial_sheet_name` (optional): Initial worksheet name (default: "Sheet1")

**Example:**
```json
{
  "function": "create_workbook",
  "filepath": "/path/to/workbook.xlsx",
  "options": {
    "initial_sheet_name": "Data"
  }
}
```

#### `get_workbook_metadata`
Retrieve workbook information including sheet names, file size, and data ranges.

**Parameters:**
- `filepath` (required): Path to Excel file
- `options.include_ranges` (optional): Include data ranges for each sheet (default: false)

### Worksheet Management

#### `create_worksheet`
Add a new worksheet to the workbook.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Name for new worksheet

#### `copy_worksheet`
Clone an existing worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Source worksheet name
- `options.target_name` (required): Name for copied worksheet

#### `delete_worksheet`
Remove a worksheet from the workbook.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name to delete

**Note:** Cannot delete the last remaining worksheet.

#### `rename_worksheet`
Rename an existing worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Current worksheet name
- `options.new_name` (required): New worksheet name

### Data Operations

#### `read_data`
Read data from a cell range.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_cell` (optional): Starting cell (e.g., "A1")
- `options.end_cell` (optional): Ending cell (e.g., "D10")

**Example:**
```json
{
  "function": "read_data",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "start_cell": "A1",
    "end_cell": "C10"
  }
}
```

#### `read_all_data`
Export all data from one or more sheets in AI-agent-friendly format (CSV, TSV, or JSON).

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (optional): Single sheet to read
- `options.sheet_names` (optional): Array of sheet names to read
- `options.format` (optional): Output format - `"csv"` (default, token-optimised), `"tsv"`, or `"json"`
- `options.max_rows` (optional): Limit rows per sheet to prevent token overflow
- `options.offset` (optional): Skip first N rows before reading (for pagination, default: 0)

**Note:** If neither `sheet_name` nor `options.sheet_names` is specified, reads all sheets. All rows are padded to the same length with empty strings for consistency.

**Example - Read all sheets as CSV:**
```json
{
  "function": "read_all_data",
  "filepath": "/path/to/workbook.xlsx",
  "options": {
    "format": "csv"
  }
}
```

**Example - Read specific sheets with row limit:**
```json
{
  "function": "read_all_data",
  "filepath": "/path/to/large-report.xlsx",
  "options": {
    "sheet_names": ["Sales", "Expenses"],
    "format": "tsv",
    "max_rows": 100
  }
}
```

**Response format:**
```json
{
  "sheets": [
    {
      "sheet_name": "Sales",
      "format": "csv",
      "data": "Month,Revenue,Tax\nJan,5000,1000\nFeb,6500,1300",
      "dimensions": {
        "total_rows": 3,
        "returned_rows": 3,
        "start_row": 1,
        "end_row": 3,
        "remaining_rows": 0,
        "columns": 3
      }
    }
  ]
}
```

#### `write_data`
Write data to cells. Formulas can be included directly in the data array.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_cell` (required): Starting cell for data
- `options.data` (required): 2D array of data to write

**Formula Support:** Any string value starting with `=` is automatically treated as a formula. Formulas are validated for safety and calculated for Apple Numbers compatibility.

**Example with formulas:**
```json
{
  "function": "write_data",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "start_cell": "A1",
    "data": [
      ["Item", "Price", "Quantity", "Total"],
      ["Apples", 1.50, 10, "=B2*C2"],
      ["Oranges", 2.00, 5, "=B3*C3"],
      ["", "", "Grand Total:", "=SUM(D2:D3)"]
    ]
  }
}
```

**Basic example:**
```json
{
  "function": "write_data",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "start_cell": "A1",
    "data": [
      ["Name", "Age", "Salary"],
      ["Alice", 30, 75000],
      ["Bob", 25, 65000]
    ]
  }
}
```

#### `read_data_with_metadata`
Read data with validation rules and metadata.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_cell` (optional): Starting cell
- `options.end_cell` (optional): Ending cell

Returns cell data with validation information including dropdown lists and validation rules.

### Formatting

#### `format_range`
Apply formatting to cell ranges.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.range` (required): Cell range (e.g., "A1:D10")
- `options.font` (optional): Font properties (bold, italic, size, colour, family)
- `options.fill` (optional): Fill properties (colour, pattern)
- `options.borders` (optional): Border properties (style, colour, sides)
- `options.alignment` (optional): Alignment properties (horizontal, vertical, wrap_text)
- `options.number_format` (optional): Number format string (e.g., "£#,##0.00")
- `options.conditional_format` (optional): Conditional formatting rules

**Style Merging:** When applying formatting to cells that already have styles, the new properties are merged with existing ones. This means you can apply fill and font in one call, then add borders in another call, and both will be preserved.

**Colour Format:** Both `"#4472C4"` and `"4472C4"` formats are accepted for colours (the `#` prefix is optional and will be stripped automatically).

**Font Example:**
```json
{
  "function": "format_range",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "range": "A1:D1",
    "font": {
      "bold": true,
      "size": 14,
      "colour": "#FF0000",
      "family": "Arial"
    },
    "fill": {
      "colour": "#FFFF00",
      "pattern": "solid"
    }
  }
}
```

**Conditional Formatting Example:**
```json
{
  "function": "format_range",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "range": "B2:B10",
    "conditional_format": {
      "type": "colour_scale",
      "rule": {
        "min_colour": "#FF0000",
        "max_colour": "#00FF00"
      }
    }
  }
}
```

### Cell Operations

#### `merge_cells`
Merge a cell range.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_cell` (required): Starting cell
- `options.end_cell` (required): Ending cell

#### `unmerge_cells`
Unmerge a cell range.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_cell` (required): Starting cell
- `options.end_cell` (required): Ending cell

#### `get_merged_cells`
List all merged cell ranges in a worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name

### Range Operations

#### `copy_range`
Copy cells to another location.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Source worksheet name
- `options.source_range` (required): Source range (e.g., "A1:C10")
- `options.target_cell` (required): Target starting cell
- `options.target_sheet` (optional): Target worksheet name (defaults to source sheet)

#### `delete_range`
Delete a range and shift cells.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_cell` (required): Starting cell
- `options.end_cell` (required): Ending cell
- `options.shift_direction` (optional): "up" or "left" (default: "up")

#### `validate_range`
Validate that a range exists.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_cell` (required): Starting cell
- `options.end_cell` (optional): Ending cell

### Row and Column Operations

#### `insert_rows`
Insert rows into a worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_row` (required): Starting row number (1-based)
- `options.count` (optional): Number of rows to insert (default: 1)

#### `insert_columns`
Insert columns into a worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_col` (required): Starting column number (1-based)
- `options.count` (optional): Number of columns to insert (default: 1)

#### `delete_rows`
Delete rows from a worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_row` (required): Starting row number (1-based)
- `options.count` (optional): Number of rows to delete (default: 1)

#### `delete_columns`
Delete columns from a worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.start_col` (required): Starting column number (1-based)
- `options.count` (optional): Number of columns to delete (default: 1)

### Charts

#### `create_chart`
Create a chart in the worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.type` (required): Chart type: "line", "bar", "column", "pie", "scatter", "area"
- `options.position` (required): Cell position for chart (e.g., "E2")
- `options.title` (optional): Chart title
- `options.x_axis_title` (optional): X-axis title
- `options.y_axis_title` (optional): Y-axis title
- `options.data_range` (optional): Simple data range
- `options.series` (optional): Detailed data series configuration
- `options.legend` (optional): Legend configuration
- `options.size` (optional): Chart dimensions (width, height)

**Simple Example:**
```json
{
  "function": "create_chart",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "type": "column",
    "position": "E2",
    "title": "Sales by Quarter",
    "data_range": "A1:C10"
  }
}
```

**Advanced Example:**
```json
{
  "function": "create_chart",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "type": "line",
    "position": "E2",
    "title": "Sales Trends",
    "x_axis_title": "Quarter",
    "y_axis_title": "Revenue (£)",
    "series": [
      {
        "name": "Product A",
        "categories": "A2:A10",
        "values": "B2:B10"
      },
      {
        "name": "Product B",
        "categories": "A2:A10",
        "values": "C2:C10"
      }
    ],
    "legend": {
      "show": true,
      "position": "bottom"
    },
    "size": {
      "width": 640,
      "height": 480
    }
  }
}
```

### Pivot Tables

#### `create_pivot_table`
Create a native Excel pivot table.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Source worksheet name
- `options.source_range` (required): Source data range (e.g., "A1:D100")
- `options.row_fields` (required): Array of row field names
- `options.data_fields` (required): Array of data field configurations
- `options.column_fields` (optional): Array of column field names
- `options.filter_fields` (optional): Array of filter field names
- `options.destination` (optional): Destination sheet and cell (default: new sheet "Pivot1" at A1)
- `options.options` (optional): Pivot table options (show_grand_totals, style)

**Example:**
```json
{
  "function": "create_pivot_table",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "source_range": "A1:D100",
    "row_fields": ["Category", "Product"],
    "data_fields": [
      {
        "field": "Sales",
        "function": "sum",
        "name": "Total Sales"
      },
      {
        "field": "Units",
        "function": "count",
        "name": "Units Sold"
      }
    ],
    "column_fields": ["Quarter"],
    "filter_fields": ["Region"],
    "destination": {
      "sheet": "Pivot Analysis",
      "cell": "A1"
    },
    "options": {
      "show_grand_totals": true,
      "style": "PivotStyleMedium9"
    }
  }
}
```

**Aggregation Functions:** sum, count, average, min, max, product, stddev, var

### Excel Tables

#### `create_table`
Create a native Excel table object.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.range` (required): Table range (e.g., "A1:D100")
- `options.name` (optional): Table name (auto-generated if not provided)
- `options.style` (optional): Table style name (default: "TableStyleMedium9")
- `options.show_header` (optional): Show header row (default: true)
- `options.show_totals` (optional): Show totals row (default: false)

**Example:**
```json
{
  "function": "create_table",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "range": "A1:D100",
    "name": "SalesTable",
    "style": "TableStyleMedium9",
    "show_header": true,
    "show_totals": false
  }
}
```

### Formulas

#### `apply_formula`
Write a formula to a cell. Note that `write_data` also supports formulas inline - use this function when you need to add formulas to existing data or update individual cells.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.cell` (required): Cell reference (e.g., "C1")
- `options.formula` (required): Excel formula (with or without leading "=")

**Example:**
```json
{
  "function": "apply_formula",
  "filepath": "/path/to/workbook.xlsx",
  "sheet_name": "Sheet1",
  "options": {
    "cell": "D2",
    "formula": "=SUM(B2:C2)"
  }
}
```

**Note:** Formulas are automatically calculated and cached for compatibility with Apple Numbers and other spreadsheet applications that don't have full formula calculation engines.

**Security:** Dangerous functions (INDIRECT, HYPERLINK, WEBSERVICE, DGET, RTD) are blocked for security reasons.

#### `validate_formula_syntax`
Validate formula syntax without applying it.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name
- `options.cell` (required): Cell reference
- `options.formula` (required): Excel formula to validate

### Data Validation

#### `get_data_validation_info`
Extract all data validation rules from a worksheet.

**Parameters:**
- `filepath` (required): Path to Excel file
- `sheet_name` (required): Worksheet name

Returns validation rules including type, operators, allowed values, prompts, and error messages.

## Common Patterns

### Create and Populate a Workbook
1. `create_workbook` - Create the workbook
2. `write_data` - Add data (can include formulas directly)
3. `format_range` - Apply formatting
4. `create_chart` - Add visualisations

### Build a Report
1. `write_data` - Write raw data
2. `create_table` - Convert to Excel table
3. `create_pivot_table` - Create analysis pivot table
4. `create_chart` - Add charts for visualisation

### Data Analysis Workflow
1. `read_data` - Load data from source
2. `write_data` - Write to analysis sheet (can include formulas directly in data)
3. `apply_formula` - Add additional calculations (optional if formulas included in step 2)
4. `format_range` - Apply conditional formatting
5. `create_pivot_table` - Summarise data

## Error Handling

### Common Errors

**Tool Not Enabled:**
```
excel tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS=excel to enable
```
Solution: Add `excel` to `ENABLE_ADDITIONAL_TOOLS` environment variable.

**File Not Found:**
```
workbook error during open on /path/to/file.xlsx: failed to open workbook
```
Solution: Verify file path is correct and file exists.

**Worksheet Not Found:**
```
worksheet error during operation on sheet 'SheetName': worksheet not found
```
Solution: Check worksheet name spelling and case sensitivity.

**Invalid Cell Reference:**
```
validation error for field 'cell' with value 'XYZ': invalid cell reference
```
Solution: Use valid Excel cell references (e.g., "A1", "B10").

**Chart Type Error:**
```
invalid chart type 'xyz', must be one of: line, bar, column, pie, scatter, area
```
Solution: Use a supported chart type.

### Excel Limits

- Maximum rows: 1,048,576
- Maximum columns: 16,384 (column XFD)
- Maximum cell value length: 32,767 characters
- Maximum worksheet name length: 31 characters
- Worksheet name restrictions: Cannot contain `:\/?*[]`

## Performance Considerations

### Large Files
- Reading 10,000 rows: ~2 seconds
- Writing 10,000 rows: ~3 seconds
- Formatting 1,000 cells: ~1 second
- Creating chart with 100 data points: ~1 second

### Best Practices
- Process data in batches for large datasets
- Minimise file open/close operations by grouping operations
- Use absolute file paths in STDIO mode
- Apply formatting after data write operations
- Limit conditional formatting rules for better performance

## Security

### File Access
- Integrated with MCP DevTools security framework
- Respects `FILESYSTEM_TOOL_ALLOWED_DIRS` configuration
- Directory traversal prevention in HTTP mode
- Files created with 0600 permissions

### Formula Safety
- Dangerous functions blocked: INDIRECT, HYPERLINK, WEBSERVICE, DGET, RTD
- Formula validation before execution
- Clear error messages for blocked functions

## Compatibility

Generated Excel files are compatible with:
- Microsoft Excel 2016+
- LibreOffice Calc
- Google Sheets (with some limitations on advanced features)

Excel features fully supported:
- Workbook and worksheet operations
- Data types: string, number, boolean, date, formula
- Cell formatting and conditional formatting
- Charts (line, bar, column, pie, scatter, area)
- Native pivot tables
- Excel tables with styling
- Data validation rules
- Formulas (with security restrictions)
