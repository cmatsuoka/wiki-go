package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"wiki-go/internal/auth"
	"wiki-go/internal/logger"
	"wiki-go/internal/service"
)

// SourceHandler handles requests to get the raw markdown content of a page
func SourceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Method not allowed",
		})
		return
	}

	session := auth.GetSession(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/source")

	content, err := service.GetSource(cfg, session, path)
	if err != nil {
		switch {
		case err == service.ErrUnauthorized:
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Unauthorized. Admin or editor access required.",
			})
		case err == service.ErrNotFound:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Document not found",
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to read document",
			})
		}
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(content))
}

// SaveHandler handles requests to save the markdown content of a page
func SaveHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Method not allowed",
		})
		return
	}

	session := auth.GetSession(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/save")

	content, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to read request body",
		})
		return
	}
	defer r.Body.Close()

	if err := service.Save(cfg, session, path, string(content)); err != nil {
		switch {
		case err == service.ErrUnauthorized:
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Unauthorized. Admin or editor access required.",
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to save document",
			})
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Document saved successfully",
	})
}

// CreateDocumentHandler handles the API endpoint for creating new documents
func CreateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	session := auth.GetSession(r)

	var req service.CreateDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request payload", http.StatusBadRequest, err.Error())
		return
	}

	url, err := service.Create(cfg, session, req)
	if err != nil {
		switch {
		case err == service.ErrUnauthorized:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Unauthorized. Admin or editor access required.",
			})
		case err == service.ErrConflict:
			sendJSONError(w, "Document already exists", http.StatusConflict, "")
		default:
			sendJSONError(w, "Failed to create document", http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"url":     url,
		"message": "Document created successfully",
	}

	json.NewEncoder(w).Encode(response)
}

// sendJSONError sends a JSON error response with status code
func sendJSONError(w http.ResponseWriter, message string, statusCode int, errorDetails string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"success": false,
		"message": message,
	}

	if errorDetails != "" {
		response["error"] = errorDetails
	}

	json.NewEncoder(w).Encode(response)
	logger.Error("Error response: %s (%d) - %s", message, statusCode, errorDetails)
}

// DocumentHandler is a combined handler for document operations (GET, DELETE)
func DocumentHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		DeleteDocumentHandler(w, r)
	case http.MethodGet:
		// For now just return the document path
		docPath := strings.TrimPrefix(r.URL.Path, "/api/document")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"path":    docPath,
			"message": "Document retrieval not yet implemented",
		})
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Method not allowed",
		})
	}
}

// DeleteDocumentHandler handles requests to delete a document
func DeleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Method not allowed",
		})
		return
	}

	session := auth.GetSession(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/document")

	if err := service.Delete(cfg, session, path, false); err != nil {
		switch {
		case err == service.ErrUnauthorized:
			sendJSONError(w, "Authentication required", http.StatusUnauthorized, "Admin or editor access required to delete documents")
		case err == service.ErrNotFound:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Document not found",
			})
		default:
			sendJSONError(w, "Error deleting document", http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Document deleted successfully",
	})
}
