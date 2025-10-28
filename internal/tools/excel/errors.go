package excel

import "fmt"

// WorkbookError represents errors related to workbook operations
type WorkbookError struct {
	Operation string
	Path      string
	Cause     error
}

func (e *WorkbookError) Error() string {
	return fmt.Sprintf("workbook error during %s on %s: %v", e.Operation, e.Path, e.Cause)
}

func (e *WorkbookError) Unwrap() error {
	return e.Cause
}

// SheetError represents errors related to worksheet operations
type SheetError struct {
	Operation string
	SheetName string
	Cause     error
}

func (e *SheetError) Error() string {
	return fmt.Sprintf("worksheet error during %s on sheet '%s': %v", e.Operation, e.SheetName, e.Cause)
}

func (e *SheetError) Unwrap() error {
	return e.Cause
}

// DataError represents errors related to data operations
type DataError struct {
	Operation string
	Location  string
	Cause     error
}

func (e *DataError) Error() string {
	return fmt.Sprintf("data error during %s at %s: %v", e.Operation, e.Location, e.Cause)
}

func (e *DataError) Unwrap() error {
	return e.Cause
}

// ValidationError represents validation failures
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s' with value '%v': %s", e.Field, e.Value, e.Message)
}

// FormatError represents formatting errors
type FormatError struct {
	Operation string
	Range     string
	Cause     error
}

func (e *FormatError) Error() string {
	return fmt.Sprintf("formatting error during %s on range '%s': %v", e.Operation, e.Range, e.Cause)
}

func (e *FormatError) Unwrap() error {
	return e.Cause
}

// ChartError represents chart creation errors
type ChartError struct {
	Operation string
	ChartType string
	Cause     error
}

func (e *ChartError) Error() string {
	return fmt.Sprintf("chart error during %s for type '%s': %v", e.Operation, e.ChartType, e.Cause)
}

func (e *ChartError) Unwrap() error {
	return e.Cause
}

// FormulaError represents formula-related errors
type FormulaError struct {
	Cell    string
	Formula string
	Message string
}

func (e *FormulaError) Error() string {
	return fmt.Sprintf("formula error at cell '%s' for formula '%s': %s", e.Cell, e.Formula, e.Message)
}

// RangeError represents range operation errors
type RangeError struct {
	Operation string
	Range     string
	Cause     error
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("range error during %s on range '%s': %v", e.Operation, e.Range, e.Cause)
}

func (e *RangeError) Unwrap() error {
	return e.Cause
}
