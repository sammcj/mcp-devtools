package excel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// ExcelTool implements Excel file (xlsx) manipulation
type ExcelTool struct{}

// Configuration
var excelBasePath string

// init registers the Excel tool and initialises configuration
func init() {
	registry.Register(&ExcelTool{})

	// Initialise base path from environment or default
	excelBasePath = os.Getenv("EXCEL_FILES_PATH")
	if excelBasePath == "" {
		homeDir, _ := os.UserHomeDir()
		excelBasePath = filepath.Join(homeDir, ".mcp-devtools", "excel")
	}
}

// Definition returns the tool's definition for MCP registration
func (t *ExcelTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"excel",
		mcp.WithDescription(`Excel (.xlsx) manipulation: create/edit workbooks, sheets, data, formulas, charts, pivot tables, formatting, and validation. Supports many operations from simple data writes to complex formatted tables. Use this tool when creating or editing Excel spreadsheets.

EFFICIENT WORKFLOW - Use create_table for formatted tables in one call:
  create_table: range="A1:B6", data=[["Breed","Count"],["Lab",100],...], style="TableStyleMedium9", auto_size=true
  → Writes data, applies professional styling, and auto-sizes columns in one operation
  → More efficient than separate write_data + format_range + auto_size_columns calls

Other key workflows: write_data (auto-detects formulas starting with '='), format_range (merges with existing styles), create_chart/pivot_table.

Functions: create_workbook (supports initial_sheets for multi-sheet creation), create_worksheet, read/write_data, format_range, create_table, create_chart, create_pivot_table, formulas, validation, row/column ops, and more.

Use get_tool_help tool with tool_name="excel" for detailed examples, troubleshooting, and parameter reference.`),
		mcp.WithString("function",
			mcp.Required(),
			mcp.Description("Operation to perform. For formatted tables, use create_table (all-in-one). For data with formulas, use write_data. For styling, use format_range."),
			mcp.Enum(
				// Workbook operations
				"create_workbook", "get_workbook_metadata", "create_worksheet",
				// Data operations
				"read_data", "write_data", "read_data_with_metadata",
				// Worksheet management
				"copy_worksheet", "delete_worksheet", "rename_worksheet",
				// Formatting
				"format_range",
				// Cell operations
				"merge_cells", "unmerge_cells", "get_merged_cells",
				// Range operations
				"copy_range", "delete_range", "validate_range",
				// Row/Column operations
				"insert_rows", "insert_columns", "delete_rows", "delete_columns", "auto_size_columns",
				// Charts
				"create_chart",
				// Pivot tables and tables
				"create_pivot_table", "create_table",
				// Formulas
				"apply_formula", "validate_formula_syntax",
				// Data validation
				"get_data_validation_info",
			),
		),
		mcp.WithString("filepath",
			mcp.Required(),
			mcp.Description("Path to xlsx file"),
		),
		mcp.WithString("sheet_name",
			mcp.Description("Worksheet name (required for most operations except create_workbook)"),
		),
		mcp.WithObject("options",
			mcp.Description("Function-specific options and parameters"),
			mcp.Properties(map[string]any{
				// Common data operation parameters
				"start_cell": map[string]any{
					"type":        "string",
					"description": "Starting cell reference (e.g., 'A1')",
				},
				"end_cell": map[string]any{
					"type":        "string",
					"description": "Ending cell reference (e.g., 'D10')",
				},
				"data": map[string]any{
					"type":        "array",
					"description": "2D array of data to write. Formulas auto-detected when starting with '='. Example: [['Month','Sales','Tax'],['Jan',5000,'=B2*0.2']]",
				},
				"range": map[string]any{
					"type":        "string",
					"description": "Cell range in A1 notation (e.g., 'A1:D10'). For create_table, defines table area including headers.",
				},
				// Workbook parameters
				"initial_sheet_name": map[string]any{
					"type":        "string",
					"description": "Initial worksheet name when creating workbook (single sheet)",
				},
				"initial_sheets": map[string]any{
					"type":        "array",
					"description": "Array of sheet names for create_workbook. Example: ['Dogs','Cats','Birds'] More efficient than creating workbook then adding sheets individually.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"include_ranges": map[string]any{
					"type":        "boolean",
					"description": "Include data ranges in metadata",
					"default":     false,
				},
				// Worksheet parameters
				"target_name": map[string]any{
					"type":        "string",
					"description": "Target worksheet name for copy operations",
				},
				"new_name": map[string]any{
					"type":        "string",
					"description": "New name for rename operations",
				},
				// Row/column parameters
				"start_row": map[string]any{
					"type":        "number",
					"description": "Starting row number (1-based)",
				},
				"start_col": map[string]any{
					"type":        "number",
					"description": "Starting column number (1-based)",
				},
				"count": map[string]any{
					"type":        "number",
					"description": "Number of rows/columns",
					"default":     1,
				},
				// Range operation parameters
				"source_range": map[string]any{
					"type":        "string",
					"description": "Source range for copy operations",
				},
				"target_cell": map[string]any{
					"type":        "string",
					"description": "Target cell for copy operations",
				},
				"target_sheet": map[string]any{
					"type":        "string",
					"description": "Target worksheet name for copy operations",
				},
				"shift_direction": map[string]any{
					"type":        "string",
					"description": "Direction to shift cells ('up' or 'left')",
					"enum":        []string{"up", "left"},
					"default":     "up",
				},
				// Formatting parameters
				"font": map[string]any{
					"type":        "object",
					"description": "Font properties for format_range. Example: {bold: true, size: 12, colour: 'FF0000'}.",
				},
				"fill": map[string]any{
					"type":        "object",
					"description": "Fill properties for format_range. Example: {colour: 'E2EFDA', pattern: 'solid'}.",
				},
				"borders": map[string]any{
					"type":        "object",
					"description": "Border properties for format_range. Example: {style: 'thin', colour: '000000', sides: ['top','bottom']}. Defaults to all slides.",
				},
				"alignment": map[string]any{
					"type":        "object",
					"description": "Alignment properties (horizontal, vertical, wrap_text)",
				},
				"number_format": map[string]any{
					"type":        "string",
					"description": "Number format string. Examples: '#,##0.00' (thousands), '£#,##0.00' (currency), '0.00%' (percentage), 'dd/mm/yyyy' (date)",
				},
				"conditional_format": map[string]any{
					"type":        "object",
					"description": "Conditional formatting rules",
				},
				// Chart parameters
				"type": map[string]any{
					"type":        "string",
					"description": "Chart type (line, bar, column, pie, scatter, area)",
				},
				"data_range": map[string]any{
					"type":        "string",
					"description": "Data range for charts",
				},
				"position": map[string]any{
					"type":        "string",
					"description": "Cell position for chart",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Chart or axis title",
				},
				"series": map[string]any{
					"type":        "array",
					"description": "Chart data series configuration",
				},
				// Pivot table parameters
				"row_fields": map[string]any{
					"type":        "array",
					"description": "Row fields for pivot table",
				},
				"column_fields": map[string]any{
					"type":        "array",
					"description": "Column fields for pivot table",
				},
				"data_fields": map[string]any{
					"type":        "array",
					"description": "Data fields for pivot table",
				},
				// Table parameters
				"name": map[string]any{
					"type":        "string",
					"description": "Table name for create_table. If omitted, auto-generates from sheet name (e.g., 'Dogs' sheet → 'DogsTable'). Specify unique names when creating multiple tables.",
				},
				"style": map[string]any{
					"type":        "string",
					"description": "Table style name for create_table. Examples: 'TableStyleMedium9' (blue), 'TableStyleLight2' (minimal), 'TableStyleDark1' (dark).",
				},
				"auto_size": map[string]any{
					"type":        "boolean",
					"description": "Set column widths to fit content (omit unless false)",
					"default":     true,
				},
				// Formula parameters
				"cell": map[string]any{
					"type":        "string",
					"description": "Cell reference for formula",
				},
				"formula": map[string]any{
					"type":        "string",
					"description": "Excel formula (must start with '=')",
				},
			}),
		),
		// Tool annotations
		mcp.WithReadOnlyHintAnnotation(false),   // Can modify Excel files
		mcp.WithDestructiveHintAnnotation(true), // Can delete worksheets, ranges, etc.
		mcp.WithIdempotentHintAnnotation(false), // Operations are generally not idempotent
		mcp.WithOpenWorldHintAnnotation(false),  // No external network calls (local file operations only)
	)
}

// Execute executes the Excel tool
func (t *ExcelTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check tool enablement first
	if !tools.IsToolEnabled("excel") {
		return nil, fmt.Errorf("excel tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS=excel to enable") // this should never occur
	}

	// Extract common parameters
	function, ok := args["function"].(string)
	if !ok || function == "" {
		return nil, &ValidationError{Field: "function", Value: args["function"], Message: "function parameter is required"}
	}

	filepath, ok := args["filepath"].(string)
	if !ok || filepath == "" {
		return nil, &ValidationError{Field: "filepath", Value: args["filepath"], Message: "filepath parameter is required"}
	}

	// Resolve and validate filepath
	fullPath, err := resolveExcelPath(filepath)
	if err != nil {
		return nil, err
	}

	// Security integration: check file access
	if err := security.CheckFileAccess(fullPath); err != nil {
		return nil, fmt.Errorf("file access denied: %w", err)
	}

	// Extract sheet_name if provided
	sheetName, _ := args["sheet_name"].(string)

	// Extract options object
	options, _ := args["options"].(map[string]any)
	if options == nil {
		options = make(map[string]any)
	}

	logger.WithFields(logrus.Fields{
		"function":   function,
		"filepath":   fullPath,
		"sheet_name": sheetName,
	}).Info("Executing Excel operation")

	// Dispatch to appropriate handler
	switch function {
	case "create_workbook":
		return handleCreateWorkbook(ctx, logger, fullPath, options)
	case "get_workbook_metadata":
		return handleGetWorkbookMetadata(ctx, logger, fullPath, options)
	case "create_worksheet":
		return handleCreateWorksheet(ctx, logger, fullPath, sheetName)
	case "read_data":
		return handleReadData(ctx, logger, fullPath, sheetName, options)
	case "write_data":
		return handleWriteData(ctx, logger, fullPath, sheetName, options)
	case "read_data_with_metadata":
		return handleReadDataWithMetadata(ctx, logger, fullPath, sheetName, options)
	case "copy_worksheet":
		return handleCopyWorksheet(ctx, logger, fullPath, sheetName, options)
	case "delete_worksheet":
		return handleDeleteWorksheet(ctx, logger, fullPath, sheetName)
	case "rename_worksheet":
		return handleRenameWorksheet(ctx, logger, fullPath, sheetName, options)
	case "format_range":
		return handleFormatRange(ctx, logger, fullPath, sheetName, options)
	case "merge_cells":
		return handleMergeCells(ctx, logger, fullPath, sheetName, options)
	case "unmerge_cells":
		return handleUnmergeCells(ctx, logger, fullPath, sheetName, options)
	case "get_merged_cells":
		return handleGetMergedCells(ctx, logger, fullPath, sheetName)
	case "copy_range":
		return handleCopyRange(ctx, logger, fullPath, sheetName, options)
	case "delete_range":
		return handleDeleteRange(ctx, logger, fullPath, sheetName, options)
	case "validate_range":
		return handleValidateRange(ctx, logger, fullPath, sheetName, options)
	case "insert_rows":
		return handleInsertRows(ctx, logger, fullPath, sheetName, options)
	case "insert_columns":
		return handleInsertColumns(ctx, logger, fullPath, sheetName, options)
	case "delete_rows":
		return handleDeleteRows(ctx, logger, fullPath, sheetName, options)
	case "delete_columns":
		return handleDeleteColumns(ctx, logger, fullPath, sheetName, options)
	case "auto_size_columns":
		return handleAutoSizeColumns(ctx, logger, fullPath, sheetName, options)
	case "create_chart":
		return handleCreateChart(ctx, logger, fullPath, sheetName, options)
	case "create_pivot_table":
		return handleCreatePivotTable(ctx, logger, fullPath, sheetName, options)
	case "create_table":
		return handleCreateTable(ctx, logger, fullPath, sheetName, options)
	case "apply_formula":
		return handleApplyFormula(ctx, logger, fullPath, sheetName, options)
	case "validate_formula_syntax":
		return handleValidateFormulaSyntax(ctx, logger, options)
	case "get_data_validation_info":
		return handleGetDataValidationInfo(ctx, logger, fullPath, sheetName)
	default:
		return nil, fmt.Errorf("unknown function: %s", function)
	}
}

// resolveExcelPath resolves file paths based on transport mode
func resolveExcelPath(filePath string) (string, error) {
	if filePath == "" {
		return "", &ValidationError{Field: "filepath", Value: filePath, Message: "filepath parameter is required"}
	}

	// If absolute path, use directly (stdio mode)
	if filepath.IsAbs(filePath) {
		return filePath, nil
	}

	// Relative path: resolve from base directory (HTTP mode)
	// Security: prevent directory traversal
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		return "", &ValidationError{Field: "filepath", Value: filePath, Message: "directory traversal not allowed"}
	}

	fullPath := filepath.Join(excelBasePath, cleanPath)

	// Ensure directory exists for write operations
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	return fullPath, nil
}

// ProvideExtendedInfo provides detailed usage information for the Excel tool
func (t *ExcelTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Create formatted table with data in one call",
				Arguments: map[string]any{
					"function":   "create_table",
					"filepath":   "/path/to/report.xlsx",
					"sheet_name": "Sales",
					"options": map[string]any{
						"range": "A1:C6",
						"data": [][]any{
							{"Product", "Region", "Revenue"},
							{"Widget", "North", 15000},
							{"Gadget", "South", 22000},
							{"Widget", "East", 18000},
							{"Gadget", "West", 25000},
						},
						"style":     "TableStyleMedium9",
						"auto_size": true,
					},
				},
				ExpectedResult: "Creates table with data, applies style, and auto-sizes columns in one operation. More efficient than separate write_data, format_range, and auto_size_columns calls.",
			},
			{
				Description: "Write data with formulas",
				Arguments: map[string]any{
					"function":   "write_data",
					"filepath":   "/path/to/calc.xlsx",
					"sheet_name": "Summary",
					"options": map[string]any{
						"start_cell": "A1",
						"data": [][]any{
							{"Month", "Sales", "Tax", "Total"},
							{"Jan", 5000, "=B2*0.2", "=B2+C2"},
							{"Feb", 6500, "=B3*0.2", "=B3+C3"},
							{"Total", "=SUM(B2:B3)", "=SUM(C2:C3)", "=SUM(D2:D3)"},
						},
					},
				},
				ExpectedResult: "Writes data with formulas auto-detected (values starting with '='). Formulas are calculated and cached.",
			},
			{
				Description: "Apply number formatting to currency column",
				Arguments: map[string]any{
					"function":   "format_range",
					"filepath":   "/path/to/report.xlsx",
					"sheet_name": "Sales",
					"options": map[string]any{
						"range":         "C2:C100",
						"number_format": "£#,##0.00",
					},
				},
				ExpectedResult: "Applies currency formatting with thousands separator. Merges with existing cell formatting.",
			},
			{
				Description: "Create chart from data range",
				Arguments: map[string]any{
					"function":   "create_chart",
					"filepath":   "/path/to/report.xlsx",
					"sheet_name": "Sales",
					"options": map[string]any{
						"type":       "column",
						"data_range": "A1:B6",
						"position":   "E2",
						"title":      "Regional Sales",
					},
				},
				ExpectedResult: "Creates column chart positioned at E2 showing data from A1:B6.",
			},
			{
				Description: "Cross-sheet formula reference",
				Arguments: map[string]any{
					"function":   "write_data",
					"filepath":   "/path/to/multi.xlsx",
					"sheet_name": "Summary",
					"options": map[string]any{
						"start_cell": "A1",
						"data": [][]any{
							{"Total from Sheet1", "=Sheet1!B10"},
							{"Total from Sheet2", "=Sheet2!B10"},
							{"Grand Total", "=SUM(B1:B2)"},
						},
					},
				},
				ExpectedResult: "Creates formulas referencing other sheets using 'SheetName!CellRef' syntax.",
			},
		},
		CommonPatterns: []string{
			"For simple formatted tables: Use create_table with options.data, options.style, and options.auto_size=true for all-in-one creation",
			"Table naming: Names auto-generate from sheet names ('Dogs' → 'DogsTable'). One table per sheet works automatically; multiple tables need explicit names.",
			"Formula auto-detection: Any cell value starting with '=' is automatically treated as a formula in write_data and create_table",
			"Cross-sheet formulas: Use 'SheetName!A1' syntax in formula strings to reference other worksheets",
			"Style merging: format_range merges new formatting with existing cell styles rather than replacing them",
			"Number formatting: Use options.number_format for currency ('£#,##0.00'), percentages ('0.00%'), or custom formats",
			"Efficient workflows: Prefer create_table over separate write_data + format_range + auto_size_columns calls",
			"Range validation: Use validate_range before using ranges in formulas to catch errors early",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Formula returns #REF! or #NAME? error",
				Solution: "Check cell references are valid and within Excel limits (max row: 1048576, max col: 16384 / XFD). Use validate_range to verify ranges before using in formulas. Ensure sheet names in cross-sheet formulas match exactly.",
			},
			{
				Problem:  "Need to apply different formatting to header, data, and total rows",
				Solution: "This requires multiple format_range calls - one for each section. Each call merges with existing formatting, so you can build up complex styles incrementally (e.g., first call adds borders to all cells, second call adds bold to headers).",
			},
			{
				Problem:  "Table style not applied or columns not auto-sized",
				Solution: "Ensure you're using create_table (not write_data) and that options.style and options.auto_size are set. Default auto_size is true. Check table name doesn't start with a digit or contain spaces.",
			},
			{
				Problem:  "Error: 'table name already exists' when creating multiple tables",
				Solution: "Table names auto-generate from sheet names (e.g., 'Dogs' → 'DogsTable'). When creating multiple tables on the same sheet, specify unique names: options.name='CustomName1', options.name='CustomName2'.",
			},
			{
				Problem:  "Formulas show as literal text instead of calculating",
				Solution: "Ensure formula strings start with '=' character. Formulas are auto-detected in write_data and create_table. Very long formulas (>8192 chars) or those with unsafe functions are written as literal text with a warning.",
			},
			{
				Problem:  "Cannot create multiple sheets at workbook creation",
				Solution: "Use options.initial_sheets array to create multiple sheets at once, or use create_worksheet after create_workbook for individual sheet creation.",
			},
		},
		ParameterDetails: map[string]string{
			"function":                  "Operation to perform. Key workflows: create_table (all-in-one), write_data (supports formulas), format_range (styling), create_chart/pivot_table (visualisation).",
			"options.data":              "2D array for write_data or create_table. Values starting with '=' are auto-detected as formulas. Example: [['Total', '=SUM(B2:B5)'], ['Tax', '=B6*0.2']]",
			"options.number_format":     "Excel number format string. Examples: '#,##0.00' (thousands separator), '£#,##0.00' (currency), '0.00%' (percentage), 'dd/mm/yyyy' (date).",
			"options.range":             "Cell range in A1 notation (e.g., 'A1:D10'). Required for format_range, create_table, and many operations. Use validate_range to check validity.",
			"create_table.options":      "Combine data, style, name, and auto_size for efficient table creation. options.data writes content, options.style applies table style (e.g., 'TableStyleMedium9'), options.auto_size=true auto-fits columns.",
			"options.style":             "Table style name for create_table. Examples: 'TableStyleMedium2', 'TableStyleLight9', 'TableStyleDark1'. Applies professional formatting in one parameter.",
			"options.formula":           "Excel formula without leading '='. Used in apply_formula. For write_data/create_table, formulas are auto-detected when values start with '='.",
			"options.initial_sheets":    "Array of sheet names to create when creating a new workbook. Alternative to creating workbook then adding sheets individually.",
			"format_range.options.font": "Font properties object: {bold: true, italic: true, size: 12, colour: 'FF0000', family: 'Arial'}. Accepts both 'colour' and 'color' spellings.",
			"format_range.options.fill": "Fill properties object: {colour: 'E2EFDA', pattern: 'solid'}. Use hex colours without '#' prefix.",
		},
		WhenToUse:    "Creating, editing, or formatting Excel spreadsheets with formulas, charts, tables, or data validation. Ideal for generating reports, data analysis outputs, structured data exports, or financial documents. Supports complex formatting, conditional formatting, pivot tables, and cross-sheet formula references.",
		WhenNotToUse: "For simple CSV data export without formatting (use CSV tools instead). For reading extremely large datasets >100k rows (consider streaming or database approaches). For complex manual spreadsheet calculations better suited to interactive Excel usage. For real-time collaborative editing (use Google Sheets API instead).",
	}
}
