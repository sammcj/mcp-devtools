package docprocessing

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/security"
)

// executeBatch processes multiple documents concurrently
func (t *DocumentProcessorTool) executeBatch(ctx context.Context, args map[string]any, sources []any) (*mcp.CallToolResult, error) {
	startTime := time.Now()

	// Convert sources to strings
	var sourceStrings []string
	for _, source := range sources {
		if sourceStr, ok := source.(string); ok {
			sourceStrings = append(sourceStrings, strings.TrimSpace(sourceStr))
		}
	}

	if len(sourceStrings) == 0 {
		return nil, fmt.Errorf("no valid sources provided")
	}

	// Determine concurrency limit
	maxConcurrency := min(t.getMaxConcurrency(args), len(sourceStrings))

	// Create channels for work distribution
	sourceChan := make(chan string, len(sourceStrings))
	resultChan := make(chan *DocumentProcessingResponse, len(sourceStrings))

	// Fill source channel
	for _, source := range sourceStrings {
		sourceChan <- source
	}
	close(sourceChan)

	// Start worker goroutines
	var wg sync.WaitGroup
	for range maxConcurrency {
		wg.Go(func() {
			for source := range sourceChan {
				// Create individual request for this source
				individualArgs := make(map[string]any)
				for k, v := range args {
					if k != "sources" { // Exclude sources array
						individualArgs[k] = v
					}
				}
				individualArgs["source"] = source

				// Parse request
				req, err := t.parseRequest(individualArgs)
				if err != nil {
					resultChan <- &DocumentProcessingResponse{
						Source: source,
						Error:  fmt.Sprintf("failed to parse request: %v", err),
					}
					continue
				}

				// Process document
				response, err := t.processDocument(req)
				if err != nil {
					resultChan <- &DocumentProcessingResponse{
						Source: source,
						Error:  err.Error(),
					}
					continue
				}

				// Security: Analyse processed content for batch processing
				if security.IsEnabled() && response.Error == "" {
					sourceContext := security.SourceContext{
						Tool: "document_processing",
						URL:  source,
					}

					result, err := security.AnalyseContent(response.Content, sourceContext)
					if err == nil {
						switch result.Action {
						case security.ActionBlock:
							response.Error = fmt.Sprintf("content blocked by security policy: %s", result.Message)
						case security.ActionWarn:
							// Note: In batch processing, security warnings are noted but don't fail the processing
							// Individual responses will include the security information
						}
					}
				}

				// Cache result if successful
				if t.shouldUseCache() && response.Error == "" {
					cacheKey := t.cacheManager.GenerateCacheKey(req)
					_ = t.cacheManager.Set(cacheKey, response)
				}

				resultChan <- response
			}
		})
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []DocumentProcessingResponse
	for response := range resultChan {
		results = append(results, *response)
	}

	// Create batch summary
	summary := t.createBatchSummary(results)

	// Create batch response
	batchResponse := BatchProcessingResponse{
		Results:   results,
		Summary:   summary,
		TotalTime: time.Since(startTime),
		Timestamp: time.Now(),
	}

	return t.newToolResultJSON(batchResponse)
}

// getMaxConcurrency determines the maximum concurrency for batch processing
func (t *DocumentProcessorTool) getMaxConcurrency(args map[string]any) int {
	// Check if max_concurrency is specified
	if maxConc, ok := args["max_concurrency"].(float64); ok {
		requested := int(maxConc)
		if requested > 0 && requested <= 10 {
			return requested
		}
	}

	// Default: CPU cores - 1, minimum 1, maximum 10
	cores := runtime.NumCPU()
	defaultConcurrency := min(max(cores-1, 1), 10)

	return defaultConcurrency
}

// createBatchSummary creates a summary of batch processing results
func (t *DocumentProcessorTool) createBatchSummary(results []DocumentProcessingResponse) BatchSummary {
	summary := BatchSummary{
		TotalDocuments: len(results),
	}

	for _, result := range results {
		if result.Error == "" {
			summary.SuccessfulCount++

			// Aggregate metadata
			if result.Metadata != nil {
				summary.TotalPages += result.Metadata.PageCount
				summary.TotalWords += result.Metadata.WordCount
			}

			// Count images and tables
			summary.TotalImages += len(result.Images)
			summary.TotalTables += len(result.Tables)

			// Count cache hits
			if result.CacheHit {
				summary.CacheHitCount++
			}
		} else {
			summary.FailedCount++
		}
	}

	return summary
}
