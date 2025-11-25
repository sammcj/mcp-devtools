package anthropic

// AnthropicModel represents an Anthropic Claude model with all provider IDs and metadata
type AnthropicModel struct {
	ModelName          string   `json:"modelName"`                  // e.g., "Claude Sonnet 4.5"
	ClaudeAPIID        string   `json:"claudeApiId"`                // e.g., "claude-sonnet-4-5-20250929"
	ClaudeAPIAlias     string   `json:"claudeApiAlias"`             // e.g., "claude-sonnet-4-5"
	AWSBedrockID       string   `json:"awsBedrockId"`               // e.g., "anthropic.claude-sonnet-4-5-20250929-v1:0"
	GCPVertexAIID      string   `json:"gcpVertexAiId"`              // e.g., "claude-sonnet-4-5@20250929"
	Pricing            string   `json:"pricing"`                    // e.g., "$3 / input MTok $15 / output MTok"
	ComparativeLatency string   `json:"comparativeLatency"`         // e.g., "Fast", "Fastest", "Moderate"
	ContextWindow      string   `json:"contextWindow"`              // e.g., "200K tokens / 1M tokens (beta)"
	MaxOutput          string   `json:"maxOutput"`                  // e.g., "64K tokens"
	KnowledgeCutoff    string   `json:"knowledgeCutoff"`            // e.g., "Jan 2025"
	TrainingDataCutoff string   `json:"trainingDataCutoff"`         // e.g., "Jul 2025"
	ModelFamily        string   `json:"modelFamily"`                // e.g., "sonnet", "haiku", "opus"
	ModelVersion       string   `json:"modelVersion"`               // e.g., "4.5", "4.1"
	RegionsSupported   []string `json:"regionsSupported,omitempty"` // AWS regions where model is available
}

// AnthropicModelSearchResult represents search results for Anthropic models
type AnthropicModelSearchResult struct {
	Models     []AnthropicModel `json:"models"`
	TotalCount int              `json:"totalCount"`
}
