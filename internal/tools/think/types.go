package think

import "time"

// ThinkRequest represents the input parameters for the think tool
type ThinkRequest struct {
	Thought string `json:"thought"`
	HowHard string `json:"how_hard"`
}

// ThinkResponse represents the output of the think tool
type ThinkResponse struct {
	Thought   string    `json:"thought"`
	Timestamp time.Time `json:"timestamp"`
}
