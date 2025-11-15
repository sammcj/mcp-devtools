package excel

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleGetDataValidationInfo retrieves data validation rules from a worksheet
func handleGetDataValidationInfo(logger *logrus.Logger, filePath string, sheetName string) (*mcp.CallToolResult, error) {
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
	}).Info("Getting data validation info")

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
			Operation: "get_data_validation_info",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Get data validation rules
	validations, err := f.GetDataValidations(sheetName)
	if err != nil {
		return nil, &SheetError{
			Operation: "get_data_validation_info",
			SheetName: sheetName,
			Cause:     fmt.Errorf("failed to get data validations: %w", err),
		}
	}

	// Build validation rules array
	validationRules := make([]map[string]any, 0, len(validations))

	for _, validation := range validations {
		if validation == nil {
			continue
		}

		rule := map[string]any{
			"range":    validation.Sqref,
			"type":     validation.Type,
			"operator": validation.Operator,
		}

		// Add formula values (min/max for numeric, source for list)
		if validation.Formula1 != "" {
			if validation.Type == "list" {
				rule["source"] = validation.Formula1
			} else {
				rule["minimum"] = validation.Formula1
			}
		}

		if validation.Formula2 != "" {
			rule["maximum"] = validation.Formula2
		}

		// Add prompt information
		if validation.ShowInputMessage {
			prompt := map[string]any{
				"show": true,
			}
			if validation.PromptTitle != nil {
				prompt["title"] = *validation.PromptTitle
			}
			if validation.Prompt != nil {
				prompt["message"] = *validation.Prompt
			}
			rule["prompt"] = prompt
		}

		// Add error information
		if validation.ShowErrorMessage {
			errorInfo := map[string]any{
				"show":  true,
				"style": validation.ErrorStyle,
			}
			if validation.ErrorTitle != nil {
				errorInfo["title"] = *validation.ErrorTitle
			}
			if validation.Error != nil {
				errorInfo["message"] = *validation.Error
			}
			rule["error"] = errorInfo
		}

		validationRules = append(validationRules, rule)
	}

	result := map[string]any{
		"validation_rules": validationRules,
	}

	return mcp.NewToolResultJSON(result)
}
