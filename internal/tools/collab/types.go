package collab

import "time"

// Session represents a collaboration session between agents
type Session struct {
	SessionID    string                  `json:"session_id"`
	Topic        string                  `json:"topic"`
	Status       string                  `json:"status"` // "active" or "closed"
	Participants map[string]*Participant `json:"participants"`
	CreatedBy    string                  `json:"created_by"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
	MessageCount int                     `json:"message_count"`
	Summary      string                  `json:"summary,omitempty"`
}

// Participant represents an agent participating in a session
type Participant struct {
	JoinedAt time.Time `json:"joined_at"`
	LastRead int       `json:"last_read"`
}

// Message represents a single message in a collaboration session
type Message struct {
	ID        int       `json:"id"`
	From      string    `json:"from"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Valid message types
var validMessageTypes = map[string]bool{
	"feature_request":        true,
	"implementation_summary": true,
	"question":               true,
	"feedback":               true,
	"bug_report":             true,
	"api_change":             true,
	"general":                true,
}

// Limits for input validation
const (
	maxTopicLength       = 500
	maxContentLength     = 100000
	maxParticipantLength = 128
	maxSummaryLength     = 2000
)

// createSessionResponse is the response for creating a session
type createSessionResponse struct {
	SessionID   string `json:"session_id"`
	Topic       string `json:"topic"`
	Participant string `json:"participant"`
	Status      string `json:"status"`
	Hints       string `json:"hints,omitempty"`
}

// joinSessionResponse is the response for joining a session
type joinSessionResponse struct {
	SessionID    string    `json:"session_id"`
	Topic        string    `json:"topic"`
	Status       string    `json:"status"`
	Participant  string    `json:"participant"`
	Participants []string  `json:"participants"`
	MessageCount int       `json:"message_count"`
	Messages     []Message `json:"messages,omitempty"`
	Hints        string    `json:"hints,omitempty"`
}

// postResponse is the response for posting a message
type postResponse struct {
	MessageID int    `json:"message_id"`
	SessionID string `json:"session_id"`
}

// checkResponse is the response for checking new messages
type checkResponse struct {
	SessionID   string    `json:"session_id"`
	NewMessages []Message `json:"new_messages"`
	HasNew      bool      `json:"has_new"`
}

// readResponse is the response for reading all messages
type readResponse struct {
	SessionID string    `json:"session_id"`
	Topic     string    `json:"topic"`
	Messages  []Message `json:"messages"`
	Total     int       `json:"total"`
}

// sessionSummary is a summary of a session for listing
type sessionSummary struct {
	SessionID    string   `json:"session_id"`
	Topic        string   `json:"topic"`
	Status       string   `json:"status"`
	Participants []string `json:"participants"`
	MessageCount int      `json:"message_count"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// listSessionsResponse is the response for listing sessions
type listSessionsResponse struct {
	Sessions []sessionSummary `json:"sessions"`
	Total    int              `json:"total"`
}

// closeResponse is the response for closing a session
type closeResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Summary   string `json:"summary,omitempty"`
}

// waitResponse is the response for the collab_wait tool
type waitResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"` // "new_messages" or "timeout"
	NewCount  int    `json:"new_count"`
	Message   string `json:"message"`
}
