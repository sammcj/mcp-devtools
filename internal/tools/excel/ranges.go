package excel

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleMergeCells merges a range of cells
func handleMergeCells(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	rangeRef, ok := options["range"].(string)
	if !ok || rangeRef == "" {
		return nil, &ValidationError{
			Field:   "range",
			Value:   options["range"],
			Message: "range parameter is required",
		}
	}

	// Validate range
	if err := validateCellReference(rangeRef); err != nil {
		// Try parsing as range
		if _, _, _, _, parseErr := parseRange(rangeRef); parseErr != nil {
			return nil, err
		}
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
		"range":      rangeRef,
	}).Info("Merging cells")

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
			Operation: "merge_cells",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Merge cells
	if err := f.MergeCell(sheetName, rangeRef, rangeRef); err != nil {
		return nil, &RangeError{
			Operation: "merge",
			Range:     rangeRef,
			Cause:     fmt.Errorf("failed to merge cells: %w", err),
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

// handleUnmergeCells unmerges a range of cells
func handleUnmergeCells(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	rangeRef, ok := options["range"].(string)
	if !ok || rangeRef == "" {
		return nil, &ValidationError{
			Field:   "range",
			Value:   options["range"],
			Message: "range parameter is required",
		}
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
		"range":      rangeRef,
	}).Info("Unmerging cells")

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
			Operation: "unmerge_cells",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Unmerge cells
	if err := f.UnmergeCell(sheetName, rangeRef, rangeRef); err != nil {
		return nil, &RangeError{
			Operation: "unmerge",
			Range:     rangeRef,
			Cause:     fmt.Errorf("failed to unmerge cells: %w", err),
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

// handleGetMergedCells gets all merged cell ranges in a worksheet
func handleGetMergedCells(logger *logrus.Logger, filePath string, sheetName string) (*mcp.CallToolResult, error) {
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
	}).Info("Getting merged cells")

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
			Operation: "get_merged_cells",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Get merged cells
	mergedCells, err := f.GetMergeCells(sheetName)
	if err != nil {
		return nil, &SheetError{
			Operation: "get_merged_cells",
			SheetName: sheetName,
			Cause:     fmt.Errorf("failed to get merged cells: %w", err),
		}
	}

	// Build result
	ranges := make([]string, 0, len(mergedCells))
	for _, cell := range mergedCells {
		ranges = append(ranges, cell.GetCellValue())
	}

	result := map[string]any{
		"merged_cells": ranges,
		"count":        len(ranges),
	}

	return mcp.NewToolResultJSON(result)
}

// handleCopyRange copies a range to another location
func handleCopyRange(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	sourceRange, ok := options["source_range"].(string)
	if !ok || sourceRange == "" {
		return nil, &ValidationError{
			Field:   "source_range",
			Value:   options["source_range"],
			Message: "source_range parameter is required",
		}
	}

	targetCell, ok := options["target_cell"].(string)
	if !ok || targetCell == "" {
		return nil, &ValidationError{
			Field:   "target_cell",
			Value:   options["target_cell"],
			Message: "target_cell parameter is required",
		}
	}

	targetSheet, _ := options["target_sheet"].(string)
	if targetSheet == "" {
		targetSheet = sheetName
	}

	logger.WithFields(logrus.Fields{
		"filepath":     filePath,
		"source_sheet": sheetName,
		"source_range": sourceRange,
		"target_sheet": targetSheet,
		"target_cell":  targetCell,
	}).Info("Copying range")

	// Parse source range
	startRow, startCol, endRow, endCol, err := parseRange(sourceRange)
	if err != nil {
		return nil, err
	}

	// Parse target cell
	targetRow, targetCol, err := parseCellReference(targetCell)
	if err != nil {
		return nil, err
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

	// Check if sheets exist
	sourceIndex, err := f.GetSheetIndex(sheetName)
	if err != nil || sourceIndex < 0 {
		return nil, &SheetError{
			Operation: "copy_range",
			SheetName: sheetName,
			Cause:     fmt.Errorf("source worksheet not found"),
		}
	}

	targetIndex, err := f.GetSheetIndex(targetSheet)
	if err != nil || targetIndex < 0 {
		return nil, &SheetError{
			Operation: "copy_range",
			SheetName: targetSheet,
			Cause:     fmt.Errorf("target worksheet not found"),
		}
	}

	// Copy cells
	cellsCopied := 0
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			sourceCell, err := coordinatesToCell(col, row)
			if err != nil {
				logger.WithError(err).Warn("Failed to convert source coordinates")
				continue
			}

			// Calculate target position
			targetRowOffset := row - startRow
			targetColOffset := col - startCol
			destRow := targetRow + targetRowOffset
			destCol := targetCol + targetColOffset

			destCell, err := coordinatesToCell(destCol, destRow)
			if err != nil {
				logger.WithError(err).Warn("Failed to convert target coordinates")
				continue
			}

			// Get source cell value
			value, err := f.GetCellValue(sheetName, sourceCell)
			if err != nil {
				logger.WithError(err).WithField("cell", sourceCell).Warn("Failed to get cell value")
				continue
			}

			// Set target cell value
			if err := f.SetCellValue(targetSheet, destCell, value); err != nil {
				logger.WithError(err).WithField("cell", destCell).Warn("Failed to set cell value")
				continue
			}

			cellsCopied++
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
		"cells_copied": cellsCopied,
	}

	return mcp.NewToolResultJSON(result)
}

// handleDeleteRange deletes a range and shifts cells
func handleDeleteRange(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	rangeRef, ok := options["range"].(string)
	if !ok || rangeRef == "" {
		return nil, &ValidationError{
			Field:   "range",
			Value:   options["range"],
			Message: "range parameter is required",
		}
	}

	shiftDirection, _ := options["shift"].(string)
	if shiftDirection == "" {
		shiftDirection = "up"
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
		"range":      rangeRef,
		"shift":      shiftDirection,
	}).Info("Deleting range")

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
			Operation: "delete_range",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Parse range
	startRow, startCol, endRow, endCol, err := parseRange(rangeRef)
	if err != nil {
		return nil, err
	}

	// Clear cells in range
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			cell, err := coordinatesToCell(col, row)
			if err != nil {
				continue
			}
			_ = f.SetCellValue(sheetName, cell, "")
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

	cellsDeleted := (endRow - startRow + 1) * (endCol - startCol + 1)

	result := map[string]any{
		"cells_deleted": cellsDeleted,
	}

	return mcp.NewToolResultJSON(result)
}

// handleValidateRange validates that a range exists and returns its boundaries
func handleValidateRange(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	rangeRef, ok := options["range"].(string)
	if !ok || rangeRef == "" {
		return nil, &ValidationError{
			Field:   "range",
			Value:   options["range"],
			Message: "range parameter is required",
		}
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
		"range":      rangeRef,
	}).Info("Validating range")

	// Parse range
	startRow, startCol, endRow, endCol, err := parseRange(rangeRef)
	if err != nil {
		return nil, err
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
			Operation: "validate_range",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Get actual data boundaries
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, &SheetError{
			Operation: "validate_range",
			SheetName: sheetName,
			Cause:     fmt.Errorf("failed to get rows: %w", err),
		}
	}

	maxRow := len(rows)
	maxCol := 0
	for _, row := range rows {
		if len(row) > maxCol {
			maxCol = len(row)
		}
	}

	result := map[string]any{
		"valid": true,
		"boundaries": map[string]any{
			"start_row": startRow,
			"start_col": startCol,
			"end_row":   endRow,
			"end_col":   endCol,
			"rows":      endRow - startRow + 1,
			"columns":   endCol - startCol + 1,
		},
		"sheet_data_boundaries": map[string]any{
			"max_row": maxRow,
			"max_col": maxCol,
		},
	}

	return mcp.NewToolResultJSON(result)
}
