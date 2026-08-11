package upstream

import (
	"encoding/json"
	"fmt"
	"maps"
)

// MCP protocol details this client speaks. There is no handshake in 2026-07-28:
// every request carries the version, capabilities and client identity in _meta,
// and repeats the method (and target name) in headers so gateways can route
// without parsing the body.
const (
	ProtocolVersion = "2026-07-28"

	metaKeyProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaKeyClientInfo         = "io.modelcontextprotocol/clientInfo"
	metaKeyClientCapabilities = "io.modelcontextprotocol/clientCapabilities"

	headerProtocolVersion = "MCP-Protocol-Version"
	headerMethod          = "Mcp-Method"
	headerName            = "Mcp-Name"
)

// Message represents a JSON-RPC 2.0 message.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`

	// Name is the tool, prompt or resource this request targets, sent as the
	// Mcp-Name header. Not part of the JSON-RPC body.
	Name string `json:"-"`
}

// newRequest builds a request. The _meta fields a stateless request must carry
// are added at send time, so the same message can be retried against an
// upstream that predates 2026-07-28. Params must marshal to a JSON object.
func newRequest(id any, method, name string, params map[string]any) (*Message, error) {
	body := make(map[string]any, len(params))
	maps.Copy(body, params)

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s params: %w", method, err)
	}

	return &Message{JSONRPC: "2.0", ID: id, Method: method, Name: name, Params: raw}, nil
}

// encode renders the message for the wire. In stateless mode it adds the _meta
// block 2026-07-28 requires; in legacy mode it sends the params untouched,
// which is what an older upstream expects.
func (m *Message) encode(stateless bool) ([]byte, error) {
	if !stateless {
		return json.Marshal(m)
	}

	params := map[string]any{}
	if len(m.Params) > 0 {
		if err := json.Unmarshal(m.Params, &params); err != nil {
			return nil, fmt.Errorf("failed to read %s params: %w", m.Method, err)
		}
	}
	if params == nil {
		// JSON null unmarshals into a nil map, which panics on assignment.
		params = map[string]any{}
	}
	params["_meta"] = map[string]any{
		metaKeyProtocolVersion:    ProtocolVersion,
		metaKeyClientCapabilities: map[string]any{},
		metaKeyClientInfo: map[string]any{
			"name":    "mcp-devtools-proxy",
			"version": ProtocolVersion,
		},
	}

	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s params: %w", m.Method, err)
	}

	withMeta := *m
	withMeta.Params = raw
	return json.Marshal(&withMeta)
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
