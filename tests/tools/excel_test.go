package tools_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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

// createMultiSheetTestWorkbook creates a test workbook with multiple sheets
func createMultiSheetTestWorkbook(t *testing.T, path string) {
	t.Helper()

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Warning: failed to close workbook: %v", err)
		}
	}()

	// Rename default sheet to "Sales"
	defaultSheet := f.GetSheetName(0)
	_ = f.SetSheetName(defaultSheet, "Sales")

	// Add Sales data
	_ = f.SetCellValue("Sales", "A1", "Month")
	_ = f.SetCellValue("Sales", "B1", "Revenue")
	_ = f.SetCellValue("Sales", "A2", "Jan")
	_ = f.SetCellValue("Sales", "B2", 5000)
	_ = f.SetCellValue("Sales", "A3", "Feb")
	_ = f.SetCellValue("Sales", "B3", 6500)

	// Create Expenses sheet
	_, _ = f.NewSheet("Expenses")
	_ = f.SetCellValue("Expenses", "A1", "Category")
	_ = f.SetCellValue("Expenses", "B1", "Amount")
	_ = f.SetCellValue("Expenses", "A2", "Rent")
	_ = f.SetCellValue("Expenses", "B2", 2000)
	_ = f.SetCellValue("Expenses", "A3", "Utilities")
	_ = f.SetCellValue("Expenses", "B3", 500)

	// Create Empty sheet
	_, _ = f.NewSheet("Empty")

	if err := f.SaveAs(path); err != nil {
		t.Fatalf("Failed to create multi-sheet test workbook: %v", err)
	}
}

func TestExcel_ReadAllData_AllSheets_CSV(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createMultiSheetTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "read_all_data",
		"filepath": testFile,
		"options": map[string]any{
			"format": "csv",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify result has content
	testutils.AssertTrue(t, len(result.Content) > 0)
}

func TestExcel_ReadAllData_SpecificSheets_TSV(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createMultiSheetTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "read_all_data",
		"filepath": testFile,
		"options": map[string]any{
			"sheet_names": []any{"Sales"},
			"format":      "tsv",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify result has content
	testutils.AssertTrue(t, len(result.Content) > 0)
}

func TestExcel_ReadAllData_MaxRows(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createMultiSheetTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "read_all_data",
		"filepath": testFile,
		"options": map[string]any{
			"sheet_names": []any{"Sales"},
			"format":      "csv",
			"max_rows":    float64(2), // Only read 2 rows (header + 1 data row)
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify result has content
	testutils.AssertTrue(t, len(result.Content) > 0)
}

func TestExcel_ReadAllData_JSON(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createMultiSheetTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "read_all_data",
		"filepath": testFile,
		"options": map[string]any{
			"sheet_names": []any{"Sales"},
			"format":      "json",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify result has content
	testutils.AssertTrue(t, len(result.Content) > 0)

	// Validate JSON structure
	textContent, ok := mcp.AsTextContent(result.Content[0])
	testutils.AssertTrue(t, ok)

	var jsonData map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &jsonData)
	testutils.AssertNoError(t, err)

	// Verify sheets array exists
	sheets, ok := jsonData["sheets"].([]any)
	testutils.AssertTrue(t, ok)
	testutils.AssertTrue(t, len(sheets) > 0)

	// Verify first sheet structure
	sheet := sheets[0].(map[string]any)
	testutils.AssertTrue(t, sheet["sheet_name"] != nil)
	testutils.AssertTrue(t, sheet["format"] == "json")
	testutils.AssertTrue(t, sheet["data"] != nil)

	// Validate that the data field contains valid JSON (2D array)
	dataStr := sheet["data"].(string)
	var arrayData [][]string
	err = json.Unmarshal([]byte(dataStr), &arrayData)
	testutils.AssertNoError(t, err)
	testutils.AssertTrue(t, len(arrayData) > 0) // Should have at least one row
}

func TestExcel_ReadAllData_InvalidFormat(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createMultiSheetTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "read_all_data",
		"filepath": testFile,
		"options": map[string]any{
			"format": "invalid_format",
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "format must be one of")
}

func TestExcel_ReadAllData_NonExistentSheet(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createMultiSheetTestWorkbook(t, testFile)

	args := map[string]any{
		"function": "read_all_data",
		"filepath": testFile,
		"options": map[string]any{
			"sheet_names": []any{"NonExistent"},
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "worksheet not found")
}

func TestExcel_ReadAllData_Pagination(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	createMultiSheetTestWorkbook(t, testFile)

	// Test 1: Read first 2 rows with offset 0
	args := map[string]any{
		"function": "read_all_data",
		"filepath": testFile,
		"options": map[string]any{
			"sheet_names": []any{"Sales"},
			"format":      "csv",
			"max_rows":    float64(2),
			"offset":      float64(0),
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
	testutils.AssertTrue(t, len(result.Content) > 0)

	// Test 2: Read next 2 rows using offset
	args["options"] = map[string]any{
		"sheet_names": []any{"Sales"},
		"format":      "csv",
		"max_rows":    float64(2),
		"offset":      float64(2),
	}

	result2, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result2)
	testutils.AssertTrue(t, len(result2.Content) > 0)

	// Test 3: Offset beyond data range - should skip the sheet and return empty sheets array
	args["options"] = map[string]any{
		"sheet_names": []any{"Sales"},
		"format":      "csv",
		"max_rows":    float64(10),
		"offset":      float64(1000),
	}

	result3, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result3)

	// Parse result to verify empty sheets array
	textContent3, ok3 := mcp.AsTextContent(result3.Content[0])
	testutils.AssertTrue(t, ok3)

	var result3Data map[string]any
	err = json.Unmarshal([]byte(textContent3.Text), &result3Data)
	testutils.AssertNoError(t, err)

	sheets3 := result3Data["sheets"].([]any)
	testutils.AssertTrue(t, len(sheets3) == 0) // Should be empty when offset beyond data

	// Test 4: Negative offset should return error
	args["options"] = map[string]any{
		"sheet_names": []any{"Sales"},
		"format":      "csv",
		"offset":      float64(-1),
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "offset must be non-negative")
}

func TestExcel_ReadAllData_IrregularRowLengths(t *testing.T) {
	// Enable the tool for this test
	defer enableExcelTool(t)()

	tool := &excel.ExcelTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create temp directory and test file with irregular row lengths
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "irregular.xlsx")

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Warning: Failed to close workbook: %v", err)
		}
	}()

	sheetName := "IrregularData"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		t.Fatalf("Failed to create sheet: %v", err)
	}
	f.SetActiveSheet(index)
	_ = f.DeleteSheet("Sheet1")

	// Create rows with varying column counts
	// Row 1: 3 columns
	_ = f.SetCellValue(sheetName, "A1", "Col1")
	_ = f.SetCellValue(sheetName, "B1", "Col2")
	_ = f.SetCellValue(sheetName, "C1", "Col3")

	// Row 2: 5 columns (widest)
	_ = f.SetCellValue(sheetName, "A2", "Data1")
	_ = f.SetCellValue(sheetName, "B2", "Data2")
	_ = f.SetCellValue(sheetName, "C2", "Data3")
	_ = f.SetCellValue(sheetName, "D2", "Data4")
	_ = f.SetCellValue(sheetName, "E2", "Data5")

	// Row 3: 2 columns
	_ = f.SetCellValue(sheetName, "A3", "Short1")
	_ = f.SetCellValue(sheetName, "B3", "Short2")

	if err := f.SaveAs(testFile); err != nil {
		t.Fatalf("Failed to create irregular test workbook: %v", err)
	}

	// Test CSV format
	csvArgs := map[string]any{
		"function":   "read_all_data",
		"filepath":   testFile,
		"sheet_name": sheetName,
		"options": map[string]any{
			"format": "csv",
		},
	}

	csvResult, err := tool.Execute(ctx, logger, cache, csvArgs)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, csvResult)

	// Verify CSV data has normalised row lengths
	csvTextContent, csvOk := mcp.AsTextContent(csvResult.Content[0])
	testutils.AssertTrue(t, csvOk)

	var csvData map[string]any
	err = json.Unmarshal([]byte(csvTextContent.Text), &csvData)
	testutils.AssertNoError(t, err)

	sheets := csvData["sheets"].([]any)
	testutils.AssertTrue(t, len(sheets) == 1)
	sheet := sheets[0].(map[string]any)

	// Verify all rows have 5 columns (padded to maxCols)
	dimensions := sheet["dimensions"].(map[string]any)
	testutils.AssertTrue(t, dimensions["columns"].(float64) == 5)

	csvLines := strings.Split(sheet["data"].(string), "\n")
	testutils.AssertTrue(t, len(csvLines) == 3) // 3 rows

	// Each CSV line should have 5 values (some may be empty)
	for i, line := range csvLines {
		parts := strings.Split(line, ",")
		if len(parts) != 5 {
			t.Errorf("Row %d: expected 5 columns, got %d. Line: %s", i+1, len(parts), line)
		}
	}

	// Test TSV format
	tsvArgs := map[string]any{
		"function":   "read_all_data",
		"filepath":   testFile,
		"sheet_name": sheetName,
		"options": map[string]any{
			"format": "tsv",
		},
	}

	tsvResult, err := tool.Execute(ctx, logger, cache, tsvArgs)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, tsvResult)

	tsvTextContent, tsvOk := mcp.AsTextContent(tsvResult.Content[0])
	testutils.AssertTrue(t, tsvOk)

	var tsvData map[string]any
	err = json.Unmarshal([]byte(tsvTextContent.Text), &tsvData)
	testutils.AssertNoError(t, err)

	tsvSheets := tsvData["sheets"].([]any)
	tsvSheet := tsvSheets[0].(map[string]any)
	tsvLines := strings.Split(tsvSheet["data"].(string), "\n")

	for i, line := range tsvLines {
		parts := strings.Split(line, "\t")
		if len(parts) != 5 {
			t.Errorf("TSV Row %d: expected 5 columns, got %d. Line: %s", i+1, len(parts), line)
		}
	}

	// Test JSON format
	jsonArgs := map[string]any{
		"function":   "read_all_data",
		"filepath":   testFile,
		"sheet_name": sheetName,
		"options": map[string]any{
			"format": "json",
		},
	}

	jsonResult, err := tool.Execute(ctx, logger, cache, jsonArgs)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, jsonResult)

	jsonTextContent, jsonOk := mcp.AsTextContent(jsonResult.Content[0])
	testutils.AssertTrue(t, jsonOk)

	var jsonData map[string]any
	err = json.Unmarshal([]byte(jsonTextContent.Text), &jsonData)
	testutils.AssertNoError(t, err)

	jsonSheets := jsonData["sheets"].([]any)
	jsonSheet := jsonSheets[0].(map[string]any)

	// Parse the JSON array data
	var jsonArrayData [][]string
	dataStr := jsonSheet["data"].(string)
	err = json.Unmarshal([]byte(dataStr), &jsonArrayData)
	testutils.AssertNoError(t, err)

	// Verify all rows have 5 elements
	testutils.AssertTrue(t, len(jsonArrayData) == 3)
	for i, row := range jsonArrayData {
		if len(row) != 5 {
			t.Errorf("JSON Row %d: expected 5 columns, got %d. Row: %v", i+1, len(row), row)
		}
	}
}
