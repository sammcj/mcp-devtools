package excel

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// Dangerous Excel functions that should be blocked for security
var dangerousFunctions = []string{
	// Excel dangerous functions
	"INDIRECT",     // Arbitrary cell references
	"HYPERLINK",    // External resources
	"WEBSERVICE",   // HTTP requests
	"DGET",         // Database queries
	"RTD",          // Real-time data
	"CALL",         // Calls DLL functions
	"REGISTER.ID",  // Accesses external resources
	"GET.WORKBOOK", // Accesses workbook metadata
	// Google Sheets dangerous functions (if file is opened in Google Sheets)
	"IMPORTDATA",  // Imports data from URLs
	"IMPORTXML",   // Fetches XML from URLs
	"IMPORTHTML",  // Fetches HTML from URLs
	"IMPORTFEED",  // Fetches RSS/Atom feeds
	"IMPORTRANGE", // Imports data from other Google Sheets
}

// Maximum formula length (Excel 2019+ supports up to 8192 characters)
const maxFormulaLength = 8192

// handleApplyFormula applies a formula to a cell
func handleApplyFormula(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
	if sheetName == "" {
		return nil, &ValidationError{
			Field:   "sheet_name",
			Value:   sheetName,
			Message: "sheet_name parameter is required",
		}
	}

	cell, ok := options["cell"].(string)
	if !ok || cell == "" {
		return nil, &ValidationError{
			Field:   "cell",
			Value:   options["cell"],
			Message: "cell parameter is required",
		}
	}

	formula, ok := options["formula"].(string)
	if !ok || formula == "" {
		return nil, &ValidationError{
			Field:   "formula",
			Value:   options["formula"],
			Message: "formula parameter is required",
		}
	}

	// Remove leading = if present - Excelize handles this internally
	// for better Apple Numbers compatibility (Excelize v2.10.0+)
	formula = strings.TrimPrefix(formula, "=")

	// Validate formula length
	if len(formula) > maxFormulaLength {
		return nil, &FormulaError{
			Cell:    cell,
			Formula: formula[:100] + "...", // Truncate for error message
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

	// Check for formula injection risk (warning only, don't block)
	if hasFormulaInjectionRisk(formula) {
		logger.WithFields(logrus.Fields{
			"cell":    cell,
			"formula": formula,
		}).Warn("Formula may pose CSV injection risk if file is exported to CSV and opened in spreadsheet software")
	}

	// Validate cell references are within Excel limits
	if err := validateCellReferencesInFormula(formula); err != nil {
		return nil, &FormulaError{
			Cell:    cell,
			Formula: formula,
			Message: err.Error(),
		}
	}

	logger.WithFields(logrus.Fields{
		"filepath":   filePath,
		"sheet_name": sheetName,
		"cell":       cell,
		"formula":    formula,
	}).Info("Applying formula")

	// Validate cell reference
	if err := validateCellReference(cell); err != nil {
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
			Operation: "apply_formula",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Set formula
	if err := f.SetCellFormula(sheetName, cell, formula); err != nil {
		return nil, &FormulaError{
			Cell:    cell,
			Formula: formula,
			Message: fmt.Sprintf("failed to set formula: %v", err),
		}
	}

	// Calculate the formula to cache the result for compatibility with Apple Numbers
	// Numbers doesn't have a formula calculation engine, so it needs cached values
	calculatedValue, err := f.CalcCellValue(sheetName, cell)
	if err != nil {
		// If calculation fails, log a warning but don't fail the operation
		// The formula is still set, just without a cached value
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
		}).Debug("Calculated and cached formula result")
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

// handleValidateFormulaSyntax validates a formula without applying it
func handleValidateFormulaSyntax(ctx context.Context, logger *logrus.Logger, options map[string]any) (*mcp.CallToolResult, error) {
	formula, ok := options["formula"].(string)
	if !ok || formula == "" {
		return nil, &ValidationError{
			Field:   "formula",
			Value:   options["formula"],
			Message: "formula parameter is required",
		}
	}

	// Remove leading = if present - Excelize handles this internally
	// for better Apple Numbers compatibility (Excelize v2.10.0+)
	formula = strings.TrimPrefix(formula, "=")

	logger.WithFields(logrus.Fields{
		"formula": formula,
	}).Info("Validating formula syntax")

	// Check for unsafe functions
	unsafeFuncs := checkFormulaSafety(formula)

	// Basic syntax validation
	valid := true
	message := "Formula syntax is valid"
	var validationErrors []string

	// Check formula length
	if len(formula) > maxFormulaLength {
		valid = false
		validationErrors = append(validationErrors, fmt.Sprintf("formula exceeds maximum length of %d characters (got %d)", maxFormulaLength, len(formula)))
	}

	// Check for balanced parentheses
	if !hasBalancedParentheses(formula) {
		valid = false
		validationErrors = append(validationErrors, "unbalanced parentheses")
	}

	// Check for empty formula
	if strings.TrimSpace(formula) == "" {
		valid = false
		validationErrors = append(validationErrors, "empty formula")
	}

	// Check for unsafe functions
	if len(unsafeFuncs) > 0 {
		valid = false
		validationErrors = append(validationErrors, fmt.Sprintf("contains unsafe functions: %v", unsafeFuncs))
	}

	// Check for cell reference range violations
	if err := validateCellReferencesInFormula(formula); err != nil {
		valid = false
		validationErrors = append(validationErrors, err.Error())
	}

	// Check for formula injection risk (warning only, add to result)
	injectionRisk := hasFormulaInjectionRisk(formula)
	if injectionRisk {
		logger.WithField("formula", formula).Debug("Formula may pose CSV injection risk")
	}

	if !valid {
		message = fmt.Sprintf("Formula validation failed: %s", strings.Join(validationErrors, "; "))
	}

	result := map[string]any{
		"valid":             valid,
		"message":           message,
		"unsafe_functions":  unsafeFuncs,
		"validation_errors": validationErrors,
		"injection_risk":    injectionRisk,
	}

	return mcp.NewToolResultJSON(result)
}

// checkFormulaSafety checks if a formula contains dangerous functions
func checkFormulaSafety(formula string) []string {
	upperFormula := strings.ToUpper(formula)
	foundUnsafe := make([]string, 0)

	for _, dangerousFunc := range dangerousFunctions {
		// Use regex to match function calls
		pattern := fmt.Sprintf(`\b%s\s*\(`, dangerousFunc)
		matched, err := regexp.MatchString(pattern, upperFormula)
		if err == nil && matched {
			foundUnsafe = append(foundUnsafe, dangerousFunc)
		}
	}

	return foundUnsafe
}

// hasBalancedParentheses checks if parentheses are balanced in a formula
func hasBalancedParentheses(formula string) bool {
	count := 0
	for _, char := range formula {
		switch char {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

// hasFormulaInjectionRisk checks if a formula could be dangerous when exported to CSV
func hasFormulaInjectionRisk(formula string) bool {
	if len(formula) == 0 {
		return false
	}
	// Check for CSV formula injection patterns
	// Note: We've already stripped leading = from formulas, but check for other injection chars
	// Formulas starting with +, -, @ can execute when CSV is opened
	// The = check is kept for completeness in case formula preprocessing changes
	firstChar := formula[0]
	return firstChar == '=' || firstChar == '+' || firstChar == '-' || firstChar == '@'
}

// validateCellReferencesInFormula validates that cell references are within Excel limits
func validateCellReferencesInFormula(formula string) error {
	// Pattern to match cell references: A1, $A$1, Sheet1!A1, Sheet1!$A$1
	// Matches column (letters) and row (numbers)
	cellRefPattern := regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(\$?[A-Z]+\$?\d+)(?:[^A-Za-z0-9_]|$)`)
	matches := cellRefPattern.FindAllStringSubmatch(formula, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		cellRef := match[1]
		// Remove $ signs for parsing
		cleanRef := strings.ReplaceAll(cellRef, "$", "")

		// Parse the cell reference
		row, col, err := parseCellReference(cleanRef)
		if err != nil {
			// If we can't parse it, it might be invalid, but let Excel handle it
			continue
		}

		// Check against Excel limits
		if row > MaxRows {
			return fmt.Errorf("cell reference %s exceeds maximum row limit of %d", cellRef, MaxRows)
		}
		if col > MaxColumns {
			return fmt.Errorf("cell reference %s exceeds maximum column limit of %d (column %s)", cellRef, MaxColumns, columnNumberToName(MaxColumns))
		}
	}

	return nil
}

// columnNumberToName converts a column number to a column name (e.g., 1 -> A, 27 -> AA)
func columnNumberToName(col int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+(col%26))) + name
		col /= 26
	}
	return name
}
