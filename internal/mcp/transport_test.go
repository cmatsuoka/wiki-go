package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wiki-go/internal/auth"
	"wiki-go/internal/config"
)

// extractJSONFromSSE extracts the JSON payload from an SSE response
func extractJSONFromSSE(body string) string {
	parts := strings.SplitN(body, "\ndata: ", 2)
	if len(parts) == 2 {
		return strings.TrimSuffix(parts[1], "\n\n")
	}
	return body
}

func createTestHandler() *Handler {
	return NewHandler(&config.Config{})
}

func injectSession(r *http.Request, sess *sessionEntry) *http.Request {
	ctx := context.WithValue(r.Context(), sessionKey{}, sess)
	return r.WithContext(ctx)
}

func TestHandleJSONRPCInitialize(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	reqBody := `{"jsonrpc":"2.0","method":"initialize","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = injectSession(req, &sessionEntry{})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if sessionID := w.Header().Get("Mcp-Session-Id"); sessionID == "" {
		t.Error("expected Mcp-Session-Id header, got empty")
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(w.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID == nil || *resp.ID != 1 {
		t.Errorf("expected ID 1, got %v", resp.ID)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	if result["protocolVersion"] != protocolVersion {
		t.Errorf("expected protocol version %s, got %v", protocolVersion, result["protocolVersion"])
	}
}

func TestHandleJSONRPCToolsList(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	// First initialize to get a session
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq = injectSession(initReq, &sessionEntry{})
	initW := httptest.NewRecorder()
	h.ServeHTTP(initW, initReq)

	var initResp jsonrpcResponse
	json.Unmarshal([]byte(extractJSONFromSSE(initW.Body.String())), &initResp)
	sessionID := initW.Header().Get("Mcp-Session-Id")

	// Now call tools/list
	listReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":2}`))
	listReq.Header.Set("Content-Type", "application/json")
	listReq.Header.Set("Mcp-Session-Id", sessionID)
	listReq = injectSession(listReq, &sessionEntry{})

	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", listW.Code)
	}

	var listResp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(listW.Body.String())), &listResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if listResp.ID == nil || *listResp.ID != 2 {
		t.Errorf("expected ID 2, got %v", listResp.ID)
	}

	result, ok := listResp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	if _, ok := result["tools"]; !ok {
		t.Error("expected 'tools' key in result")
	}
}

func TestHandleJSONRPCToolsCallUnknownTool(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	// Initialize
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq = injectSession(initReq, &sessionEntry{})
	initW := httptest.NewRecorder()
	h.ServeHTTP(initW, initReq)
	sessionID := initW.Header().Get("Mcp-Session-Id")

	// Call unknown tool
	callReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"nonexistent"},"id":3}`))
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Mcp-Session-Id", sessionID)
	callReq = injectSession(callReq, &sessionEntry{AuthSession: &auth.Session{Role: "editor"}})

	callW := httptest.NewRecorder()
	h.ServeHTTP(callW, callReq)

	if callW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", callW.Code)
	}

	var callResp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(callW.Body.String())), &callResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if callResp.ID == nil || *callResp.ID != 3 {
		t.Errorf("expected ID 3, got %v", callResp.ID)
	}
	if callResp.Error == nil {
		t.Fatal("expected error response for unknown tool")
	}
	if !strings.Contains(callResp.Error.Message, "Unknown tool") {
		t.Errorf("expected 'Unknown tool' in error message, got %s", callResp.Error.Message)
	}
}

func TestHandleJSONRPCMissingMethod(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	reqBody := `{"jsonrpc":"2.0","id":4}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = injectSession(req, &sessionEntry{})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(w.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID == nil || *resp.ID != 4 {
		t.Errorf("expected ID 4, got %v", resp.ID)
	}
	if resp.Error == nil || resp.Error.Code != errCodeInvalidRequest {
		t.Errorf("expected invalid request error, got %v", resp.Error)
	}
}

func TestHandleJSONRPCInvalidJSON(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{invalid json`))
	req.Header.Set("Content-Type", "application/json")
	req = injectSession(req, &sessionEntry{})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(w.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil || resp.Error.Code != errCodeParseError {
		t.Errorf("expected parse error, got %v", resp.Error)
	}
}

func TestHandleJSONRPCMissingSessionID(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	reqBody := `{"jsonrpc":"2.0","method":"tools/list","id":5}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = injectSession(req, &sessionEntry{})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(w.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID == nil || *resp.ID != 5 {
		t.Errorf("expected ID 5, got %v", resp.ID)
	}
	if resp.Error == nil || resp.Error.Code != errCodeInvalidRequest {
		t.Errorf("expected invalid request error for missing session, got %v", resp.Error)
	}
}

func TestHandleJSONRPCWrongContentType(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":6}`))
	req.Header.Set("Content-Type", "text/plain")
	req = injectSession(req, &sessionEntry{})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(w.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil || resp.Error.Code != errCodeInvalidRequest {
		t.Errorf("expected invalid request error for wrong content type, got %v", resp.Error)
	}
}

func TestHandleJSONRPCNotification(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	// Initialize
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq = injectSession(initReq, &sessionEntry{})
	initW := httptest.NewRecorder()
	h.ServeHTTP(initW, initReq)
	sessionID := initW.Header().Get("Mcp-Session-Id")

	// Send notification
	notifReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	notifReq.Header.Set("Content-Type", "application/json")
	notifReq.Header.Set("Mcp-Session-Id", sessionID)
	notifReq = injectSession(notifReq, &sessionEntry{})

	notifW := httptest.NewRecorder()
	h.ServeHTTP(notifW, notifReq)

	if notifW.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d", notifW.Code)
	}
}

func TestHandleJSONRPCUnknownMethod(t *testing.T) {
	h := createTestHandler()
	defer h.Close()

	// Initialize
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq = injectSession(initReq, &sessionEntry{})
	initW := httptest.NewRecorder()
	h.ServeHTTP(initW, initReq)
	sessionID := initW.Header().Get("Mcp-Session-Id")

	// Call unknown method
	unkReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"unknown/method","id":7}`))
	unkReq.Header.Set("Content-Type", "application/json")
	unkReq.Header.Set("Mcp-Session-Id", sessionID)
	unkReq = injectSession(unkReq, &sessionEntry{})

	unkW := httptest.NewRecorder()
	h.ServeHTTP(unkW, unkReq)

	if unkW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", unkW.Code)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(extractJSONFromSSE(unkW.Body.String())), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ID == nil || *resp.ID != 7 {
		t.Errorf("expected ID 7, got %v", resp.ID)
	}
	if resp.Error == nil || resp.Error.Code != errCodeMethodNotFound {
		t.Errorf("expected method not found error, got %v", resp.Error)
	}
}

func TestReadJSONBody(t *testing.T) {
	reqBody := `{"test": "data"}`
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(reqBody)))
	w := httptest.NewRecorder()

	body, err := readJSONBody(w, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != reqBody {
		t.Errorf("expected %s, got %s", reqBody, body)
	}
}
