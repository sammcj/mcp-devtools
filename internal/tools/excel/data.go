package excel

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleWriteData writes data to cells in a worksheet
func handleWriteData(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
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
	}).Info("Writing data to worksheet")

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
			Operation: "write_data",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Determine write mode: single cell or range
	cell, hasCell := options["cell"].(string)
	startCell, hasStartCell := options["start_cell"].(string)

	if hasCell {
		// Single cell write
		value := options["value"]
		if value == nil {
			return nil, &ValidationError{
				Field:   "value",
				Value:   value,
				Message: "value parameter is required for single cell write",
			}
		}

		if err := validateCellReference(cell); err != nil {
			return nil, err
		}

		// Check if the value is a formula (string starting with =)
		if strValue, ok := value.(string); ok && len(strValue) > 0 && strValue[0] == '=' {
			// Strip leading = for Numbers compatibility (Excelize v2.10.0+)
			formula := strings.TrimPrefix(strValue, "=")

			// Validate formula length
			if len(formula) > maxFormulaLength {
				return nil, &FormulaError{
					Cell:    cell,
					Formula: formula[:100] + "...",
					Message: fmt.Sprintf("formula exceeds maximum length of %d characters (got %d)", maxFormulaLength, len(formula)),
				}
			}

			// Validate formula safety
			if unsafeFuncs := checkFormulaSafety(formula); len(unsafeFuncs) > 0 {
				return nil, &FormulaError{
					Cell:    cell,
					Formula: formula,
					Message: fmt.Sprintf("formula contains unsafe functions: %v", unsafeFuncs),
				}
			}

			// Check for formula injection risk (warning only)
			if hasFormulaInjectionRisk(formula) {
				logger.WithFields(logrus.Fields{
					"cell":    cell,
					"formula": formula,
				}).Warn("Formula may pose CSV injection risk if file is exported to CSV")
			}

			// Validate cell references are within Excel limits
			if err := validateCellReferencesInFormula(formula); err != nil {
				return nil, &FormulaError{
					Cell:    cell,
					Formula: formula,
					Message: err.Error(),
				}
			}

			// Apply as formula
			if err := f.SetCellFormula(sheetName, cell, formula); err != nil {
				return nil, &FormulaError{
					Cell:    cell,
					Formula: formula,
					Message: fmt.Sprintf("failed to set formula: %v", err),
				}
			}

			// Calculate formula value for Numbers compatibility
			calculatedValue, err := f.CalcCellValue(sheetName, cell)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"cell":    cell,
					"formula": formula,
					"error":   err.Error(),
				}).Warn("Failed to calculate formula value for caching (formula is still set)")
			} else {
				logger.WithFields(logrus.Fields{
					"cell":             cell,
					"formula":          formula,
					"calculated_value": calculatedValue,
				}).Debug("Auto-applied formula from write_data with cached value")
			}
		} else {
			// Regular value - validate length for string values
			if strValue, ok := value.(string); ok && len(strValue) > MaxCellValueLength {
				return nil, &DataError{
					Operation: "write",
					Location:  fmt.Sprintf("sheet '%s', cell '%s'", sheetName, cell),
					Cause:     fmt.Errorf("cell value exceeds maximum length of %d characters (got %d)", MaxCellValueLength, len(strValue)),
				}
			}

			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				return nil, &DataError{
					Operation: "write",
					Location:  fmt.Sprintf("sheet '%s', cell '%s'", sheetName, cell),
					Cause:     fmt.Errorf("failed to set cell value: %w", err),
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

		result := map[string]any{}

		return mcp.NewToolResultJSON(result)

	} else if hasStartCell {
		// Range write
		data, ok := options["data"].([]any)
		if !ok || len(data) == 0 {
			return nil, &ValidationError{
				Field:   "data",
				Value:   options["data"],
				Message: "data parameter is required and must be a non-empty array for range write",
			}
		}

		if err := validateCellReference(startCell); err != nil {
			return nil, err
		}

		// Parse start cell
		startRow, startCol, err := parseCellReference(startCell)
		if err != nil {
			return nil, err
		}

		// Write data row by row
		cellsWritten := 0
		for rowOffset, rowData := range data {
			row, ok := rowData.([]any)
			if !ok {
				// Single value in row, convert to array
				row = []any{rowData}
			}

			for colOffset, cellValue := range row {
				targetRow := startRow + rowOffset
				targetCol := startCol + colOffset

				// Check Excel limits
				if targetRow > MaxRows {
					logger.WithField("row", targetRow).Warn("Row exceeds Excel limit, stopping write")
					break
				}
				if targetCol > MaxColumns {
					logger.WithField("col", targetCol).Warn("Column exceeds Excel limit, skipping")
					continue
				}

				cell, err := coordinatesToCell(targetCol, targetRow)
				if err != nil {
					logger.WithError(err).WithFields(logrus.Fields{
						"row": targetRow,
						"col": targetCol,
					}).Warn("Failed to convert coordinates")
					continue
				}

				// Check if the value is a formula (string starting with =)
				if strValue, ok := cellValue.(string); ok && len(strValue) > 0 && strValue[0] == '=' {
					// Strip leading = for Numbers compatibility (Excelize v2.10.0+)
					formula := strings.TrimPrefix(strValue, "=")

					// Validate formula length
					if len(formula) > maxFormulaLength {
						logger.WithFields(logrus.Fields{
							"cell":           cell,
							"formula_length": len(formula),
						}).Warn("Formula exceeds maximum length, writing as literal text")
						if err := f.SetCellValue(sheetName, cell, strValue); err != nil {
							logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell value")
						}
						continue
					}

					// Validate formula safety
					if unsafeFuncs := checkFormulaSafety(formula); len(unsafeFuncs) > 0 {
						logger.WithFields(logrus.Fields{
							"cell":             cell,
							"formula":          formula,
							"unsafe_functions": unsafeFuncs,
						}).Warn("Skipping unsafe formula in write_data")
						// Write as literal text with warning
						if err := f.SetCellValue(sheetName, cell, strValue); err != nil {
							logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell value")
						}
						continue
					}

					// Check for formula injection risk (warning only)
					if hasFormulaInjectionRisk(formula) {
						logger.WithFields(logrus.Fields{
							"cell":    cell,
							"formula": formula,
						}).Debug("Formula may pose CSV injection risk if file is exported to CSV")
					}

					// Validate cell references are within Excel limits
					if err := validateCellReferencesInFormula(formula); err != nil {
						logger.WithError(err).WithFields(logrus.Fields{
							"cell":    cell,
							"formula": formula,
						}).Warn("Formula contains invalid cell references, writing as literal text")
						if err := f.SetCellValue(sheetName, cell, strValue); err != nil {
							logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell value")
						}
						continue
					}

					// Apply as formula
					if err := f.SetCellFormula(sheetName, cell, formula); err != nil {
						logger.WithError(err).WithFields(logrus.Fields{
							"cell":    cell,
							"formula": formula,
						}).Warn("Failed to set formula, writing as literal text")
						// Fallback to writing as text
						if err := f.SetCellValue(sheetName, cell, strValue); err != nil {
							logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell value")
						}
					} else {
						// Calculate formula value for Numbers compatibility
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
							}).Debug("Auto-applied formula from write_data with cached value")
						}
					}
				} else {
					// Regular value - validate length for string values
					if strValue, ok := cellValue.(string); ok && len(strValue) > MaxCellValueLength {
						logger.WithFields(logrus.Fields{
							"cell":         cell,
							"value_length": len(strValue),
						}).Warn("Cell value exceeds maximum length, truncating")
						// Truncate to max length
						cellValue = strValue[:MaxCellValueLength]
					}

					if err := f.SetCellValue(sheetName, cell, cellValue); err != nil {
						logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell value")
						continue
					}
				}

				cellsWritten++
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
			"cells_written": cellsWritten,
		}

		return mcp.NewToolResultJSON(result)

	} else {
		return nil, &ValidationError{
			Field:   "cell or start_cell",
			Value:   nil,
			Message: "either 'cell' (for single cell) or 'start_cell' (for range) parameter is required",
		}
	}
}

// handleReadData reads data from a range in a worksheet
func handleReadData(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
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
	}).Info("Reading data from worksheet")

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
			Operation: "read_data",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Get range parameters
	startCell, hasStartCell := options["start_cell"].(string)
	endCell, hasEndCell := options["end_cell"].(string)
	cell, hasCell := options["cell"].(string)

	var data [][]any
	var rangeStr string

	if hasCell {
		// Single cell read
		if err := validateCellReference(cell); err != nil {
			return nil, err
		}

		value, err := f.GetCellValue(sheetName, cell)
		if err != nil {
			return nil, &DataError{
				Operation: "read",
				Location:  fmt.Sprintf("sheet '%s', cell '%s'", sheetName, cell),
				Cause:     fmt.Errorf("failed to read cell value: %w", err),
			}
		}

		data = [][]any{{value}}
		rangeStr = cell

	} else if hasStartCell {
		// Range read
		if err := validateCellReference(startCell); err != nil {
			return nil, err
		}

		var startRow, startCol, endRow, endCol int
		startRow, startCol, err = parseCellReference(startCell)
		if err != nil {
			return nil, err
		}

		if hasEndCell {
			// Explicit end cell
			if err := validateCellReference(endCell); err != nil {
				return nil, err
			}

			endRow, endCol, err = parseCellReference(endCell)
			if err != nil {
				return nil, err
			}

			rangeStr = fmt.Sprintf("%s:%s", startCell, endCell)
		} else {
			// Auto-detect range from start cell to used area
			rows, err := f.GetRows(sheetName)
			if err != nil {
				return nil, &SheetError{
					Operation: "read_data",
					SheetName: sheetName,
					Cause:     fmt.Errorf("failed to get rows: %w", err),
				}
			}

			if len(rows) == 0 {
				data = [][]any{}
				rangeStr = startCell
			} else {
				endRow = len(rows)
				endCol = 0
				for _, row := range rows {
					if len(row) > endCol {
						endCol = len(row)
					}
				}

				// Ensure we don't go beyond start cell
				if endRow < startRow {
					endRow = startRow
				}
				if endCol < startCol {
					endCol = startCol
				}

				endCellStr, _ := coordinatesToCell(endCol, endRow)
				rangeStr = fmt.Sprintf("%s:%s", startCell, endCellStr)
			}
		}

		// Read data from range
		if len(data) == 0 {
			for row := startRow; row <= endRow; row++ {
				rowData := make([]any, 0, endCol-startCol+1)
				for col := startCol; col <= endCol; col++ {
					cell, err := coordinatesToCell(col, row)
					if err != nil {
						logger.WithError(err).WithFields(logrus.Fields{
							"row": row,
							"col": col,
						}).Warn("Failed to convert coordinates")
						rowData = append(rowData, "")
						continue
					}

					value, err := f.GetCellValue(sheetName, cell)
					if err != nil {
						logger.WithError(err).WithField("cell", cell).Warn("Failed to get cell value")
						rowData = append(rowData, "")
						continue
					}

					rowData = append(rowData, value)
				}
				data = append(data, rowData)
			}
		}

	} else {
		// Read all data
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, &SheetError{
				Operation: "read_data",
				SheetName: sheetName,
				Cause:     fmt.Errorf("failed to get rows: %w", err),
			}
		}

		// Convert to [][]any
		for _, row := range rows {
			rowData := make([]any, len(row))
			for i, cell := range row {
				rowData[i] = cell
			}
			data = append(data, rowData)
		}

		if len(data) > 0 {
			maxCols := 0
			for _, row := range data {
				if len(row) > maxCols {
					maxCols = len(row)
				}
			}
			endCellStr, _ := coordinatesToCell(maxCols, len(data))
			rangeStr = fmt.Sprintf("A1:%s", endCellStr)
		} else {
			rangeStr = "A1"
		}
	}

	// Calculate dimensions
	rows := len(data)
	cols := 0
	if rows > 0 {
		cols = len(data[0])
	}

	result := map[string]any{
		"range": rangeStr,
		"data":  data,
		"dimensions": map[string]any{
			"rows":    rows,
			"columns": cols,
		},
	}

	return mcp.NewToolResultJSON(result)
}

// handleReadDataWithMetadata reads data with validation information
func handleReadDataWithMetadata(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
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
	}).Info("Reading data with metadata from worksheet")

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
			Operation: "read_data_with_metadata",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Get range parameters
	startCell, hasStartCell := options["start_cell"].(string)
	endCell, hasEndCell := options["end_cell"].(string)

	var startRow, startCol, endRow, endCol int

	if !hasStartCell {
		startCell = "A1"
	}

	if err := validateCellReference(startCell); err != nil {
		return nil, err
	}

	startRow, startCol, err = parseCellReference(startCell)
	if err != nil {
		return nil, err
	}

	if hasEndCell {
		if err := validateCellReference(endCell); err != nil {
			return nil, err
		}

		endRow, endCol, err = parseCellReference(endCell)
		if err != nil {
			return nil, err
		}
	} else {
		// Auto-detect range
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, &SheetError{
				Operation: "read_data_with_metadata",
				SheetName: sheetName,
				Cause:     fmt.Errorf("failed to get rows: %w", err),
			}
		}

		if len(rows) == 0 {
			endRow = startRow
			endCol = startCol
		} else {
			endRow = len(rows)
			endCol = 0
			for _, row := range rows {
				if len(row) > endCol {
					endCol = len(row)
				}
			}
		}
	}

	// Get data validation rules for the sheet
	validationRules, err := f.GetDataValidations(sheetName)
	if err != nil {
		logger.WithError(err).Warn("Failed to get data validation rules")
		validationRules = nil
	}

	// Build cells array with metadata
	cells := make([]map[string]any, 0)

	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			cellRef, err := coordinatesToCell(col, row)
			if err != nil {
				logger.WithError(err).WithFields(logrus.Fields{
					"row": row,
					"col": col,
				}).Warn("Failed to convert coordinates")
				continue
			}

			value, err := f.GetCellValue(sheetName, cellRef)
			if err != nil {
				logger.WithError(err).WithField("cell", cellRef).Warn("Failed to get cell value")
				value = ""
			}

			cellData := map[string]any{
				"address": cellRef,
				"value":   value,
				"row":     row,
				"column":  col,
			}

			// Check if this cell has validation rules
			validation := findValidationForCell(cellRef, validationRules, f, sheetName, logger)
			if validation != nil {
				cellData["validation"] = validation
			} else {
				cellData["validation"] = nil
			}

			cells = append(cells, cellData)
		}
	}

	result := map[string]any{
		"range": fmt.Sprintf("%s:%s", startCell, endCell),
		"cells": cells,
	}

	return mcp.NewToolResultJSON(result)
}

// findValidationForCell finds data validation rules for a specific cell
func findValidationForCell(cellRef string, validations []*excelize.DataValidation, f *excelize.File, sheetName string, logger *logrus.Logger) map[string]any {
	if len(validations) == 0 {
		return nil
	}

	// Check each validation rule to see if it applies to this cell
	for _, validation := range validations {
		if validation == nil {
			continue
		}

		// Check if cell is within validation range
		// validation.Sqref contains the range like "B2:B100"
		sqref := validation.Sqref
		if sqref == "" {
			continue
		}

		// Parse validation range
		startRow, startCol, endRow, endCol, err := parseRange(sqref)
		if err != nil {
			logger.WithError(err).WithField("sqref", sqref).Debug("Failed to parse validation range")
			continue
		}

		// Parse current cell
		cellRow, cellCol, err := parseCellReference(cellRef)
		if err != nil {
			continue
		}

		// Check if cell is within range
		if cellRow >= startRow && cellRow <= endRow && cellCol >= startCol && cellCol <= endCol {
			// Build validation metadata
			validationData := map[string]any{
				"type":     validation.Type,
				"operator": validation.Operator,
			}

			// Get allowed values for list type
			if validation.Type == "list" {
				allowedValues := make([]string, 0)

				// Check if Formula1 is a range reference or a list
				if validation.Formula1 != "" {
					formula := validation.Formula1
					// Check if it's a range reference (e.g., "E1:E3" or "$E$1:$E$3")
					if len(formula) > 0 && (formula[0] == '$' || (formula[0] >= 'A' && formula[0] <= 'Z')) {
						// Try to read values from the referenced range
						cleanFormula := formula
						if cleanFormula[0] == '$' {
							cleanFormula = formula[1:] // Remove leading $
						}

						// Try to parse as range
						_, _, _, _, err := parseRange(cleanFormula)
						if err == nil {
							// It's a range, try to read values
							_, err := f.GetRows(sheetName)
							if err == nil {
								// Parse the range and extract values
								// This is a simplified approach - just add a note
								validationData["source_range"] = formula
							}
						}
					}

					// Parse comma-separated values if not a range
					if _, hasSourceRange := validationData["source_range"]; !hasSourceRange {
						// Split by comma (Excel uses comma for lists)
						values := splitExcelList(formula)
						allowedValues = append(allowedValues, values...)
					}
				}

				if len(allowedValues) > 0 {
					validationData["allowed_values"] = allowedValues
				}
			}

			// Add prompt and error messages if present
			hasPromptTitle := validation.PromptTitle != nil && *validation.PromptTitle != ""
			hasPrompt := validation.Prompt != nil && *validation.Prompt != ""
			if validation.ShowInputMessage && (hasPromptTitle || hasPrompt) {
				prompt := map[string]any{
					"show": true,
				}
				if hasPromptTitle {
					prompt["title"] = *validation.PromptTitle
				} else {
					prompt["title"] = ""
				}
				if hasPrompt {
					prompt["message"] = *validation.Prompt
				} else {
					prompt["message"] = ""
				}
				validationData["prompt"] = prompt
			}

			hasErrorTitle := validation.ErrorTitle != nil && *validation.ErrorTitle != ""
			hasError := validation.Error != nil && *validation.Error != ""
			if validation.ShowErrorMessage && (hasErrorTitle || hasError) {
				errorMsg := map[string]any{
					"style": validation.ErrorStyle,
				}
				if hasErrorTitle {
					errorMsg["title"] = *validation.ErrorTitle
				} else {
					errorMsg["title"] = ""
				}
				if hasError {
					errorMsg["message"] = *validation.Error
				} else {
					errorMsg["message"] = ""
				}
				validationData["error"] = errorMsg
			}

			// Add min/max values for numeric validations
			if validation.Formula1 != "" && validation.Type != "list" {
				validationData["minimum"] = validation.Formula1
			}
			if validation.Formula2 != "" {
				validationData["maximum"] = validation.Formula2
			}

			return validationData
		}
	}

	return nil
}

// splitExcelList splits an Excel list string (comma or quote-delimited)
func splitExcelList(list string) []string {
	if list == "" {
		return nil
	}

	// Remove quotes if present
	if len(list) >= 2 && list[0] == '"' && list[len(list)-1] == '"' {
		list = list[1 : len(list)-1]
	}

	// Split by comma
	parts := []string{}
	current := ""
	for _, char := range list {
		if char == ',' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
