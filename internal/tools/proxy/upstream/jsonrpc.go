package upstream

import "encoding/json"

// Message represents a JSON-RPC 2.0 message.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// IsRequest returns true if the message is a request.
func (m *Message) IsRequest() bool {
	return m.Method != "" && m.ID != nil
}

// IsResponse returns true if the message is a response.
func (m *Message) IsResponse() bool {
	return m.ID != nil && m.Method == "" && (m.Result != nil || m.Error != nil)
}

// IsNotification returns true if the message is a notification.
func (m *Message) IsNotification() bool {
	return m.Method != "" && m.ID == nil
}
