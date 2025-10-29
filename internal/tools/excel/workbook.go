package excel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleCreateWorkbook creates a new Excel workbook
func handleCreateWorkbook(ctx context.Context, logger *logrus.Logger, filePath string, options map[string]any) (*mcp.CallToolResult, error) {
	logger.WithField("filepath", filePath).Info("Creating new workbook")

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return nil, &WorkbookError{
			Operation: "create",
			Path:      filePath,
			Cause:     fmt.Errorf("file already exists"),
		}
	}

	// Create new workbook
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logger.WithError(err).Warn("Failed to close workbook")
		}
	}()

	// Determine which sheet creation mode to use
	var sheetNames []string

	// Check for initial_sheets array (takes precedence)
	if initialSheets, ok := options["initial_sheets"].([]any); ok && len(initialSheets) > 0 {
		// Validate and convert sheet names
		for _, sheetNameAny := range initialSheets {
			sheetName, ok := sheetNameAny.(string)
			if !ok || sheetName == "" {
				return nil, &ValidationError{
					Field:   "initial_sheets",
					Value:   sheetNameAny,
					Message: "all sheet names must be non-empty strings",
				}
			}

			// Validate worksheet name
			if err := validateWorksheetName(sheetName); err != nil {
				return nil, err
			}

			sheetNames = append(sheetNames, sheetName)
		}

		// Rename the default sheet to the first name
		defaultSheet := f.GetSheetName(0)
		if err := f.SetSheetName(defaultSheet, sheetNames[0]); err != nil {
			return nil, &WorkbookError{
				Operation: "create",
				Path:      filePath,
				Cause:     fmt.Errorf("failed to rename initial sheet: %w", err),
			}
		}

		// Create additional sheets
		for i := 1; i < len(sheetNames); i++ {
			if _, err := f.NewSheet(sheetNames[i]); err != nil {
				return nil, &WorkbookError{
					Operation: "create",
					Path:      filePath,
					Cause:     fmt.Errorf("failed to create sheet '%s': %w", sheetNames[i], err),
				}
			}
		}
	} else {
		// Fallback to single sheet name
		initialSheetName, _ := options["initial_sheet_name"].(string)
		if initialSheetName != "" {
			// Validate worksheet name
			if err := validateWorksheetName(initialSheetName); err != nil {
				return nil, err
			}

			// Rename default sheet
			defaultSheet := f.GetSheetName(0)
			if err := f.SetSheetName(defaultSheet, initialSheetName); err != nil {
				return nil, &WorkbookError{
					Operation: "create",
					Path:      filePath,
					Cause:     fmt.Errorf("failed to rename initial sheet: %w", err),
				}
			}
			sheetNames = []string{initialSheetName}
		} else {
			// Use default sheet name
			sheetNames = []string{f.GetSheetName(0)}
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, &WorkbookError{
				Operation: "create",
				Path:      filePath,
				Cause:     fmt.Errorf("failed to create directory: %w", err),
			}
		}
	}

	// Save workbook with secure permissions
	if err := f.SaveAs(filePath, excelize.Options{Password: ""}); err != nil {
		return nil, &WorkbookError{
			Operation: "create",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to save workbook: %w", err),
		}
	}

	// Set file permissions
	if err := os.Chmod(filePath, 0600); err != nil {
		logger.WithError(err).Warn("Failed to set file permissions")
	}

	result := map[string]any{
		"sheets": sheetNames,
	}

	return mcp.NewToolResultJSON(result)
}

// handleGetWorkbookMetadata retrieves metadata about a workbook
func handleGetWorkbookMetadata(ctx context.Context, logger *logrus.Logger, filePath string, options map[string]any) (*mcp.CallToolResult, error) {
	logger.WithField("filepath", filePath).Info("Getting workbook metadata")

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, &WorkbookError{
			Operation: "get_metadata",
			Path:      filePath,
			Cause:     fmt.Errorf("file not found: %w", err),
		}
	}

	// Open workbook
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, &WorkbookError{
			Operation: "get_metadata",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to open workbook: %w", err),
		}
	}
	defer func() {
		if err := f.Close(); err != nil {
			logger.WithError(err).Warn("Failed to close workbook")
		}
	}()

	// Get worksheet names
	sheetList := f.GetSheetList()

	// Build metadata
	metadata := map[string]any{
		"file_size":     fileInfo.Size(),
		"modified_date": fileInfo.ModTime().Format(time.RFC3339),
		"sheet_count":   len(sheetList),
		"sheet_names":   sheetList,
	}

	// Include data ranges if requested
	includeRanges, _ := options["include_ranges"].(bool)
	if includeRanges {
		ranges := make(map[string]any)
		for _, sheetName := range sheetList {
			// Get used range for this sheet
			rows, err := f.GetRows(sheetName)
			if err != nil {
				logger.WithError(err).WithField("sheet", sheetName).Warn("Failed to get rows")
				continue
			}

			if len(rows) > 0 {
				maxRow := len(rows)
				maxCol := 0
				for _, row := range rows {
					if len(row) > maxCol {
						maxCol = len(row)
					}
				}

				if maxRow > 0 && maxCol > 0 {
					// Convert to cell reference
					endCell, err := excelize.CoordinatesToCellName(maxCol, maxRow)
					if err != nil {
						logger.WithError(err).Warn("Failed to convert coordinates")
						continue
					}

					ranges[sheetName] = map[string]any{
						"range":   fmt.Sprintf("A1:%s", endCell),
						"rows":    maxRow,
						"columns": maxCol,
					}
				}
			}
		}
		metadata["data_ranges"] = ranges
	}

	// Convert to JSON for readable output
	resultJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, &WorkbookError{
			Operation: "get_metadata",
			Path:      filePath,
			Cause:     fmt.Errorf("failed to marshal metadata: %w", err),
		}
	}

	result := map[string]any{
		"metadata": string(resultJSON),
	}

	return mcp.NewToolResultJSON(result)
}

// validateWorksheetName validates a worksheet name according to Excel rules
func validateWorksheetName(name string) error {
	if name == "" {
		return &ValidationError{
			Field:   "worksheet_name",
			Value:   name,
			Message: "worksheet name cannot be empty",
		}
	}

	if len(name) > 31 {
		return &ValidationError{
			Field:   "worksheet_name",
			Value:   name,
			Message: "worksheet name cannot exceed 31 characters",
		}
	}

	// Check for invalid characters
	invalidChars := []rune{':', '\\', '/', '?', '*', '[', ']'}
	for _, char := range name {
		if slices.Contains(invalidChars, char) {
			return &ValidationError{
				Field:   "worksheet_name",
				Value:   name,
				Message: fmt.Sprintf("worksheet name cannot contain character '%c'", char),
			}
		}
	}

	return nil
}
