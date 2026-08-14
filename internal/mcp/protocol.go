package mcp

import (
	"encoding/json"
)

// --- JSON-RPC 2.0 types ---

// jsonrpcRequest is a JSON-RPC 2.0 request
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      *int            `json:"id,omitempty"` // nil for notifications
}

// jsonrpcResponse is a JSON-RPC 2.0 response or notification
type jsonrpcResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonrpcError `json:"error,omitempty"`
	ID      *int          `json:"id,omitempty"`
}

// jsonrpcError is a JSON-RPC 2.0 error object
type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- Standard JSON-RPC 2.0 error codes ---

const (
	errCodeParseError     = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternalError  = -32603
)

// --- MCP capability types ---

// jsonrpcCapabilities describes server capabilities
type jsonrpcCapabilities struct {
	Tools *jsonrpcToolCapabilities `json:"tools,omitempty"`
}

// jsonrpcToolCapabilities describes tool-related capabilities
type jsonrpcToolCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// --- Tool types (used in Phase 3) ---

// jsonrpcTool defines an MCP tool
type jsonrpcTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolCallParams represents the parameters for a tools/call request
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// parseToolCallParams parses the params field of a tools/call request
func parseToolCallParams(params json.RawMessage) ToolCallParams {
	var p ToolCallParams
	if params == nil {
		return p
	}
	_ = json.Unmarshal(params, &p)
	return p
}

// --- Response constructors ---

// jsonrpcSuccess creates a successful JSON-RPC response
func jsonrpcSuccess(id *int, result any) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}

// jsonrpcParseError creates a parse error response
func jsonrpcParseError(id *int, msg string) jsonrpcResponse {
	return jsonrpcErrorResp(id, errCodeParseError, msg)
}

// jsonrpcInvalidRequest creates an invalid request response
func jsonrpcInvalidRequest(id *int, msg string) jsonrpcResponse {
	return jsonrpcErrorResp(id, errCodeInvalidRequest, msg)
}

// jsonrpcMethodNotFound creates a method not found response
func jsonrpcMethodNotFound(id *int, msg string) jsonrpcResponse {
	return jsonrpcErrorResp(id, errCodeMethodNotFound, msg)
}

// jsonrpcInvalidParams creates an invalid params response
func jsonrpcInvalidParams(id *int, msg string) jsonrpcResponse {
	return jsonrpcErrorResp(id, errCodeInvalidParams, msg)
}

// jsonrpcInternalError creates an internal error response
func jsonrpcInternalError(id *int, msg string) jsonrpcResponse {
	return jsonrpcErrorResp(id, errCodeInternalError, msg)
}

// jsonrpcToolNotFound creates a tool not found response (application error)
func jsonrpcToolNotFound(id *int, msg string) jsonrpcResponse {
	return jsonrpcErrorResp(id, -1, msg)
}

// jsonrpcErrorResp creates an error response with the given code and message
func jsonrpcErrorResp(id *int, code int, msg string) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		Error: &jsonrpcError{
			Code:    code,
			Message: msg,
		},
		ID: id,
	}
}
