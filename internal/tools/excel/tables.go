package excel

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleCreateTable creates an Excel table object in the worksheet
// Optionally writes data first and auto-sizes columns for an all-in-one table creation
func handleCreateTable(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
	}).Info("Creating Excel table in worksheet")

	// Validate required parameters
	tableRange, ok := options["range"].(string)
	if !ok || tableRange == "" {
		return nil, &ValidationError{
			Field:   "range",
			Value:   options["range"],
			Message: "range parameter is required (e.g., 'A1:D100')",
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
		if err := f.Close(); err != nil {
			logger.WithError(err).Warn("Failed to close workbook")
		}
	}()

	// Check if sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "create_table",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	cellsWritten := 0
	// Write data if provided
	if data, ok := options["data"].([]any); ok && len(data) > 0 {
		// Parse range to get start cell
		startRow, startCol, _, _, err := parseRange(tableRange)
		if err != nil {
			return nil, err
		}

		// Write data to worksheet
		for rowIdx, rowData := range data {
			rowArray, ok := rowData.([]any)
			if !ok {
				continue
			}

			for colIdx, cellValue := range rowArray {
				currentRow := startRow + rowIdx
				currentCol := startCol + colIdx

				cell, err := coordinatesToCell(currentCol, currentRow)
				if err != nil {
					logger.WithError(err).Warn("Failed to convert coordinates")
					continue
				}

				// Check if value is a formula
				if strValue, ok := cellValue.(string); ok && len(strValue) > 0 && strValue[0] == '=' {
					// Strip leading = for consistency with write_data
					formula := strValue[1:]

					// Apply formula
					if err := f.SetCellFormula(sheetName, cell, formula); err != nil {
						logger.WithError(err).WithField("cell", cell).Warn("Failed to set formula")
						continue
					}

					// Calculate formula value for Numbers compatibility
					// Numbers requires cached values to display formulas correctly
					calculatedValue, err := f.CalcCellValue(sheetName, cell)
					if err != nil {
						logger.WithFields(logrus.Fields{
							"cell":    cell,
							"formula": formula,
							"error":   err.Error(),
						}).Debug("Failed to calculate formula value for caching (formula is still set)")
					} else {
						logger.WithFields(logrus.Fields{
							"cell":             cell,
							"formula":          formula,
							"calculated_value": calculatedValue,
						}).Debug("Calculated formula value for Numbers compatibility")
					}
				} else {
					if err := f.SetCellValue(sheetName, cell, cellValue); err != nil {
						logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell value")
						continue
					}
				}

				cellsWritten++
			}
		}
	}

	// Generate or use provided table name
	tableName := generateTableName(sheetName, options)

	// Validate table name
	if err := validateTableName(tableName); err != nil {
		return nil, err
	}

	// Build table configuration
	tableConfig := buildTableConfig(tableRange, tableName, options)

	// Add table to worksheet
	if err := f.AddTable(sheetName, tableConfig); err != nil {
		// Check if this is a duplicate table name error
		errStr := err.Error()
		if containsAny(errStr, []string{"same name", "already exists", "duplicate"}) {
			return nil, &ChartError{
				Operation: "create_table",
				ChartType: "excel_table",
				Cause:     fmt.Errorf("table name '%s' already exists in this workbook. Please specify a unique name using options.name parameter (e.g., {\"name\": \"CustomTableName\"})", tableName),
			}
		}
		return nil, &ChartError{ // Reusing ChartError for tables
			Operation: "create_table",
			ChartType: "excel_table",
			Cause:     fmt.Errorf("failed to create table: %w", err),
		}
	}

	// Auto-size columns (default: true)
	autoSize := true
	if val, ok := options["auto_size"].(bool); ok {
		autoSize = val
	}
	columnsResized := 0
	if autoSize {
		// Get all rows to determine column widths
		rows, err := f.GetRows(sheetName)
		if err == nil && len(rows) > 0 {
			// Calculate max width for each column
			columnWidths := make(map[int]float64)
			for _, row := range rows {
				for colIdx, cellValue := range row {
					contentLength := len(cellValue)
					width := float64(contentLength) + 2.0

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
		}
	}

	// Save workbook with secure permissions
	if err := saveWorkbookWithPermissions(f, filePath, logger); err != nil {
		return nil, &WorkbookError{
			Operation: "save",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to save workbook: %w", err),
		}
	}

	result := map[string]any{
		"table_name": tableName,
	}

	if cellsWritten > 0 {
		result["cells_written"] = cellsWritten
	}
	if columnsResized > 0 {
		result["columns_resized"] = columnsResized
	}

	return mcp.NewToolResultJSON(result)
}

// generateTableName generates a table name based on sheet name or uses the provided one
func generateTableName(sheetName string, options map[string]any) string {
	// Check if name is provided
	if name, ok := options["name"].(string); ok && name != "" {
		return name
	}

	// Generate a name based on the sheet name
	// This provides better default names and reduces conflicts
	// E.g., "Dogs" sheet → "DogsTable", "Cats" sheet → "CatsTable"
	baseName := sanitizeTableName(sheetName)
	tableName := baseName + "Table"

	// Ensure the generated name is valid
	// If sheet name is empty or invalid, fall back to "Table1"
	if tableName == "Table" || len(tableName) > 255 {
		tableName = "Table1"
	}

	return tableName
}

// sanitizeTableName sanitizes a string for use in table names
func sanitizeTableName(name string) string {
	// Remove spaces and special characters, keep only alphanumeric
	result := ""
	for _, char := range name {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result += string(char)
		}
	}

	// If result is empty or starts with digit, add prefix
	if len(result) == 0 {
		return "Table"
	}
	if result[0] >= '0' && result[0] <= '9' {
		result = "Table" + result
	}

	return result
}

// containsAny checks if a string contains any of the given substrings
func containsAny(str string, substrs []string) bool {
	for _, substr := range substrs {
		if len(substr) > 0 && len(str) >= len(substr) {
			for i := 0; i <= len(str)-len(substr); i++ {
				if str[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// validateTableName validates the table name
func validateTableName(name string) error {
	if name == "" {
		return &ValidationError{
			Field:   "name",
			Value:   name,
			Message: "table name cannot be empty",
		}
	}

	// Excel table name restrictions:
	// - Cannot start with a digit
	// - Cannot contain spaces or special characters
	// - Maximum 255 characters

	if len(name) > 255 {
		return &ValidationError{
			Field:   "name",
			Value:   name,
			Message: "table name cannot exceed 255 characters",
		}
	}

	// Check first character
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		return &ValidationError{
			Field:   "name",
			Value:   name,
			Message: "table name cannot start with a digit",
		}
	}

	// Check for spaces (Excel doesn't allow spaces in table names)
	for _, char := range name {
		if char == ' ' {
			return &ValidationError{
				Field:   "name",
				Value:   name,
				Message: "table name cannot contain spaces",
			}
		}
	}

	return nil
}

// buildTableConfig constructs an Excelize table configuration
func buildTableConfig(tableRange, tableName string, options map[string]any) *excelize.Table {
	config := &excelize.Table{
		Range: tableRange,
		Name:  tableName,
	}

	// Set table style
	if style, ok := options["style"].(string); ok && style != "" {
		config.StyleName = style
	} else {
		// Default table style
		config.StyleName = "TableStyleMedium9"
	}

	// Show header row
	showHeader := true
	if showHeaderOpt, ok := options["show_header"].(bool); ok {
		showHeader = showHeaderOpt
	}
	config.ShowHeaderRow = &showHeader

	// Show totals row (note: not all table styles support totals row)
	// Excelize may not have ShowTotalRow field, so we skip this for now
	// If needed, this can be added through additional configuration

	return config
}
