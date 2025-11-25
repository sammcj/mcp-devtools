package anthropic

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sammcj/mcp-devtools/internal/tools/webfetch"
	"github.com/sirupsen/logrus"
)

const (
	// AnthropicModelsURL is the URL to Anthropic's model documentation
	AnthropicModelsURL = "https://platform.claude.com/docs/en/about-claude/models/overview"

	// CacheTTL is how long to cache the model data (24 hours)
	CacheTTL = 24 * time.Hour
)

// CachedModelData holds cached model data with expiry
type CachedModelData struct {
	Models    []AnthropicModel
	FetchedAt time.Time
}

// Parser handles fetching and parsing Anthropic model documentation
type Parser struct {
	webClient *webfetch.WebClient
	cache     *sync.Map
	logger    *logrus.Logger
}

// NewParser creates a new Anthropic model parser
func NewParser(logger *logrus.Logger, cache *sync.Map) *Parser {
	return &Parser{
		webClient: webfetch.NewWebClient(),
		cache:     cache,
		logger:    logger,
	}
}

// GetLatestModels fetches and parses the latest Anthropic models
// It returns only the latest version of each model family
func (p *Parser) GetLatestModels(ctx context.Context) ([]AnthropicModel, error) {
	// Check cache first
	cacheKey := "anthropic_models"
	if cached, ok := p.cache.Load(cacheKey); ok {
		if cachedData, ok := cached.(CachedModelData); ok {
			if time.Since(cachedData.FetchedAt) < CacheTTL {
				p.logger.Debug("Returning cached Anthropic model data")
				return cachedData.Models, nil
			}
		}
	}

	p.logger.Info("Fetching latest Anthropic models from documentation")

	// Fetch the HTML content
	response, err := p.webClient.FetchContent(ctx, p.logger, AnthropicModelsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Anthropic documentation: %w", err)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(response.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Find the model comparison table
	models, err := p.parseModelTable(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse model table: %w", err)
	}

	// Filter to only latest version of each model family
	latestModels := p.filterLatestModels(models)

	// Cache the result
	p.cache.Store(cacheKey, CachedModelData{
		Models:    latestModels,
		FetchedAt: time.Now(),
	})

	return latestModels, nil
}

// parseModelTable parses the model comparison table from the HTML document
func (p *Parser) parseModelTable(doc *goquery.Document) ([]AnthropicModel, error) {
	var models []AnthropicModel

	// Find the table - look for tables that contain model information
	// The table should have headers like "Feature", "Claude Sonnet", etc.
	var tableFound bool
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// Check if this table has the expected headers
		headers := []string{}
		table.Find("thead tr th, thead tr td").Each(func(j int, th *goquery.Selection) {
			headers = append(headers, strings.TrimSpace(th.Text()))
		})

		// If the first header is "Feature", this is likely our table
		if len(headers) > 0 && strings.Contains(headers[0], "Feature") {
			tableFound = true
			models = p.parseTableRows(table, headers)
		}
	})

	if !tableFound {
		return nil, fmt.Errorf("model comparison table not found")
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no models parsed from table")
	}

	return models, nil
}

// parseTableRows parses the table rows and extracts model information
func (p *Parser) parseTableRows(table *goquery.Selection, headers []string) []AnthropicModel {
	// Create a map to store row data: rowName -> columnIndex -> value
	rowData := make(map[string][]string)

	// Parse all rows
	table.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
		cells := []string{}
		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			// Clean up cell text
			text := strings.TrimSpace(cell.Text())
			cells = append(cells, text)
		})

		if len(cells) > 0 {
			rowName := cells[0] // First column is the feature name
			rowData[rowName] = cells
		}
	})

	// Now construct models from the column data
	// Skip first column (Feature names) and process model columns
	var models []AnthropicModel
	for colIdx := 1; colIdx < len(headers); colIdx++ {
		modelName := headers[colIdx]
		if modelName == "" || modelName == "Feature" {
			continue
		}

		model := AnthropicModel{
			ModelName: modelName,
		}

		// Extract data from each row for this column
		if cells, ok := rowData["Claude API ID"]; ok && colIdx < len(cells) {
			model.ClaudeAPIID = cells[colIdx]
		}
		if cells, ok := rowData["Claude API alias"]; ok && colIdx < len(cells) {
			model.ClaudeAPIAlias = cells[colIdx]
		}
		// Try alternative spellings
		if model.ClaudeAPIAlias == "" {
			if cells, ok := rowData["Claude API alias 1"]; ok && colIdx < len(cells) {
				model.ClaudeAPIAlias = cells[colIdx]
			}
		}
		if cells, ok := rowData["AWS Bedrock ID"]; ok && colIdx < len(cells) {
			model.AWSBedrockID = cells[colIdx]
		}
		if cells, ok := rowData["GCP Vertex AI ID"]; ok && colIdx < len(cells) {
			model.GCPVertexAIID = cells[colIdx]
		}
		if cells, ok := rowData["Pricing"]; ok && colIdx < len(cells) {
			model.Pricing = cells[colIdx]
		}
		// Try alternative row names
		if model.Pricing == "" {
			if cells, ok := rowData["Pricing 2"]; ok && colIdx < len(cells) {
				model.Pricing = cells[colIdx]
			}
		}
		if cells, ok := rowData["Comparative latency"]; ok && colIdx < len(cells) {
			model.ComparativeLatency = cells[colIdx]
		}
		if cells, ok := rowData["Context window"]; ok && colIdx < len(cells) {
			model.ContextWindow = cells[colIdx]
		}
		if cells, ok := rowData["Max output"]; ok && colIdx < len(cells) {
			model.MaxOutput = cells[colIdx]
		}
		if cells, ok := rowData["Reliable knowledge cutoff"]; ok && colIdx < len(cells) {
			model.KnowledgeCutoff = cells[colIdx]
		}
		if cells, ok := rowData["Training data cutoff"]; ok && colIdx < len(cells) {
			model.TrainingDataCutoff = cells[colIdx]
		}

		// Extract model family and version from model name
		family, version := extractModelFamilyAndVersion(modelName)
		model.ModelFamily = family
		model.ModelVersion = version

		// Only add models with at least a Claude API ID
		if model.ClaudeAPIID != "" {
			models = append(models, model)
		}
	}

	return models
}

// extractModelFamilyAndVersion extracts the model family (sonnet, haiku, opus) and version from model name
func extractModelFamilyAndVersion(modelName string) (family, version string) {
	lowerName := strings.ToLower(modelName)

	// Extract family
	if strings.Contains(lowerName, "sonnet") {
		family = "sonnet"
	} else if strings.Contains(lowerName, "haiku") {
		family = "haiku"
	} else if strings.Contains(lowerName, "opus") {
		family = "opus"
	}

	// Extract version using regex (e.g., "4.5", "4.1", "4")
	versionRegex := regexp.MustCompile(`\d+(?:\.\d+)?`)
	if matches := versionRegex.FindString(modelName); matches != "" {
		version = matches
	}

	return family, version
}

// filterLatestModels filters to only the latest version of each model family
func (p *Parser) filterLatestModels(models []AnthropicModel) []AnthropicModel {
	// Group models by family
	familyModels := make(map[string][]AnthropicModel)
	for _, model := range models {
		if model.ModelFamily != "" {
			familyModels[model.ModelFamily] = append(familyModels[model.ModelFamily], model)
		}
	}

	// For each family, keep only the model with the highest version
	var latestModels []AnthropicModel
	for _, familyGroup := range familyModels {
		if len(familyGroup) == 0 {
			continue
		}

		// Find the latest version
		latest := familyGroup[0]
		for _, model := range familyGroup[1:] {
			if compareVersions(model.ModelVersion, latest.ModelVersion) > 0 {
				latest = model
			}
		}
		latestModels = append(latestModels, latest)
	}

	return latestModels
}

// compareVersions compares two version strings (e.g., "4.5" vs "4.1")
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Simple version comparison - split by "." and compare numerically
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Pad shorter version with zeros
	maxLen := max(len(parts2), len(parts1))

	for i := range maxLen {
		var n1, n2 int
		if i < len(parts1) {
			_, _ = fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			_, _ = fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 > n2 {
			return 1
		} else if n1 < n2 {
			return -1
		}
	}

	return 0
}
