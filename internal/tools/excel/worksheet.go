package excel

import (
	"context"
	"fmt"
	"slices"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleCreateWorksheet adds a new worksheet to an existing workbook
func handleCreateWorksheet(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	// Validate worksheet name
	if err := validateWorksheetName(sheetName); err != nil {
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
	}).Info("Creating worksheet")

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

	// Check if sheet already exists
	sheetList := f.GetSheetList()
	if slices.Contains(sheetList, sheetName) {
		return nil, &SheetError{
			Operation: "create",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet already exists"),
		}
	}

	// Create new worksheet
	_, err = f.NewSheet(sheetName)
	if err != nil {
		return nil, &SheetError{
			Operation: "create",
			SheetName: sheetName,
			Cause:     fmt.Errorf("failed to create worksheet: %w", err),
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

// handleCopyWorksheet creates a copy of an existing worksheet
func handleCopyWorksheet(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	targetName, ok := options["target_name"].(string)
	if !ok || targetName == "" {
		return nil, &ValidationError{
			Field:   "target_name",
			Value:   options["target_name"],
			Message: "target_name parameter is required",
		}
	}

	// Validate target worksheet name
	if err := validateWorksheetName(targetName); err != nil {
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"filepath":     filePath,
		"source_sheet": sheetName,
		"target_sheet": targetName,
	}).Info("Copying worksheet")

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

	// Check if source sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "copy",
			SheetName: sheetName,
			Cause:     fmt.Errorf("source worksheet not found"),
		}
	}

	// Check if target sheet already exists
	sheetList := f.GetSheetList()
	if slices.Contains(sheetList, targetName) {
		return nil, &SheetError{
			Operation: "copy",
			SheetName: targetName,
			Cause:     fmt.Errorf("target worksheet already exists"),
		}
	}

	// Copy worksheet
	_, err = f.NewSheet(targetName)
	if err != nil {
		return nil, &SheetError{
			Operation: "copy",
			SheetName: targetName,
			Cause:     fmt.Errorf("failed to create target worksheet: %w", err),
		}
	}

	// Copy all rows from source to target
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, &SheetError{
			Operation: "copy",
			SheetName: sheetName,
			Cause:     fmt.Errorf("failed to read source worksheet: %w", err),
		}
	}

	for rowIndex, row := range rows {
		for colIndex, cellValue := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				logger.WithError(err).Warn("Failed to convert coordinates")
				continue
			}
			if err := f.SetCellValue(targetName, cell, cellValue); err != nil {
				logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell value")
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
}

// handleDeleteWorksheet removes a worksheet from the workbook
func handleDeleteWorksheet(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string) (*mcp.CallToolResult, error) {
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
	}).Info("Deleting worksheet")

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
			Operation: "delete",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Prevent deletion of last sheet
	sheetList := f.GetSheetList()
	if len(sheetList) <= 1 {
		return nil, &SheetError{
			Operation: "delete",
			SheetName: sheetName,
			Cause:     fmt.Errorf("cannot delete the last remaining worksheet"),
		}
	}

	// Delete worksheet
	if err := f.DeleteSheet(sheetName); err != nil {
		logger.WithError(err).WithField("sheet_name", sheetName).Warn("DeleteSheet returned an error, but continuing")
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

// handleRenameWorksheet renames an existing worksheet
func handleRenameWorksheet(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	newName, ok := options["new_name"].(string)
	if !ok || newName == "" {
		return nil, &ValidationError{
			Field:   "new_name",
			Value:   options["new_name"],
			Message: "new_name parameter is required",
		}
	}

	// Validate new worksheet name
	if err := validateWorksheetName(newName); err != nil {
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"filepath": filePath,
		"old_name": sheetName,
		"new_name": newName,
	}).Info("Renaming worksheet")

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

	// Check if source sheet exists
	sheetIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sheetIndex < 0 {
		return nil, &SheetError{
			Operation: "rename",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Check if target name already exists
	sheetList := f.GetSheetList()
	if slices.Contains(sheetList, newName) {
		return nil, &SheetError{
			Operation: "rename",
			SheetName: newName,
			Cause:     fmt.Errorf("worksheet with new name already exists"),
		}
	}

	// Rename worksheet
	if err := f.SetSheetName(sheetName, newName); err != nil {
		return nil, &SheetError{
			Operation: "rename",
			SheetName: sheetName,
			Cause:     fmt.Errorf("failed to rename worksheet: %w", err),
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
