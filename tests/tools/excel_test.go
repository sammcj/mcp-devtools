package tools_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/excel"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/xuri/excelize/v2"
)

// enableExcelTool enables the Excel tool for testing
func enableExcelTool(t *testing.T) func() {
	t.Helper()
	originalValue := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "excel")

	return func() {
		if originalValue == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalValue)
		}
	}
}

// createTestWorkbook creates a test Excel workbook with sample data
func createTestWorkbook(t *testing.T, path string) {
	t.Helper()

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Warning: failed to close workbook: %v", err)
		}
	}()

	// Create Sheet1 with sample data
	sheet := "Sheet1"
	_ = f.SetCellValue(sheet, "A1", "Name")
	_ = f.SetCellValue(sheet, "B1", "Age")
	_ = f.SetCellValue(sheet, "C1", "Salary")

	_ = f.SetCellValue(sheet, "A2", "Alice")
	_ = f.SetCellValue(sheet, "B2", 30)
	_ = f.SetCellValue(sheet, "C2", 75000)

	_ = f.SetCellValue(sheet, "A3", "Bob")
	_ = f.SetCellValue(sheet, "B3", 25)
	_ = f.SetCellValue(sheet, "C3", 65000)

	_ = f.SetCellValue(sheet, "A4", "Charlie")
	_ = f.SetCellValue(sheet, "B4", 35)
	_ = f.SetCellValue(sheet, "C4", 85000)

	if err := f.SaveAs(path); err != nil {
		t.Fatalf("Failed to create test workbook: %v", err)
	}
}

func TestExcel_Definition(t *testing.T) {
	tool := &excel.ExcelTool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "excel", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "workbook") {
		t.Errorf("Expected description to contain 'workbook', got: %s", desc)
	}
	if !testutils.Contains(desc, "chart") {
		t.Errorf("Expected description to contain 'chart', got: %s", desc)
	}
	if !testutils.Contains(desc, "pivot") {
		t.Errorf("Expected description to contain 'pivot', got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestExcel_Execute_ToolNotEnabled(t *testing.T) {
	// Skip if tool is enabled
	if os.Getenv("ENABLE_ADDITIONAL_TOOLS") != "" {
		t.Skip("Skipping test because ENABLE_ADDITIONAL_TOOLS is set")
	}

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"function": "create_workbook",
		"filepath": "/tmp/test.xlsx",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "not enabled")
}

func TestExcel_Execute_MissingFunction(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"filepath": "/tmp/test.xlsx",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "function")
}

func TestExcel_Execute_MissingFilepath(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"function": "create_workbook",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "filepath")
}

func TestExcel_Execute_UnknownFunction(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"function": "unknown_function",
		"filepath": "/tmp/test.xlsx",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "unknown function")
}

func TestExcel_CreateChart_MissingSheetName(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "create_chart",
		"filepath": testFile,
		"options": map[string]any{
			"type":     "column",
			"position": "E2",
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "sheet_name")
}

func TestExcel_CreateChart_MissingType(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_chart",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"position": "E2",
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "type")
}

func TestExcel_CreateChart_InvalidType(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_chart",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"type":     "invalid_type",
			"position": "E2",
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "invalid chart type")
}

func TestExcel_CreateChart_Success(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_chart",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"type":       "column",
			"position":   "E2",
			"title":      "Sales Chart",
			"data_range": "A1:C4",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_CreatePivotTable_MissingParameters(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	tests := []struct {
		name    string
		options map[string]any
		errMsg  string
	}{
		{
			name:    "missing source_range",
			options: map[string]any{},
			errMsg:  "source_range",
		},
		{
			name: "missing row_fields",
			options: map[string]any{
				"source_range": "A1:C4",
			},
			errMsg: "row_fields",
		},
		{
			name: "missing data_fields",
			options: map[string]any{
				"source_range": "A1:C4",
				"row_fields":   []any{"Name"},
			},
			errMsg: "data_fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{
				"function":   "create_pivot_table",
				"filepath":   testFile,
				"sheet_name": "Sheet1",
				"options":    tt.options,
			}

			_, err := tool.Execute(ctx, logger, cache, args)
			testutils.AssertError(t, err)
			testutils.AssertErrorContains(t, err, tt.errMsg)
		})
	}
}

func TestExcel_CreatePivotTable_Success(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_pivot_table",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"source_range": "A1:C4",
			"row_fields":   []any{"Name"},
			"data_fields": []any{
				map[string]any{
					"field":    "Salary",
					"function": "sum",
					"name":     "Total Salary",
				},
			},
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_CreateTable_MissingRange(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_table",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options":    map[string]any{},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "range")
}

func TestExcel_CreateTable_Success(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_table",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C4",
			"name":  "DataTable",
			"style": "TableStyleMedium9",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_CreateTable_AutoGenerateName(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_table",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C4",
			// No name provided, should auto-generate
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_CreateTable_AllInOne(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")

	// Create an empty workbook first
	args := map[string]any{
		"function": "create_workbook",
		"filepath": testFile,
	}
	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)

	// Test all-in-one table creation with data, auto-size, and style
	args = map[string]any{
		"function":   "create_table",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C4",
			"data": []any{
				[]any{"Product", "Quantity", "Price"},
				[]any{"Apples", 50, 1.20},
				[]any{"Oranges", 30, 1.50},
				[]any{"Bananas", 40, 0.80},
			},
			"name":      "ProductTable",
			"style":     "TableStyleMedium2",
			"auto_size": true,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify the file exists and has the table
	_, err = os.Stat(testFile)
	testutils.AssertNoError(t, err)

	// Open the file and verify table was created
	f, err := excelize.OpenFile(testFile)
	testutils.AssertNoError(t, err)
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Warning: failed to close workbook: %v", err)
		}
	}()

	// Verify data was written
	val, err := f.GetCellValue("Sheet1", "A1")
	testutils.AssertNoError(t, err)
	testutils.AssertEqual(t, "Product", val)

	val, err = f.GetCellValue("Sheet1", "B2")
	testutils.AssertNoError(t, err)
	testutils.AssertEqual(t, "50", val)
}

func TestExcel_ChartTypes(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	chartTypes := []string{"line", "bar", "column", "pie", "scatter", "area"}

	for _, chartType := range chartTypes {
		t.Run(chartType, func(t *testing.T) {
			args := map[string]any{
				"function":   "create_chart",
				"filepath":   testFile,
				"sheet_name": "Sheet1",
				"options": map[string]any{
					"type":       chartType,
					"position":   "E2",
					"data_range": "A1:C4",
				},
			}

			result, err := tool.Execute(ctx, logger, cache, args)
			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)
		})
	}
}

// Phase 2: Workbook and Worksheet Operations Tests

func TestExcel_CreateWorkbook_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "new_workbook.xlsx")

	args := map[string]any{
		"function": "create_workbook",
		"filepath": testFile,
		"options": map[string]any{
			"initial_sheet_name": "DataSheet",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("Workbook file was not created: %s", testFile)
	}
}

func TestExcel_GetWorkbookMetadata(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "get_workbook_metadata",
		"filepath": testFile,
		"options": map[string]any{
			"include_ranges": true,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_CreateWorksheet_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "create_worksheet",
		"filepath":   testFile,
		"sheet_name": "NewSheet",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_CopyWorksheet_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "copy_worksheet",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"target_name": "Sheet1_Copy",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_DeleteWorksheet_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	// Create a second sheet first
	createArgs := map[string]any{
		"function":   "create_worksheet",
		"filepath":   testFile,
		"sheet_name": "Sheet2",
	}
	_, err := tool.Execute(ctx, logger, cache, createArgs)
	testutils.AssertNoError(t, err)

	// Now delete the original sheet
	deleteArgs := map[string]any{
		"function":   "delete_worksheet",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
	}

	result, err := tool.Execute(ctx, logger, cache, deleteArgs)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_RenameWorksheet_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "rename_worksheet",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"new_name": "RenamedSheet",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

// Phase 3: Data Operations Tests

func TestExcel_WriteData_VariousTypes(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	// Test data with various types - need to use []any for outer array
	testData := []any{
		[]any{"String", "Number", "Boolean", "Float"},
		[]any{"Alice", 30, true, 99.99},
		[]any{"Bob", 25, false, 75.50},
		[]any{"Charlie", nil, true, 100.00},
	}

	args := map[string]any{
		"function":   "write_data",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"start_cell": "A1",
			"data":       testData,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_ReadData_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "read_data",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"start_cell": "A1",
			"end_cell":   "C4",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_ReadDataWithMetadata_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "read_data_with_metadata",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"start_cell": "A1",
			"end_cell":   "C4",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

// Phase 4: Formatting Tests

func TestExcel_FormatRange_BasicFormatting(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "format_range",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C1",
			"font": map[string]any{
				"bold":   true,
				"size":   14.0,
				"colour": "#FF0000",
				"family": "Arial",
			},
			"fill": map[string]any{
				"colour":  "#FFFF00",
				"pattern": "solid",
			},
			"alignment": map[string]any{
				"horizontal": "centre",
				"vertical":   "middle",
				"wrap_text":  true,
			},
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_FormatRange_NumberFormat(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "format_range",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range":         "C2:C4",
			"number_format": "£#,##0.00",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_FormatRange_ConditionalFormatting(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	tests := []struct {
		name   string
		cfType string
		rule   map[string]any
	}{
		{
			name:   "colour_scale",
			cfType: "colour_scale",
			rule: map[string]any{
				"min_colour": "#FF0000",
				"max_colour": "#00FF00",
			},
		},
		{
			name:   "data_bar",
			cfType: "data_bar",
			rule: map[string]any{
				"colour": "#0000FF",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{
				"function":   "format_range",
				"filepath":   testFile,
				"sheet_name": "Sheet1",
				"options": map[string]any{
					"range": "B2:B4",
					"conditional_format": map[string]any{
						"type": tt.cfType,
						"rule": tt.rule,
					},
				},
			}

			result, err := tool.Execute(ctx, logger, cache, args)
			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)
		})
	}
}

// Phase 5: Range Operations Tests

func TestExcel_MergeCells_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "merge_cells",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C1",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_UnmergeCells_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	// Merge first
	mergeArgs := map[string]any{
		"function":   "merge_cells",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C1",
		},
	}
	_, err := tool.Execute(ctx, logger, cache, mergeArgs)
	testutils.AssertNoError(t, err)

	// Now unmerge
	unmergeArgs := map[string]any{
		"function":   "unmerge_cells",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C1",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, unmergeArgs)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_GetMergedCells_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "get_merged_cells",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_CopyRange_SameSheet(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "copy_range",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"source_range": "A1:C2",
			"target_cell":  "E1",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_DeleteRange_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "delete_range",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range":           "A2:C2",
			"shift_direction": "up",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_ValidateRange_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "validate_range",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"range": "A1:C4",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

// Phase 5: Row/Column Operations Tests

func TestExcel_InsertRows_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "insert_rows",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"start_row": 2.0,
			"count":     2.0,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_InsertColumns_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "insert_columns",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"start_column": 2.0,
			"count":        1.0,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_DeleteRows_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "delete_rows",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"start_row": 3.0,
			"count":     1.0,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_DeleteColumns_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "delete_columns",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"start_column": 3.0,
			"count":        1.0,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

// Phase 6: Formula Tests

func TestExcel_ApplyFormula_Success(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "apply_formula",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"cell":    "D2",
			"formula": "=SUM(B2:C2)",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_ApplyFormula_UnsafeFunction(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	// Test each dangerous function
	dangerousFunctions := []string{"INDIRECT", "HYPERLINK", "WEBSERVICE", "DGET", "RTD"}

	for _, dangerousFunc := range dangerousFunctions {
		t.Run(dangerousFunc, func(t *testing.T) {
			args := map[string]any{
				"function":   "apply_formula",
				"filepath":   testFile,
				"sheet_name": "Sheet1",
				"options": map[string]any{
					"cell":    "D2",
					"formula": fmt.Sprintf("=%s(A1)", dangerousFunc),
				},
			}

			_, err := tool.Execute(ctx, logger, cache, args)
			testutils.AssertError(t, err)
			testutils.AssertErrorContains(t, err, "unsafe")
		})
	}
}

func TestExcel_ValidateFormulaSyntax_Valid(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "validate_formula_syntax",
		"filepath": testFile,
		"options": map[string]any{
			"cell":    "D2",
			"formula": "=SUM(A1:A10)",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestExcel_ValidateFormulaSyntax_Invalid(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	tests := []struct {
		name    string
		formula string
	}{
		{"unbalanced_parentheses", "=SUM(A1:A10"},
		{"empty_formula", "="},
		{"unsafe_function", "=INDIRECT(A1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{
				"function": "validate_formula_syntax",
				"filepath": testFile,
				"options": map[string]any{
					"cell":    "D2",
					"formula": tt.formula,
				},
			}

			result, err := tool.Execute(ctx, logger, cache, args)
			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)
		})
	}
}

// Phase 9: Data Validation Tests

func TestExcel_GetDataValidationInfo(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "get_data_validation_info",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

// Phase 10: Error Handling Tests

func TestExcel_InvalidCellReference(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "apply_formula",
		"filepath":   testFile,
		"sheet_name": "Sheet1",
		"options": map[string]any{
			"cell":    "XYZ999",
			"formula": "=SUM(A1:A10)",
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
}

func TestExcel_NonExistentFile(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"function":   "read_data",
		"filepath":   "/tmp/nonexistent_file.xlsx",
		"sheet_name": "Sheet1",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
}

func TestExcel_NonExistentWorksheet(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	args := map[string]any{
		"function":   "read_data",
		"filepath":   testFile,
		"sheet_name": "NonExistentSheet",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
}

func TestExcel_InvalidWorksheetName(t *testing.T) {
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createTestWorkbook(t, testFile)

	// Test invalid characters
	invalidNames := []string{
		"Sheet:Name",
		"Sheet/Name",
		"Sheet?Name",
		"Sheet*Name",
		"Sheet[Name]",
	}

	for _, invalidName := range invalidNames {
		t.Run(invalidName, func(t *testing.T) {
			args := map[string]any{
				"function":   "create_worksheet",
				"filepath":   testFile,
				"sheet_name": invalidName,
			}

			_, err := tool.Execute(ctx, logger, cache, args)
			testutils.AssertError(t, err)
		})
	}
}
