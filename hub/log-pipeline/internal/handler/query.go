package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type LogItem struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Service   string `json:"service"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// QueryLogs returns an empty but correctly shaped logs response for now.
func QueryLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":   make([]LogItem, 0),
		"limit":   limit,
		"next":    "",
		"filters": r.URL.Query(),
	})
}

func GetByTraceID(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("trace_id")
	writeJSON(w, http.StatusOK, map[string]any{
		"trace_id": traceID,
		"items":    make([]LogItem, 0),
	})
}

func Stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"time":        time.Now().UTC().Format(time.RFC3339),
		"total_logs":  0,
		"error_logs":  0,
		"warn_logs":   0,
		"by_service":  map[string]int{},
		"by_severity": map[string]int{},
	})
}
