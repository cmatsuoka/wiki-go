package mcp

import (
	"encoding/json"
	"testing"
)

func TestJsonrpcSuccess(t *testing.T) {
	id := 1
	resp := jsonrpcSuccess(&id, map[string]string{"key": "value"})

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC 2.0, got %s", resp.JSONRPC)
	}
	if resp.ID == nil || *resp.ID != 1 {
		t.Errorf("expected ID 1, got %v", resp.ID)
	}
	if result, ok := resp.Result.(map[string]string); !ok || result["key"] != "value" {
		t.Errorf("unexpected result: %v", resp.Result)
	}
}

func TestJsonrpcSuccessNilID(t *testing.T) {
	resp := jsonrpcSuccess(nil, map[string]string{"key": "value"})

	if resp.ID != nil {
		t.Errorf("expected nil ID, got %v", resp.ID)
	}
}

func TestJsonrpcErrorResp(t *testing.T) {
	id := 2
	resp := jsonrpcErrorResp(&id, -32600, "invalid request")

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC 2.0, got %s", resp.JSONRPC)
	}
	if resp.ID == nil || *resp.ID != 2 {
		t.Errorf("expected ID 2, got %v", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "invalid request" {
		t.Errorf("expected message 'invalid request', got %s", resp.Error.Message)
	}
}

func TestJsonrpcParseError(t *testing.T) {
	id := 3
	resp := jsonrpcParseError(&id, "parse failed")

	if resp.Error == nil || resp.Error.Code != errCodeParseError {
		t.Errorf("expected parse error code %d, got %v", errCodeParseError, resp.Error)
	}
	if resp.ID == nil || *resp.ID != 3 {
		t.Errorf("expected ID 3, got %v", resp.ID)
	}
}

func TestJsonrpcInvalidRequest(t *testing.T) {
	id := 4
	resp := jsonrpcInvalidRequest(&id, "bad request")

	if resp.Error == nil || resp.Error.Code != errCodeInvalidRequest {
		t.Errorf("expected invalid request code %d, got %v", errCodeInvalidRequest, resp.Error)
	}
	if resp.ID == nil || *resp.ID != 4 {
		t.Errorf("expected ID 4, got %v", resp.ID)
	}
}

func TestJsonrpcMethodNotFound(t *testing.T) {
	id := 5
	resp := jsonrpcMethodNotFound(&id, "unknown method")

	if resp.Error == nil || resp.Error.Code != errCodeMethodNotFound {
		t.Errorf("expected method not found code %d, got %v", errCodeMethodNotFound, resp.Error)
	}
	if resp.ID == nil || *resp.ID != 5 {
		t.Errorf("expected ID 5, got %v", resp.ID)
	}
}

func TestJsonrpcInvalidParams(t *testing.T) {
	id := 6
	resp := jsonrpcInvalidParams(&id, "bad params")

	if resp.Error == nil || resp.Error.Code != errCodeInvalidParams {
		t.Errorf("expected invalid params code %d, got %v", errCodeInvalidParams, resp.Error)
	}
	if resp.ID == nil || *resp.ID != 6 {
		t.Errorf("expected ID 6, got %v", resp.ID)
	}
}

func TestJsonrpcInternalError(t *testing.T) {
	id := 7
	resp := jsonrpcInternalError(&id, "internal failure")

	if resp.Error == nil || resp.Error.Code != errCodeInternalError {
		t.Errorf("expected internal error code %d, got %v", errCodeInternalError, resp.Error)
	}
	if resp.ID == nil || *resp.ID != 7 {
		t.Errorf("expected ID 7, got %v", resp.ID)
	}
}

func TestJsonrpcToolNotFound(t *testing.T) {
	id := 8
	resp := jsonrpcToolNotFound(&id, "tool missing")

	if resp.Error == nil || resp.Error.Code != -1 {
		t.Errorf("expected tool not found code -1, got %v", resp.Error)
	}
	if resp.ID == nil || *resp.ID != 8 {
		t.Errorf("expected ID 8, got %v", resp.ID)
	}
}

func TestParseToolCallParams(t *testing.T) {
	t.Run("valid params", func(t *testing.T) {
		params := json.RawMessage(`{"name": "test_tool", "arguments": {"arg1": "val1"}}`)
		p := parseToolCallParams(params)

		if p.Name != "test_tool" {
			t.Errorf("expected name 'test_tool', got %s", p.Name)
		}
		if string(p.Arguments) != `{"arg1": "val1"}` {
			t.Errorf("unexpected arguments: %s", p.Arguments)
		}
	})

	t.Run("nil params", func(t *testing.T) {
		p := parseToolCallParams(nil)
		if p.Name != "" {
			t.Errorf("expected empty name, got %s", p.Name)
		}
		if p.Arguments != nil {
			t.Errorf("expected nil arguments, got %s", p.Arguments)
		}
	})

	t.Run("empty params", func(t *testing.T) {
		p := parseToolCallParams(json.RawMessage(`{}`))
		if p.Name != "" {
			t.Errorf("expected empty name, got %s", p.Name)
		}
	})
}
