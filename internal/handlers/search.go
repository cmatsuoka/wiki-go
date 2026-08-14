package handlers

import (
	"encoding/json"
	"net/http"
	"wiki-go/internal/auth"
	"wiki-go/internal/config"
	"wiki-go/internal/service"
)

type SearchRequest struct {
	Query string `json:"query"`
}

func SearchHandler(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	session := auth.GetSession(r)
	results := service.Search(cfg, session, req.Query)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
