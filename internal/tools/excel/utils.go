package excel

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// Cell reference validation patterns
var (
	cellReferencePattern = regexp.MustCompile(`^[A-Z]+[0-9]+$`)
)

// Excel limits
const (
	MaxRows            = 1048576
	MaxColumns         = 16384
	MaxCellValueLength = 32767 // Maximum characters in a cell value
)

// File permissions
const (
	filePermissions = 0600 // User read/write only
)

// parseCellReference converts a cell reference (e.g., "A1") to row and column numbers (1-based)
func parseCellReference(cell string) (row, col int, err error) {
	col, row, err = excelize.CellNameToCoordinates(cell)
	if err != nil {
		return 0, 0, &ValidationError{
			Field:   "cell_reference",
			Value:   cell,
			Message: fmt.Sprintf("invalid cell reference: %v", err),
		}
	}
	return row, col, nil
}

// coordinatesToCell converts row and column numbers (1-based) to a cell reference (e.g., "A1")
func coordinatesToCell(col, row int) (string, error) {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return "", &ValidationError{
			Field:   "coordinates",
			Value:   fmt.Sprintf("col=%d, row=%d", col, row),
			Message: fmt.Sprintf("invalid coordinates: %v", err),
		}
	}
	return cell, nil
}

// parseRange parses a range reference (e.g., "A1:B10") and returns start and end coordinates (1-based)
func parseRange(rangeStr string) (startRow, startCol, endRow, endCol int, err error) {
	if rangeStr == "" {
		return 0, 0, 0, 0, &ValidationError{
			Field:   "range",
			Value:   rangeStr,
			Message: "range cannot be empty",
		}
	}

	// Handle single cell as a range (e.g., "A1" -> "A1:A1")
	if cellReferencePattern.MatchString(rangeStr) {
		col, row, err := excelize.CellNameToCoordinates(rangeStr)
		if err != nil {
			return 0, 0, 0, 0, &ValidationError{
				Field:   "range",
				Value:   rangeStr,
				Message: fmt.Sprintf("invalid cell reference: %v", err),
			}
		}
		return row, col, row, col, nil
	}

	// Parse range (e.g., "A1:B10")
	parts := strings.Split(rangeStr, ":")
	if len(parts) != 2 {
		return 0, 0, 0, 0, &ValidationError{
			Field:   "range",
			Value:   rangeStr,
			Message: "invalid range format, expected 'A1:B10'",
		}
	}

	// Parse start cell
	startCol, startRow, err = excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return 0, 0, 0, 0, &ValidationError{
			Field:   "range",
			Value:   rangeStr,
			Message: fmt.Sprintf("invalid start cell: %v", err),
		}
	}

	// Parse end cell
	endCol, endRow, err = excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return 0, 0, 0, 0, &ValidationError{
			Field:   "range",
			Value:   rangeStr,
			Message: fmt.Sprintf("invalid end cell: %v", err),
		}
	}

	// Validate range order
	if startRow > endRow || startCol > endCol {
		return 0, 0, 0, 0, &ValidationError{
			Field:   "range",
			Value:   rangeStr,
			Message: "start cell must be before end cell",
		}
	}

	return startRow, startCol, endRow, endCol, nil
}

// validateCellReference validates a cell reference against Excel limits
func validateCellReference(cell string) error {
	if cell == "" {
		return &ValidationError{
			Field:   "cell_reference",
			Value:   cell,
			Message: "cell reference cannot be empty",
		}
	}

	if !cellReferencePattern.MatchString(cell) {
		return &ValidationError{
			Field:   "cell_reference",
			Value:   cell,
			Message: "invalid cell reference format",
		}
	}

	col, row, err := excelize.CellNameToCoordinates(cell)
	if err != nil {
		return &ValidationError{
			Field:   "cell_reference",
			Value:   cell,
			Message: fmt.Sprintf("invalid cell reference: %v", err),
		}
	}

	if row < 1 || row > MaxRows {
		return &ValidationError{
			Field:   "cell_reference",
			Value:   cell,
			Message: fmt.Sprintf("row number must be between 1 and %d", MaxRows),
		}
	}

	if col < 1 || col > MaxColumns {
		return &ValidationError{
			Field:   "cell_reference",
			Value:   cell,
			Message: fmt.Sprintf("column number must be between 1 and %d", MaxColumns),
		}
	}

	return nil
}

// getNumberOption safely extracts a numeric option from the options map
// Handles both float64 (from JSON) and int types
func getNumberOption(options map[string]any, key string) (int, bool) {
	val, exists := options[key]
	if !exists {
		return 0, false
	}

	switch v := val.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

// saveWorkbookWithPermissions saves a workbook and sets secure file permissions
func saveWorkbookWithPermissions(f *excelize.File, filePath string, logger *logrus.Logger) error {
	// Update formula calculations before saving for Numbers compatibility
	// This ensures calculated values are cached in the file
	if err := f.UpdateLinkedValue(); err != nil {
		logger.WithError(err).Debug("Failed to update linked values (non-critical)")
	}

	// Save the file with calculation enabled
	saveOpts := excelize.Options{
		Password: "",
		// Note: ReCal option was removed in Excelize v2.8.0+
		// Formula calculations are now handled by UpdateLinkedValue() above
	}

	if err := f.SaveAs(filePath, saveOpts); err != nil {
		return err
	}

	// Set secure permissions (user read/write only)
	if err := os.Chmod(filePath, filePermissions); err != nil {
		logger.WithError(err).WithField("filepath", filePath).Warn("Failed to set file permissions to 0600")
		// Don't fail the operation, just warn
	}

	return nil
}
