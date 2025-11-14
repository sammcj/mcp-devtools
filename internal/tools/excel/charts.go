package excel

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// handleCreateChart creates a chart in the worksheet
func handleCreateChart(logger *logrus.Logger, filePath string, sheetName string, options map[string]any) (*mcp.CallToolResult, error) {
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
	}).Info("Creating chart in worksheet")

	// Validate required parameters
	chartType, ok := options["type"].(string)
	if !ok || chartType == "" {
		return nil, &ValidationError{
			Field:   "type",
			Value:   options["type"],
			Message: "chart type is required (line, bar, column, pie, scatter, area)",
		}
	}

	position, ok := options["position"].(string)
	if !ok || position == "" {
		return nil, &ValidationError{
			Field:   "position",
			Value:   options["position"],
			Message: "position parameter is required (e.g., 'E2')",
		}
	}

	// Validate chart type
	excelChartType, err := mapChartType(chartType)
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
			Operation: "create_chart",
			SheetName: sheetName,
			Cause:     fmt.Errorf("worksheet not found"),
		}
	}

	// Build chart configuration
	chartConfig := buildChartConfig(excelChartType, sheetName, options)

	// Add chart to worksheet
	if err := f.AddChart(sheetName, position, chartConfig); err != nil {
		return nil, &ChartError{
			Operation: "create",
			ChartType: chartType,
			Cause:     fmt.Errorf("failed to create chart: %w", err),
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

// mapChartType maps user-friendly chart type names to Excelize chart types
func mapChartType(chartType string) (excelize.ChartType, error) {
	chartTypes := map[string]excelize.ChartType{
		"line":    excelize.Line,
		"bar":     excelize.Bar,
		"column":  excelize.Col,
		"pie":     excelize.Pie,
		"scatter": excelize.Scatter,
		"area":    excelize.Area,
	}

	excelType, ok := chartTypes[chartType]
	if !ok {
		return 0, &ValidationError{
			Field:   "type",
			Value:   chartType,
			Message: fmt.Sprintf("invalid chart type '%s', must be one of: line, bar, column, pie, scatter, area", chartType),
		}
	}

	return excelType, nil
}

// buildChartConfig constructs an Excelize chart configuration from options
func buildChartConfig(chartType excelize.ChartType, sheetName string, options map[string]any) *excelize.Chart {
	config := &excelize.Chart{
		Type: chartType,
	}

	// Set chart title
	if title, ok := options["title"].(string); ok && title != "" {
		config.Title = []excelize.RichTextRun{
			{
				Text: title,
			},
		}
	}

	// Set X-axis title
	if xAxisTitle, ok := options["x_axis_title"].(string); ok && xAxisTitle != "" {
		config.XAxis = excelize.ChartAxis{
			Title: []excelize.RichTextRun{
				{
					Text: xAxisTitle,
				},
			},
		}
	}

	// Set Y-axis title
	if yAxisTitle, ok := options["y_axis_title"].(string); ok && yAxisTitle != "" {
		config.YAxis = excelize.ChartAxis{
			Title: []excelize.RichTextRun{
				{
					Text: yAxisTitle,
				},
			},
		}
	}

	// Configure legend
	legendConfig := buildLegendConfig(options)
	if legendConfig != nil {
		config.Legend = *legendConfig
	}

	// Configure data labels
	if _, ok := options["data_labels"].(map[string]any); ok {
		config.ShowBlanksAs = "gap" // Default: don't plot blank cells
		// Excelize automatically shows labels based on series configuration
	}

	// Configure chart size
	if sizeConfig, ok := options["size"].(map[string]any); ok {
		if width, ok := sizeConfig["width"].(float64); ok {
			config.Dimension.Width = uint(width)
		}
		if height, ok := sizeConfig["height"].(float64); ok {
			config.Dimension.Height = uint(height)
		}
	}

	// Set default size if not specified
	if config.Dimension.Width == 0 {
		config.Dimension.Width = uint(640)
	}
	if config.Dimension.Height == 0 {
		config.Dimension.Height = uint(480)
	}

	// Build series from options
	series := buildChartSeries(sheetName, options)
	config.Series = series

	return config
}

// buildLegendConfig constructs legend configuration
func buildLegendConfig(options map[string]any) *excelize.ChartLegend {
	legendConfig, ok := options["legend"].(map[string]any)
	if !ok {
		return nil
	}

	// Check if legend should be shown
	if show, ok := legendConfig["show"].(bool); ok && !show {
		return &excelize.ChartLegend{
			Position: "none",
		}
	}

	legend := &excelize.ChartLegend{
		Position: "bottom", // Default position
	}

	// Set legend position
	if position, ok := legendConfig["position"].(string); ok {
		validPositions := map[string]bool{
			"top":    true,
			"bottom": true,
			"left":   true,
			"right":  true,
			"none":   true,
		}
		if validPositions[position] {
			legend.Position = position
		}
	}

	return legend
}

// buildChartSeries constructs chart series from options
func buildChartSeries(sheetName string, options map[string]any) []excelize.ChartSeries {
	var series []excelize.ChartSeries

	// Check for series configuration
	if seriesConfig, ok := options["series"].([]any); ok {
		// Use detailed series configuration
		for _, s := range seriesConfig {
			seriesMap, ok := s.(map[string]any)
			if !ok {
				continue
			}

			chartSeries := excelize.ChartSeries{}

			// Series name
			if name, ok := seriesMap["name"].(string); ok && name != "" {
				chartSeries.Name = name
			}

			// Categories (X-axis data)
			if categories, ok := seriesMap["categories"].(string); ok && categories != "" {
				chartSeries.Categories = fmt.Sprintf("%s!%s", sheetName, categories)
			}

			// Values (Y-axis data)
			if values, ok := seriesMap["values"].(string); ok && values != "" {
				chartSeries.Values = fmt.Sprintf("%s!%s", sheetName, values)
			}

			// Marker configuration
			if marker, ok := seriesMap["marker"].(map[string]any); ok {
				chartSeries.Marker = buildMarkerConfig(marker)
			}

			// Line configuration
			if line, ok := seriesMap["line"].(map[string]any); ok {
				chartSeries.Line = buildLineConfig(line)
			}

			series = append(series, chartSeries)
		}
	} else if dataRange, ok := options["data_range"].(string); ok && dataRange != "" {
		// Simple data range configuration - create a single series
		series = append(series, excelize.ChartSeries{
			Categories: fmt.Sprintf("%s!%s", sheetName, dataRange),
			Values:     fmt.Sprintf("%s!%s", sheetName, dataRange),
		})
	}

	return series
}

// buildMarkerConfig constructs marker configuration
func buildMarkerConfig(marker map[string]any) excelize.ChartMarker {
	config := excelize.ChartMarker{}

	if symbol, ok := marker["symbol"].(string); ok {
		config.Symbol = symbol
	}

	if size, ok := marker["size"].(float64); ok {
		config.Size = int(size)
	}

	return config
}

// buildLineConfig constructs line configuration
func buildLineConfig(line map[string]any) excelize.ChartLine {
	config := excelize.ChartLine{}

	// Note: Type field for ChartLine is ChartLineType, not string
	// We'll skip style configuration for now as it requires specific type mapping
	// Common line styles would need to be mapped to excelize.ChartLineType values

	if width, ok := line["width"].(float64); ok {
		config.Width = width
	}

	if smooth, ok := line["smooth"].(bool); ok {
		config.Smooth = smooth
	}

	return config
}
