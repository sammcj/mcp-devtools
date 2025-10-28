package excel

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleFormatRange applies formatting to a cell range
func handleFormatRange(ctx context.Context, logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
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
	}).Info("Formatting range")

	// Validate range
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
			Operation: "format_range",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Check for conditional formatting
	if conditionalFormat, ok := options["conditional_format"].(map[string]any); ok {
		return applyConditionalFormatting(ctx, logger, f, filePath, sheetName, rangeRef, conditionalFormat)
	}

	// Build style from options
	style := &excelize.Style{}

	// Font properties
	if fontMap, ok := options["font"].(map[string]any); ok {
		font := &excelize.Font{}
		if bold, ok := fontMap["bold"].(bool); ok {
			font.Bold = bold
		}
		if italic, ok := fontMap["italic"].(bool); ok {
			font.Italic = italic
		}
		if underline, ok := fontMap["underline"].(string); ok {
			font.Underline = underline
		}
		if size, ok := fontMap["size"].(float64); ok {
			font.Size = size
		} else if size, ok := fontMap["size"].(int); ok {
			font.Size = float64(size)
		}
		if colour, ok := fontMap["colour"].(string); ok {
			font.Color = normalizeColour(colour)
		} else if color, ok := fontMap["color"].(string); ok {
			font.Color = normalizeColour(color)
		}
		if family, ok := fontMap["family"].(string); ok {
			font.Family = family
		}
		style.Font = font
	}

	// Fill properties
	if fillMap, ok := options["fill"].(map[string]any); ok {
		fill := excelize.Fill{
			Type: "pattern",
		}
		if colour, ok := fillMap["colour"].(string); ok {
			fill.Color = []string{normalizeColour(colour)}
		} else if color, ok := fillMap["color"].(string); ok {
			fill.Color = []string{normalizeColour(color)}
		}
		if pattern, ok := fillMap["pattern"].(string); ok {
			fill.Pattern = getPatternType(pattern)
		} else {
			fill.Pattern = 1 // solid
		}
		style.Fill = fill
	}

	// Border properties
	if borderMap, ok := options["borders"].(map[string]any); ok {
		borders := make([]excelize.Border, 0)

		borderStyle, _ := borderMap["style"].(string)
		if borderStyle == "" {
			borderStyle = "thin"
		}

		borderColour, _ := borderMap["colour"].(string)
		if borderColour == "" {
			borderColour, _ = borderMap["color"].(string)
		}
		if borderColour == "" {
			borderColour = "000000"
		}
		borderColour = normalizeColour(borderColour)

		sides, _ := borderMap["sides"].([]any)
		if len(sides) == 0 {
			// Default to all sides
			sides = []any{"top", "bottom", "left", "right"}
		}

		for _, side := range sides {
			sideStr, ok := side.(string)
			if !ok {
				continue
			}

			border := excelize.Border{
				Type:  sideStr,
				Color: borderColour,
				Style: getBorderStyle(borderStyle),
			}
			borders = append(borders, border)
		}

		style.Border = borders
	}

	// Alignment properties
	if alignMap, ok := options["alignment"].(map[string]any); ok {
		alignment := &excelize.Alignment{}
		if horizontal, ok := alignMap["horizontal"].(string); ok {
			alignment.Horizontal = horizontal
		}
		if vertical, ok := alignMap["vertical"].(string); ok {
			alignment.Vertical = vertical
		}
		if wrapText, ok := alignMap["wrap_text"].(bool); ok {
			alignment.WrapText = wrapText
		}
		if rotation, ok := alignMap["rotation"].(float64); ok {
			alignment.TextRotation = int(rotation)
		} else if rotation, ok := alignMap["rotation"].(int); ok {
			alignment.TextRotation = rotation
		}
		style.Alignment = alignment
	}

	// Number format
	if numFormat, ok := options["number_format"].(string); ok {
		style.CustomNumFmt = &numFormat
	}

	// Protection
	if protectionMap, ok := options["protection"].(map[string]any); ok {
		protection := &excelize.Protection{}
		if locked, ok := protectionMap["locked"].(bool); ok {
			protection.Locked = locked
		}
		if hidden, ok := protectionMap["hidden"].(bool); ok {
			protection.Hidden = hidden
		}
		style.Protection = protection
	}

	// Apply style to range with merging
	// To preserve existing formatting, we need to merge styles for each cell
	cellsFormatted := 0
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			cell, err := coordinatesToCell(col, row)
			if err != nil {
				logger.WithError(err).WithFields(logrus.Fields{
					"row": row,
					"col": col,
				}).Warn("Failed to convert coordinates")
				continue
			}

			// Get existing cell style
			existingStyleID, err := f.GetCellStyle(sheetName, cell)
			if err != nil {
				logger.WithError(err).WithField("cell", cell).Debug("Failed to get existing style, using new style")
				existingStyleID = 0 // Use default style
			}

			// Get existing style definition
			var existingStyle *excelize.Style
			if existingStyleID > 0 {
				existingStyle, err = f.GetStyle(existingStyleID)
				if err != nil {
					logger.WithError(err).WithField("cell", cell).Debug("Failed to get existing style definition")
					existingStyle = &excelize.Style{} // Use empty style
				}
			} else {
				existingStyle = &excelize.Style{}
			}

			// Merge new style with existing style
			mergedStyle := mergeStyles(existingStyle, style)

			// Create merged style
			mergedStyleID, err := f.NewStyle(mergedStyle)
			if err != nil {
				logger.WithError(err).WithField("cell", cell).Warn("Failed to create merged style")
				continue
			}

			// Apply merged style
			if err := f.SetCellStyle(sheetName, cell, cell, mergedStyleID); err != nil {
				logger.WithError(err).WithField("cell", cell).Warn("Failed to set cell style")
				continue
			}

			cellsFormatted++
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
		"cells_formatted": cellsFormatted,
	}

	return mcp.NewToolResultJSON(result)
}

// applyConditionalFormatting applies conditional formatting rules
func applyConditionalFormatting(ctx context.Context, logger *logrus.Logger, f *excelize.File, filePath string, sheetName string, rangeRef string, conditionalFormat map[string]any) (*mcp.CallToolResult, error) {
	formatType, ok := conditionalFormat["type"].(string)
	if !ok {
		return nil, &ValidationError{
			Field:   "conditional_format.type",
			Value:   conditionalFormat["type"],
			Message: "conditional format type is required",
		}
	}

	rule, ok := conditionalFormat["rule"].(map[string]any)
	if !ok {
		return nil, &ValidationError{
			Field:   "conditional_format.rule",
			Value:   conditionalFormat["rule"],
			Message: "conditional format rule is required",
		}
	}

	switch formatType {
	case "colour_scale", "color_scale":
		return applyColourScale(f, filePath, sheetName, rangeRef, rule, logger)
	case "data_bar", "databar":
		return applyDataBar(f, filePath, sheetName, rangeRef, rule, logger)
	case "icon_set", "iconset":
		return applyIconSet(f, filePath, sheetName, rangeRef, rule, logger)
	case "cell_value", "top10", "duplicate", "unique", "formula":
		return applyRuleBasedFormatting(f, filePath, sheetName, rangeRef, formatType, rule, logger)
	default:
		return nil, &ValidationError{
			Field:   "conditional_format.type",
			Value:   formatType,
			Message: fmt.Sprintf("unsupported conditional format type: %s", formatType),
		}
	}
}

// applyColourScale applies colour scale conditional formatting
func applyColourScale(f *excelize.File, filePath string, sheetName string, rangeRef string, rule map[string]any, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	minColour, _ := rule["min_colour"].(string)
	if minColour == "" {
		minColour, _ = rule["min_color"].(string)
	}
	if minColour == "" {
		minColour = "FF0000" // Default red
	}
	minColour = normalizeColour(minColour)

	maxColour, _ := rule["max_colour"].(string)
	if maxColour == "" {
		maxColour, _ = rule["max_color"].(string)
	}
	if maxColour == "" {
		maxColour = "00FF00" // Default green
	}
	maxColour = normalizeColour(maxColour)

	midColour, _ := rule["mid_colour"].(string)
	if midColour == "" {
		midColour, _ = rule["mid_color"].(string)
	}
	if midColour != "" {
		midColour = normalizeColour(midColour)
	}

	format := []excelize.ConditionalFormatOptions{
		{
			Type:     "2_color_scale",
			Criteria: "=",
			MinType:  "min",
			MinColor: minColour,
			MaxType:  "max",
			MaxColor: maxColour,
		},
	}

	// If mid colour is specified, use 3-colour scale
	if midColour != "" {
		format[0].Type = "3_color_scale"
		format[0].MidType = "percentile"
		format[0].MidColor = midColour

		if midValue, ok := rule["mid_value"].(float64); ok {
			format[0].MidValue = fmt.Sprintf("%.0f", midValue)
		} else if midValue, ok := rule["mid_value"].(int); ok {
			format[0].MidValue = fmt.Sprintf("%d", midValue)
		} else {
			format[0].MidValue = "50"
		}
	}

	if err := f.SetConditionalFormat(sheetName, rangeRef, format); err != nil {
		return nil, &FormatError{
			Operation: "conditional_format_colour_scale",
			Range:     rangeRef,
			Cause:     fmt.Errorf("failed to set colour scale: %w", err),
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
		"type": "colour_scale",
	}

	return mcp.NewToolResultJSON(result)
}

// applyDataBar applies data bar conditional formatting
func applyDataBar(f *excelize.File, filePath string, sheetName string, rangeRef string, rule map[string]any, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	barColour, _ := rule["bar_colour"].(string)
	if barColour == "" {
		barColour, _ = rule["bar_color"].(string)
	}
	if barColour == "" {
		barColour = "#638EC6" // Default blue
	}

	showValue := true
	if show, ok := rule["show_value"].(bool); ok {
		showValue = show
	}

	format := []excelize.ConditionalFormatOptions{
		{
			Type:     "data_bar",
			Criteria: "=",
			MinType:  "min",
			MaxType:  "max",
			BarColor: barColour,
		},
	}

	// Set bar direction
	if !showValue {
		// Unfortunately, Excelize doesn't have a direct way to hide values in data bars
		// This would need to be done manually in Excel
		logger.Warn("show_value=false is not fully supported by Excelize")
	}

	if err := f.SetConditionalFormat(sheetName, rangeRef, format); err != nil {
		return nil, &FormatError{
			Operation: "conditional_format_data_bar",
			Range:     rangeRef,
			Cause:     fmt.Errorf("failed to set data bar: %w", err),
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
		"type": "data_bar",
	}

	return mcp.NewToolResultJSON(result)
}

// applyIconSet applies icon set conditional formatting
func applyIconSet(f *excelize.File, filePath string, sheetName string, rangeRef string, rule map[string]any, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	iconStyle, _ := rule["icon_style"].(string)
	if iconStyle == "" {
		iconStyle = "3Arrows" // Default 3 arrows
	}

	reverse := false
	if rev, ok := rule["reverse"].(bool); ok {
		reverse = rev
	}

	format := []excelize.ConditionalFormatOptions{
		{
			Type:         "icon_set",
			IconStyle:    iconStyle,
			ReverseIcons: reverse,
		},
	}

	if err := f.SetConditionalFormat(sheetName, rangeRef, format); err != nil {
		return nil, &FormatError{
			Operation: "conditional_format_icon_set",
			Range:     rangeRef,
			Cause:     fmt.Errorf("failed to set icon set: %w", err),
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
		"type": "icon_set",
	}

	return mcp.NewToolResultJSON(result)
}

// applyRuleBasedFormatting applies rule-based conditional formatting
func applyRuleBasedFormatting(f *excelize.File, filePath string, sheetName string, rangeRef string, formatType string, rule map[string]any, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	format := excelize.ConditionalFormatOptions{
		Type: formatType,
	}

	// Get criteria/operator
	if criteria, ok := rule["criteria"].(string); ok {
		format.Criteria = criteria
	} else if operator, ok := rule["operator"].(string); ok {
		format.Criteria = operator
	}

	// Get value(s)
	if value, ok := rule["value"].(string); ok {
		format.Value = value
	} else if value, ok := rule["value"].(float64); ok {
		format.Value = fmt.Sprintf("%.2f", value)
	} else if value, ok := rule["value"].(int); ok {
		format.Value = fmt.Sprintf("%d", value)
	}

	// Get format style
	if formatStyle, ok := rule["format"].(map[string]any); ok {
		// Build format style
		style := &excelize.Style{}

		if fontMap, ok := formatStyle["font"].(map[string]any); ok {
			font := &excelize.Font{}
			if colour, ok := fontMap["colour"].(string); ok {
				font.Color = normalizeColour(colour)
			} else if color, ok := fontMap["color"].(string); ok {
				font.Color = normalizeColour(color)
			}
			if font.Color != "" {
				style.Font = font
			}
		}

		if fillMap, ok := formatStyle["fill"].(map[string]any); ok {
			if colour, ok := fillMap["colour"].(string); ok {
				style.Fill = excelize.Fill{
					Type:    "pattern",
					Pattern: 1,
					Color:   []string{normalizeColour(colour)},
				}
			} else if color, ok := fillMap["color"].(string); ok {
				style.Fill = excelize.Fill{
					Type:    "pattern",
					Pattern: 1,
					Color:   []string{normalizeColour(color)},
				}
			}
		}

		// Create the style and get its ID
		styleID, err := f.NewStyle(style)
		if err == nil {
			format.Format = &styleID
		} else {
			logger.WithError(err).Warn("Failed to create conditional format style")
		}
	}

	if err := f.SetConditionalFormat(sheetName, rangeRef, []excelize.ConditionalFormatOptions{format}); err != nil {
		return nil, &FormatError{
			Operation: fmt.Sprintf("conditional_format_%s", formatType),
			Range:     rangeRef,
			Cause:     fmt.Errorf("failed to set rule-based formatting: %w", err),
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
		"type": formatType,
	}

	return mcp.NewToolResultJSON(result)
}

// getPatternType converts pattern name to Excelize pattern type
func getPatternType(pattern string) int {
	patterns := map[string]int{
		"solid":           1,
		"darkGray":        2,
		"mediumGray":      3,
		"lightGray":       4,
		"gray125":         5,
		"gray0625":        6,
		"darkHorizontal":  7,
		"darkVertical":    8,
		"darkDown":        9,
		"darkUp":          10,
		"darkGrid":        11,
		"darkTrellis":     12,
		"lightHorizontal": 13,
		"lightVertical":   14,
		"lightDown":       15,
		"lightUp":         16,
		"lightGrid":       17,
		"lightTrellis":    18,
	}

	if patternType, ok := patterns[pattern]; ok {
		return patternType
	}

	return 1 // Default to solid
}

// getBorderStyle converts border style name to Excelize border style
func getBorderStyle(style string) int {
	styles := map[string]int{
		"thin":             1,
		"medium":           2,
		"dashed":           3,
		"dotted":           4,
		"thick":            5,
		"double":           6,
		"hair":             7,
		"mediumDashed":     8,
		"dashDot":          9,
		"mediumDashDot":    10,
		"dashDotDot":       11,
		"mediumDashDotDot": 12,
		"slantDashDot":     13,
	}

	if borderStyle, ok := styles[style]; ok {
		return borderStyle
	}

	return 1 // Default to thin
}

// mergeStyles merges a new style with an existing style
// New style properties override existing ones, but nil/empty values in new style preserve existing values
func mergeStyles(existing, new *excelize.Style) *excelize.Style {
	merged := &excelize.Style{}

	// Merge Font
	if new.Font != nil {
		merged.Font = &excelize.Font{}
		if existing.Font != nil {
			// Copy existing font properties
			*merged.Font = *existing.Font
		}
		// Override with new font properties
		if new.Font.Bold {
			merged.Font.Bold = true
		}
		if new.Font.Italic {
			merged.Font.Italic = true
		}
		if new.Font.Underline != "" {
			merged.Font.Underline = new.Font.Underline
		}
		if new.Font.Family != "" {
			merged.Font.Family = new.Font.Family
		}
		if new.Font.Size > 0 {
			merged.Font.Size = new.Font.Size
		}
		if new.Font.Strike {
			merged.Font.Strike = true
		}
		if new.Font.Color != "" {
			merged.Font.Color = new.Font.Color
		}
		if new.Font.VertAlign != "" {
			merged.Font.VertAlign = new.Font.VertAlign
		}
	} else if existing.Font != nil {
		// Keep existing font if no new font specified
		merged.Font = existing.Font
	}

	// Merge Fill
	if new.Fill.Type != "" {
		merged.Fill = new.Fill
	} else if existing.Fill.Type != "" {
		merged.Fill = existing.Fill
	}

	// Merge Borders
	if len(new.Border) > 0 {
		// Merge borders: combine existing and new borders
		borderMap := make(map[string]excelize.Border)

		// Start with existing borders
		for _, border := range existing.Border {
			borderMap[border.Type] = border
		}

		// Override/add new borders
		for _, border := range new.Border {
			borderMap[border.Type] = border
		}

		// Convert map back to slice
		merged.Border = make([]excelize.Border, 0, len(borderMap))
		for _, border := range borderMap {
			merged.Border = append(merged.Border, border)
		}
	} else if len(existing.Border) > 0 {
		merged.Border = existing.Border
	}

	// Merge Alignment
	if new.Alignment != nil {
		merged.Alignment = &excelize.Alignment{}
		if existing.Alignment != nil {
			// Copy existing alignment
			*merged.Alignment = *existing.Alignment
		}
		// Override with new alignment properties
		if new.Alignment.Horizontal != "" {
			merged.Alignment.Horizontal = new.Alignment.Horizontal
		}
		if new.Alignment.Vertical != "" {
			merged.Alignment.Vertical = new.Alignment.Vertical
		}
		if new.Alignment.WrapText {
			merged.Alignment.WrapText = true
		}
		if new.Alignment.TextRotation != 0 {
			merged.Alignment.TextRotation = new.Alignment.TextRotation
		}
		if new.Alignment.Indent != 0 {
			merged.Alignment.Indent = new.Alignment.Indent
		}
	} else if existing.Alignment != nil {
		merged.Alignment = existing.Alignment
	}

	// Merge Protection
	if new.Protection != nil {
		merged.Protection = new.Protection
	} else if existing.Protection != nil {
		merged.Protection = existing.Protection
	}

	// Merge Number Format
	if new.NumFmt != 0 {
		merged.NumFmt = new.NumFmt
	} else {
		merged.NumFmt = existing.NumFmt
	}

	if new.CustomNumFmt != nil && *new.CustomNumFmt != "" {
		merged.CustomNumFmt = new.CustomNumFmt
	} else if existing.CustomNumFmt != nil {
		merged.CustomNumFmt = existing.CustomNumFmt
	}

	if new.DecimalPlaces != nil {
		merged.DecimalPlaces = new.DecimalPlaces
	} else if existing.DecimalPlaces != nil {
		merged.DecimalPlaces = existing.DecimalPlaces
	}

	if new.NegRed {
		merged.NegRed = true
	} else {
		merged.NegRed = existing.NegRed
	}

	return merged
}

// normalizeColour strips leading # from colour strings for flexibility
// Accepts both "4472C4" and "#4472C4" formats
func normalizeColour(colour string) string {
	return strings.TrimPrefix(colour, "#")
}
