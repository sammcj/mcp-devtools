package code_rename

import (
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// lspCodec encodes JSON-RPC payloads with the protocol package's union-aware
// codec. Sealed unions such as WorkspaceEdit.DocumentChanges are dispatched by
// generated decoders that the jsonrpc2 default codec does not consult.
type lspCodec struct{}

var _ jsonrpc2.Codec = lspCodec{}

func (lspCodec) Marshal(v any) ([]byte, error) {
	switch raw := v.(type) {
	case jsonrpc2.RawMessage:
		return rawOrNull(raw), nil
	case *jsonrpc2.RawMessage:
		if raw == nil {
			return []byte("null"), nil
		}
		return rawOrNull(*raw), nil
	case json.RawMessage:
		return rawOrNull(raw), nil
	case *json.RawMessage:
		if raw == nil {
			return []byte("null"), nil
		}
		return rawOrNull(*raw), nil
	}
	return protocol.Marshal(v)
}

func (lspCodec) Unmarshal(data []byte, v any) error {
	// Raw destinations take a copy the caller owns, matching the jsonrpc2 codec contract
	switch dst := v.(type) {
	case *jsonrpc2.RawMessage:
		*dst = append(jsonrpc2.RawMessage(nil), data...)
		return nil
	case *json.RawMessage:
		*dst = append(json.RawMessage(nil), data...)
		return nil
	}
	return protocol.Unmarshal(data, v)
}

func rawOrNull(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("null")
	}
	return raw
}
