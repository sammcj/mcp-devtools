package excel

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleInsertRows inserts one or more rows
func handleInsertRows(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	startRow, ok := options["start_row"].(float64)
	if !ok {
		if intRow, ok := options["start_row"].(int); ok {
			startRow = float64(intRow)
		} else {
			return nil, &ValidationError{
				Field:   "start_row",
				Value:   options["start_row"],
				Message: "start_row parameter is required",
			}
		}
	}

	count := 1.0
	if c, ok := options["count"].(float64); ok {
		count = c
	} else if c, ok := options["count"].(int); ok {
		count = float64(c)
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
		"start_row":  int(startRow),
		"count":      int(count),
	}).Info("Inserting rows")

	// Open workbook
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, &WorkbookError{
			Operation: "open",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to open workbook: %w", err),
		}
	}
	defer func() {
		_ = f.Close()
	}()

	// Check if sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "insert_rows",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Insert rows
	for i := 0; i < int(count); i++ {
		if err := f.InsertRows(sheetName, int(startRow), 1); err != nil {
			return nil, &RangeError{
				Operation: "insert_rows",
				Range:     fmt.Sprintf("row %d", int(startRow)),
				Cause:     fmt.Errorf("failed to insert rows: %w", err),
			}
		}
	}

	// Save workbook with secure permissions
	if err := saveWorkbookWithPermissions(f, filePath, nil); err != nil {
		return nil, &WorkbookError{
			Operation: "save",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to save workbook: %w", err),
		}
	}

	result := map[string]any{
		"rows_inserted": int(count),
	}

	return mcp.NewToolResultJSON(result)
}

// handleInsertColumns inserts one or more columns
func handleInsertColumns(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	startCol, ok := options["start_column"].(float64)
	if !ok {
		if intCol, ok := options["start_column"].(int); ok {
			startCol = float64(intCol)
		} else {
			return nil, &ValidationError{
				Field:   "start_column",
				Value:   options["start_column"],
				Message: "start_column parameter is required",
			}
		}
	}

	count := 1.0
	if c, ok := options["count"].(float64); ok {
		count = c
	} else if c, ok := options["count"].(int); ok {
		count = float64(c)
	}

	logger.WithFields(logrus.Fields{
		"filepath":     filePath,
		"sheet_name":   sheetName,
		"start_column": int(startCol),
		"count":        int(count),
	}).Info("Inserting columns")

	// Open workbook
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, &WorkbookError{
			Operation: "open",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to open workbook: %w", err),
		}
	}
	defer func() {
		_ = f.Close()
	}()

	// Check if sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "insert_columns",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Convert column number to column name
	colName, err := excelize.ColumnNumberToName(int(startCol))
	if err != nil {
		return nil, &ValidationError{
			Field:   "start_column",
			Value:   startCol,
			Message: fmt.Sprintf("invalid column number: %v", err),
		}
	}

	// Insert columns
	for i := 0; i < int(count); i++ {
		if err := f.InsertCols(sheetName, colName, 1); err != nil {
			return nil, &RangeError{
				Operation: "insert_columns",
				Range:     fmt.Sprintf("column %s", colName),
				Cause:     fmt.Errorf("failed to insert columns: %w", err),
			}
		}
	}

	// Save workbook with secure permissions
	if err := saveWorkbookWithPermissions(f, filePath, nil); err != nil {
		return nil, &WorkbookError{
			Operation: "save",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to save workbook: %w", err),
		}
	}

	result := map[string]any{
		"columns_inserted": int(count),
	}

	return mcp.NewToolResultJSON(result)
}

// handleDeleteRows deletes one or more rows
func handleDeleteRows(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	startRow, ok := options["start_row"].(float64)
	if !ok {
		if intRow, ok := options["start_row"].(int); ok {
			startRow = float64(intRow)
		} else {
			return nil, &ValidationError{
				Field:   "start_row",
				Value:   options["start_row"],
				Message: "start_row parameter is required",
			}
		}
	}

	count := 1.0
	if c, ok := options["count"].(float64); ok {
		count = c
	} else if c, ok := options["count"].(int); ok {
		count = float64(c)
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
		"start_row":  int(startRow),
		"count":      int(count),
	}).Info("Deleting rows")

	// Open workbook
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, &WorkbookError{
			Operation: "open",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to open workbook: %w", err),
		}
	}
	defer func() {
		_ = f.Close()
	}()

	// Check if sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "delete_rows",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Delete rows (call RemoveRow for each row to delete)
	for i := 0; i < int(count); i++ {
		if err := f.RemoveRow(sheetName, int(startRow)); err != nil {
			return nil, &RangeError{
				Operation: "delete_rows",
				Range:     fmt.Sprintf("rows %d-%d", int(startRow), int(startRow)+int(count)-1),
				Cause:     fmt.Errorf("failed to delete rows: %w", err),
			}
		}
	}

	// Save workbook with secure permissions
	if err := saveWorkbookWithPermissions(f, filePath, nil); err != nil {
		return nil, &WorkbookError{
			Operation: "save",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to save workbook: %w", err),
		}
	}

	result := map[string]any{
		"rows_deleted": int(count),
	}

	return mcp.NewToolResultJSON(result)
}

// handleDeleteColumns deletes one or more columns
func handleDeleteColumns(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	startCol, ok := options["start_column"].(float64)
	if !ok {
		if intCol, ok := options["start_column"].(int); ok {
			startCol = float64(intCol)
		} else {
			return nil, &ValidationError{
				Field:   "start_column",
				Value:   options["start_column"],
				Message: "start_column parameter is required",
			}
		}
	}

	count := 1.0
	if c, ok := options["count"].(float64); ok {
		count = c
	} else if c, ok := options["count"].(int); ok {
		count = float64(c)
	}

	logger.WithFields(logrus.Fields{
		"filepath":     filePath,
		"sheet_name":   sheetName,
		"start_column": int(startCol),
		"count":        int(count),
	}).Info("Deleting columns")

	// Open workbook
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, &WorkbookError{
			Operation: "open",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to open workbook: %w", err),
		}
	}
	defer func() {
		_ = f.Close()
	}()

	// Check if sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "delete_columns",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Convert column number to column name
	colName, err := excelize.ColumnNumberToName(int(startCol))
	if err != nil {
		return nil, &ValidationError{
			Field:   "start_column",
			Value:   startCol,
			Message: fmt.Sprintf("invalid column number: %v", err),
		}
	}

	// Delete columns (call RemoveCol for each column to delete)
	for i := 0; i < int(count); i++ {
		if err := f.RemoveCol(sheetName, colName); err != nil {
			return nil, &RangeError{
				Operation: "delete_columns",
				Range:     fmt.Sprintf("column %s", colName),
				Cause:     fmt.Errorf("failed to delete columns: %w", err),
			}
		}
	}

	// Save workbook with secure permissions
	if err := saveWorkbookWithPermissions(f, filePath, nil); err != nil {
		return nil, &WorkbookError{
			Operation: "save",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to save workbook: %w", err),
		}
	}

	result := map[string]any{
		"columns_deleted": int(count),
	}

	return mcp.NewToolResultJSON(result)
}

// handleAutoSizeColumns automatically adjusts column widths to fit content
func handleAutoSizeColumns(ctx context.Context, filePath string, sheetName string) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	// Open workbook
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, &WorkbookError{
			Operation: "open",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to open workbook: %w", err),
		}
	}
	defer func() {
		_ = f.Close()
	}()

	// Check if sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "auto_size_columns",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Get all rows to determine column widths
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, &SheetError{
			Operation: "auto_size_columns",
			SheetName: sheetName,
			Cause:     fmt.Errorf("failed to get rows: %w", err),
		}
	}

	if len(rows) == 0 {
		result := map[string]any{}
		return mcp.NewToolResultJSON(result)
	}

	// Calculate max width for each column
	columnWidths := make(map[int]float64)
	for _, row := range rows {
		for colIdx, cellValue := range row {
			// Calculate width based on content length
			// Excel width units are approximately 1/7 of a character width
			// Add some padding for better appearance
			contentLength := len(cellValue)
			width := float64(contentLength) + 2.0 // Add 2 for padding

			// Minimum width of 8, maximum of 50
			if width < 8.0 {
				width = 8.0
			}
			if width > 50.0 {
				width = 50.0
			}

			if width > columnWidths[colIdx] {
				columnWidths[colIdx] = width
			}
		}
	}

	// Apply column widths
	columnsResized := 0
	for colIdx, width := range columnWidths {
		colName, err := excelize.ColumnNumberToName(colIdx + 1)
		if err != nil {
			continue
		}

		if err := f.SetColWidth(sheetName, colName, colName, width); err != nil {
			continue
		}

		columnsResized++
	}

	// Save workbook with secure permissions
	if err := saveWorkbookWithPermissions(f, filePath, nil); err != nil {
		return nil, &WorkbookError{
			Operation: "save",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to save workbook: %w", err),
		}
	}

	result := map[string]any{
		"columns_resized": columnsResized,
	}

	return mcp.NewToolResultJSON(result)
}
