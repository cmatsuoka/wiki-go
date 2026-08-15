package mcp

import (
	"encoding/json"
	"fmt"
	"wiki-go/internal/auth"
	"wiki-go/internal/config"
	"wiki-go/internal/service"
)

// toolResult is the MCP tools/call result payload.
type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// toolContent is a single content block in a tool result.
type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// textResult builds a successful tool result with the given text.
func textResult(text string) toolResult {
	return toolResult{
		Content: []toolContent{{Type: "text", Text: text}},
	}
}

// errorResult builds a tool result marked as an error with a readable message.
func errorResult(msg string) toolResult {
	return toolResult{
		Content: []toolContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}

// toolDefinitions lists the tools exposed by the MCP server.
var toolDefinitions = []jsonrpcTool{
	{
		Name:        "read_page",
		Description: "Return the raw markdown source of a page. If the page directory exists but has no content yet, returns placeholder content.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Wiki path of the page, e.g. 'my-page' or '' for the homepage"}
			},
			"required": ["path"]
		}`),
	},
	{
		Name:        "list_pages",
		Description: "List all documents in the wiki (title and path).",
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
	},
	{
		Name:        "search_pages",
		Description: "Full-text search across pages. Supports quoted exact phrases, 'and', and 'not' operators.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string", "description": "Search query"}
			},
			"required": ["query"]
		}`),
	},
	{
		Name:        "create_page",
		Description: "Create a new page with the given title, path, and type (markdown, kanban, or links).",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string", "description": "Page title"},
				"path": {"type": "string", "description": "Wiki path for the new page"},
				"type": {"type": "string", "enum": ["markdown", "kanban", "links"], "description": "Page type"}
			},
			"required": ["title", "path"]
		}`),
	},
	{
		Name:        "write_page",
		Description: "Overwrite a page's markdown content. Creates a version history entry if the page already exists.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Wiki path of the page"},
				"content": {"type": "string", "description": "Markdown content to write"}
			},
			"required": ["path", "content"]
		}`),
	},
	{
		Name:        "delete_page",
		Description: "Delete a page and its versions and comments. Refuses to delete the homepage or a page that has child pages.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Wiki path of the page to delete"}
			},
			"required": ["path"]
		}`),
	},
}

// handleToolsList returns the list of available tools.
func (h *Handler) handleToolsList(req *jsonrpcRequest) jsonrpcResponse {
	return jsonrpcSuccess(req.ID, map[string]interface{}{
		"tools": toolDefinitions,
	})
}

// handleToolsCall invokes a tool by name.
func (h *Handler) handleToolsCall(req *jsonrpcRequest) jsonrpcResponse {
	params := parseToolCallParams(req.Params)

	if params.Name == "" {
		return jsonrpcInvalidParams(req.ID, "Tool name is required")
	}

	cfg := h.cfg
	if cfg == nil {
		return jsonrpcInternalError(req.ID, "MCP server not configured")
	}

	sess := h.sessionFromRequest(req)
	if sess == nil {
		return jsonrpcInternalError(req.ID, "No authenticated session")
	}

	var result toolResult
	switch params.Name {
	case "read_page":
		result = h.callReadPage(cfg, sess, params.Arguments)
	case "list_pages":
		result = h.callListPages(cfg, sess)
	case "search_pages":
		result = h.callSearchPages(cfg, sess, params.Arguments)
	case "create_page":
		result = h.callCreatePage(cfg, sess, params.Arguments)
	case "write_page":
		result = h.callWritePage(cfg, sess, params.Arguments)
	case "delete_page":
		result = h.callDeletePage(cfg, sess, params.Arguments)
	default:
		return jsonrpcToolNotFound(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
	}

	return jsonrpcSuccess(req.ID, result)
}

// sessionFromRequest extracts the authenticated *auth.Session from the request context.
func (h *Handler) sessionFromRequest(req *jsonrpcRequest) *auth.Session {
	entry, ok := SessionFromContext(req.ctx)
	if !ok || entry == nil {
		return nil
	}
	s, _ := entry.AuthSession.(*auth.Session)
	return s
}

// callReadPage implements the read_page tool.
func (h *Handler) callReadPage(cfg *config.Config, sess *auth.Session, args json.RawMessage) toolResult {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errorResult("Invalid arguments: " + err.Error())
	}

	content, err := service.GetSource(cfg, sess, p.Path)
	if err != nil {
		return errorResult(mapServiceError(err))
	}
	return textResult(content)
}

// callListPages implements the list_pages tool.
func (h *Handler) callListPages(cfg *config.Config, sess *auth.Session) toolResult {
	docs, err := service.List(cfg, sess)
	if err != nil {
		return errorResult(mapServiceError(err))
	}

	b, err := json.Marshal(docs)
	if err != nil {
		return errorResult("Failed to marshal document list")
	}
	return textResult(string(b))
}

// callSearchPages implements the search_pages tool.
func (h *Handler) callSearchPages(cfg *config.Config, sess *auth.Session, args json.RawMessage) toolResult {
	var p struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errorResult("Invalid arguments: " + err.Error())
	}

	results := service.Search(cfg, sess, p.Query)
	b, err := json.Marshal(results)
	if err != nil {
		return errorResult("Failed to marshal search results")
	}
	return textResult(string(b))
}

// callCreatePage implements the create_page tool.
func (h *Handler) callCreatePage(cfg *config.Config, sess *auth.Session, args json.RawMessage) toolResult {
	var p service.CreateDocumentRequest
	if err := json.Unmarshal(args, &p); err != nil {
		return errorResult("Invalid arguments: " + err.Error())
	}

	url, err := service.Create(cfg, sess, p)
	if err != nil {
		return errorResult(mapServiceError(err))
	}
	return textResult(fmt.Sprintf("Created page at %s", url))
}

// callWritePage implements the write_page tool.
func (h *Handler) callWritePage(cfg *config.Config, sess *auth.Session, args json.RawMessage) toolResult {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errorResult("Invalid arguments: " + err.Error())
	}

	if err := service.Save(cfg, sess, p.Path, p.Content); err != nil {
		return errorResult(mapServiceError(err))
	}
	return textResult(fmt.Sprintf("Saved page %s", p.Path))
}

// callDeletePage implements the delete_page tool.
func (h *Handler) callDeletePage(cfg *config.Config, sess *auth.Session, args json.RawMessage) toolResult {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return errorResult("Invalid arguments: " + err.Error())
	}

	if err := service.Delete(cfg, sess, p.Path, true); err != nil {
		return errorResult(mapServiceError(err))
	}
	return textResult(fmt.Sprintf("Deleted page %s", p.Path))
}

// mapServiceError converts a service-layer error into a readable message.
func mapServiceError(err error) string {
	switch err {
	case service.ErrUnauthorized:
		return "Unauthorized: admin or editor access required"
	case service.ErrNotFound:
		return "Page not found"
	case service.ErrConflict:
		return "A page already exists at this path"
	case service.ErrHasChildren:
		return "Page has child pages and cannot be deleted"
	default:
		return err.Error()
	}
}
