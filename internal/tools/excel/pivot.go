package excel

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleCreatePivotTable creates a pivot table in the worksheet
func handleCreatePivotTable(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
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
	}).Info("Creating pivot table in worksheet")

	// Validate required parameters
	sourceRange, ok := options["source_range"].(string)
	if !ok || sourceRange == "" {
		return nil, &ValidationError{
			Field:   "source_range",
			Value:   options["source_range"],
			Message: "source_range parameter is required (e.g., 'A1:D100')",
		}
	}

	rowFields, ok := options["row_fields"].([]any)
	if !ok || len(rowFields) == 0 {
		return nil, &ValidationError{
			Field:   "row_fields",
			Value:   options["row_fields"],
			Message: "row_fields parameter is required and must be a non-empty array",
		}
	}

	dataFields, ok := options["data_fields"].([]any)
	if !ok || len(dataFields) == 0 {
		return nil, &ValidationError{
			Field:   "data_fields",
			Value:   options["data_fields"],
			Message: "data_fields parameter is required and must be a non-empty array",
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
			Operation: "create_pivot_table",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Determine destination sheet and cell
	destination := getDestinationConfig(options)
	destSheet := destination["sheet"].(string)
	destCell := destination["cell"].(string)

	// Create destination sheet if it doesn't exist
	if destSheet != sheetName {
		sheetIdx, _ := f.GetSheetIndex(destSheet)
		if sheetIdx < 0 {
			// Sheet doesn't exist, create it
			if _, err := f.NewSheet(destSheet); err != nil {
				return nil, &SheetError{
					Operation: "create_pivot_table",
					SheetName: destSheet,
					Cause:     fmt.Errorf("failed to create destination sheet: %w", err),
				}
			}
		}
	}

	// Build pivot table configuration
	pivotConfig := buildPivotTableConfig(sheetName, sourceRange, destSheet, destCell, rowFields, dataFields, options)

	// Add pivot table to worksheet
	if err := f.AddPivotTable(pivotConfig); err != nil {
		return nil, &ChartError{ // Reusing ChartError for pivot tables
			Operation: "create_pivot_table",
			ChartType: "pivot_table",
			Cause:     fmt.Errorf("failed to create pivot table: %w", err),
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
}

// getDestinationConfig extracts or generates destination configuration
func getDestinationConfig(options map[string]any) map[string]any {
	// Check for destination configuration
	if destination, ok := options["destination"].(map[string]any); ok {
		sheet, hasSheet := destination["sheet"].(string)
		cell, hasCell := destination["cell"].(string)

		if hasSheet && hasCell && sheet != "" && cell != "" {
			return map[string]any{
				"sheet": sheet,
				"cell":  cell,
			}
		}
	}

	// Default destination: new sheet named "Pivot1" at cell A1
	return map[string]any{
		"sheet": "Pivot1",
		"cell":  "A1",
	}
}

// buildPivotTableConfig constructs an Excelize pivot table configuration
func buildPivotTableConfig(sourceSheet, sourceRange, destSheet, destCell string, rowFields, dataFields []any, options map[string]any) *excelize.PivotTableOptions {
	// PivotTableRange needs to be a range (e.g., "Sheet!A1:B2"), not just a cell
	// We'll create a small range starting from the destination cell
	pivotRange := fmt.Sprintf("%s!%s:%s", destSheet, destCell, calculateEndCell(destCell, 1, 1))

	config := &excelize.PivotTableOptions{
		DataRange:       fmt.Sprintf("%s!%s", sourceSheet, sourceRange),
		PivotTableRange: pivotRange,
		Rows:            convertFieldsToExcelizeFormat(rowFields),
		Data:            convertDataFieldsToExcelizeFormat(dataFields),
	}

	// Add column fields if provided
	if columnFields, ok := options["column_fields"].([]any); ok && len(columnFields) > 0 {
		config.Columns = convertFieldsToExcelizeFormat(columnFields)
	}

	// Add filter fields if provided
	if filterFields, ok := options["filter_fields"].([]any); ok && len(filterFields) > 0 {
		config.Filter = convertFieldsToExcelizeFormat(filterFields)
	}

	// Configure pivot table options
	if pivotOptions, ok := options["options"].(map[string]any); ok {
		// Show grand totals
		if showGrandTotals, ok := pivotOptions["show_grand_totals"].(bool); ok {
			config.RowGrandTotals = showGrandTotals
			config.ColGrandTotals = showGrandTotals
		}

		// Pivot table style
		if style, ok := pivotOptions["style"].(string); ok && style != "" {
			config.PivotTableStyleName = style
		}
	}

	// Set default style if not specified
	if config.PivotTableStyleName == "" {
		config.PivotTableStyleName = "PivotStyleMedium9"
	}

	return config
}

// convertFieldsToExcelizeFormat converts field names to Excelize pivot field format
func convertFieldsToExcelizeFormat(fields []any) []excelize.PivotTableField {
	var excelizeFields []excelize.PivotTableField

	for _, field := range fields {
		fieldStr, ok := field.(string)
		if !ok {
			continue
		}

		excelizeFields = append(excelizeFields, excelize.PivotTableField{
			Data: fieldStr,
		})
	}

	return excelizeFields
}

// convertDataFieldsToExcelizeFormat converts data field configurations to Excelize format
func convertDataFieldsToExcelizeFormat(dataFields []any) []excelize.PivotTableField {
	var excelizeFields []excelize.PivotTableField

	for _, dataField := range dataFields {
		// Check if this is a detailed data field configuration
		if fieldMap, ok := dataField.(map[string]any); ok {
			field := excelize.PivotTableField{}

			// Field name
			if fieldName, ok := fieldMap["field"].(string); ok {
				field.Data = fieldName
			}

			// Custom name for the field
			if customName, ok := fieldMap["name"].(string); ok {
				field.Name = customName
			}

			// Aggregation function
			if function, ok := fieldMap["function"].(string); ok {
				field.Subtotal = mapAggregationFunction(function)
			} else {
				field.Subtotal = "Sum" // Default to sum
			}

			excelizeFields = append(excelizeFields, field)
		} else if fieldStr, ok := dataField.(string); ok {
			// Simple field name, use sum as default
			excelizeFields = append(excelizeFields, excelize.PivotTableField{
				Data:     fieldStr,
				Subtotal: "Sum",
			})
		}
	}

	return excelizeFields
}

// mapAggregationFunction maps user-friendly function names to Excel aggregation functions
func mapAggregationFunction(function string) string {
	functionMap := map[string]string{
		"sum":     "Sum",
		"count":   "Count",
		"average": "Average",
		"avg":     "Average",
		"min":     "Min",
		"max":     "Max",
		"product": "Product",
		"stddev":  "StdDev",
		"var":     "Var",
	}

	if excelFunc, ok := functionMap[function]; ok {
		return excelFunc
	}

	// Default to Sum if function not recognised
	return "Sum"
}

// calculateEndCell calculates an end cell given a start cell and offsets
// For example, calculateEndCell("A1", 2, 3) returns "C4" (2 columns right, 3 rows down)
func calculateEndCell(startCell string, colOffset, rowOffset int) string {
	col, row, err := excelize.CellNameToCoordinates(startCell)
	if err != nil {
		// Fallback to a simple offset
		return "B2"
	}

	endCol := col + colOffset
	endRow := row + rowOffset

	endCell, err := excelize.CoordinatesToCellName(endCol, endRow)
	if err != nil {
		// Fallback
		return "B2"
	}

	return endCell
}
