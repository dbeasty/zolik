package mcp

import "encoding/json"

// JSON-RPC 2.0, the envelope MCP speaks.

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	errParse          = -32700
	errInvalidRequest = -32600
	errNoMethod       = -32601
	errInvalidParams  = -32602
)

// isNotification is a request with no id: it is answered with nothing.
func (r rpcRequest) isNotification() bool { return len(r.ID) == 0 }
