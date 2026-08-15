package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	"wiki-go/internal/config"
)

const (
	// MCP protocol version
	protocolVersion = "2024-11-05"
	// Server info
	serverName    = "wiki-go"
	serverVersion = "0.1.0"
	// Session TTL
	sessionTTL = 24 * time.Hour
	// Cleanup interval
	cleanupInterval = 5 * time.Minute
)

// sessionKey is the context key for the authenticated session
type sessionKey struct{}

// SessionFromContext retrieves the authenticated session from the request context
func SessionFromContext(ctx context.Context) (*sessionEntry, bool) {
	s, ok := ctx.Value(sessionKey{}).(*sessionEntry)
	return s, ok
}

// sessionEntry holds the authenticated session and MCP session metadata
type sessionEntry struct {
	AuthSession  any
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ResponseChan chan []byte
}

// Handler serves MCP Streamable HTTP on a single path
type Handler struct {
	srv *mcpServer
	cfg *config.Config
}

// mcpServer holds state for the MCP server
type mcpServer struct {
	sessions map[string]*sessionEntry
	mu       sync.RWMutex
	ticker   *time.Ticker
	cancel   context.CancelFunc
}

// NewHandler creates a new MCP handler
func NewHandler(cfg *config.Config) *Handler {
	srv := &mcpServer{
		sessions: make(map[string]*sessionEntry),
		ticker:   time.NewTicker(cleanupInterval),
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv.cancel = cancel
	go srv.cleanup(ctx)
	return &Handler{srv: srv, cfg: cfg}
}

// Close stops the session cleanup goroutine
func (h *Handler) Close() {
	h.srv.cancel()
	h.srv.ticker.Stop()
}

// cleanup removes expired sessions periodically
func (s *mcpServer) cleanup(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ticker.C:
			s.mu.Lock()
			for id, entry := range s.sessions {
				if time.Now().After(entry.ExpiresAt) {
					close(entry.ResponseChan)
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

// generateSessionID generates a random 32-byte hex session ID
func (s *mcpServer) generateSessionID() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ServeHTTP handles HTTP requests for the MCP endpoint
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleSSE(w, r)
	case http.MethodPost:
		h.handleJSONRPC(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}


// handleSSE establishes an SSE connection for server-to-client notifications
func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Reuse an existing session when the client reconnects its SSE stream
	// with a sessionId (per the Streamable HTTP spec). Otherwise create a
	// new one. The session is NOT deleted when the stream closes — it lives
	// until the TTL cleanup or an explicit DELETE, so a dropped SSE stream
	// does not invalidate the client's session.
	sessionID := r.URL.Query().Get("sessionId")
	h.srv.mu.RLock()
	entry, exists := h.srv.sessions[sessionID]
	h.srv.mu.RUnlock()
	if !exists {
		sessionID = h.srv.generateSessionID()
		entry = &sessionEntry{
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(sessionTTL),
			ResponseChan: make(chan []byte, 100),
		}
		h.srv.mu.Lock()
		h.srv.sessions[sessionID] = entry
		h.srv.mu.Unlock()
	}

	// Send endpoint event as per MCP spec
	fmt.Fprintf(w, "event: endpoint\ndata: %s?sessionId=%s\n\n", r.URL.Path, sessionID)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-entry.ResponseChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

// handleDelete removes an MCP session
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		writeJSONResponse(w, jsonrpcInvalidRequest(nil, "Mcp-Session-Id header required"))
		return
	}

	h.srv.mu.Lock()
	entry := h.srv.sessions[sessionID]
	delete(h.srv.sessions, sessionID)
	h.srv.mu.Unlock()

	if entry != nil {
		close(entry.ResponseChan)
	}

	w.WriteHeader(http.StatusOK)
}

// sendResponse sends a JSON-RPC response to the session's SSE stream
func (h *Handler) sendResponse(sessionID string, resp jsonrpcResponse) {
	h.srv.mu.RLock()
	entry, ok := h.srv.sessions[sessionID]
	h.srv.mu.RUnlock()

	if !ok {
		return
	}

	b, err := marshalJSON(resp)
	if err != nil {
		b, _ = marshalJSON(jsonrpcInternalError(resp.ID, "Internal server error during marshalling"))
	}

	defer func() {
		recover()
	}()

	select {
	case entry.ResponseChan <- b:
	case <-time.After(5 * time.Second):
	}
}

// writeJSONResponse marshals and writes a JSON-RPC response to the HTTP response
func writeJSONResponse(w http.ResponseWriter, resp jsonrpcResponse) {
	b, err := marshalJSON(resp)
	if err != nil {
		b, _ = marshalJSON(jsonrpcInternalError(nil, "Internal server error during marshalling"))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// handleJSONRPC processes a JSON-RPC 2.0 request
func (h *Handler) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	// Verify content type
	ct := r.Header.Get("Content-Type")
	if ct != "application/json" && ct != "application/json; charset=utf-8" {
		writeJSONResponse(w, jsonrpcInvalidRequest(nil, "Invalid content type"))
		return
	}

	body, err := readJSONBody(w, r)
	if err != nil {
		writeJSONResponse(w, jsonrpcParseError(nil, "Failed to read request body"))
		return
	}

	var req jsonrpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONResponse(w, jsonrpcParseError(nil, "Parse error"))
		return
	}

	if req.JSONRPC != "2.0" {
		writeJSONResponse(w, jsonrpcInvalidRequest(req.ID, "Invalid JSON-RPC version"))
		return
	}

	if req.Method == "" {
		writeJSONResponse(w, jsonrpcInvalidRequest(req.ID, "Method is required"))
		return
	}

	// Authenticate: get session from context (set by AuthMiddleware)
	authEntry, _ := SessionFromContext(r.Context())
	var authSession any
	if authEntry != nil {
		authSession = authEntry.AuthSession
	}

	// Attach the request context to the JSON-RPC request so tools can access the session
	req.ctx = r.Context()

	// Get session ID from query param or header
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sessionID = r.Header.Get("Mcp-Session-Id")
	}

	var (
		entry  *sessionEntry
		exists bool
	)
	if req.Method == "initialize" && sessionID == "" {
		// Create new session for initialize
		sessionID = h.srv.generateSessionID()
		entry = &sessionEntry{
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(sessionTTL),
			ResponseChan: make(chan []byte, 100),
		}
		h.srv.mu.Lock()
		h.srv.sessions[sessionID] = entry
		h.srv.mu.Unlock()
		w.Header().Set("Mcp-Session-Id", sessionID)
	} else if sessionID == "" {
		writeJSONResponse(w, jsonrpcInvalidRequest(req.ID, "Mcp-Session-Id required"))
		return
	} else {
		h.srv.mu.RLock()
		entry, exists = h.srv.sessions[sessionID]
		h.srv.mu.RUnlock()

		if !exists {
			writeJSONResponse(w, jsonrpcInvalidRequest(req.ID, "Session not found"))
			return
		}
	}

	// Dispatch to MCP method handlers
	var response jsonrpcResponse
	switch req.Method {
	case "initialize":
		h.srv.mu.Lock()
		entry.AuthSession = authSession
		h.srv.mu.Unlock()

		response = jsonrpcSuccess(req.ID, map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    jsonrpcCapabilities{Tools: &jsonrpcToolCapabilities{}},
			"serverInfo": map[string]interface{}{
				"name":    serverName,
				"version": serverVersion,
			},
		})
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
		return
	case "tools/list":
		response = h.handleToolsList(&req)
	case "tools/call":
		response = h.handleToolsCall(&req)
	default:
		response = jsonrpcMethodNotFound(req.ID, fmt.Sprintf("Unknown method: %s", req.Method))
	}

	writeJSONResponse(w, response)
}

// readJSONBody reads the request body up to a reasonable limit
func readJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	const maxBodySize = 10 * 1024 * 1024 // 10MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// marshalJSON marshals v to JSON
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
