package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

type AlertItem struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func List(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []AlertItem{})
}

func Stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"time":     time.Now().UTC().Format(time.RFC3339),
		"total":    0,
		"critical": 0,
		"warning":  0,
	})
}
