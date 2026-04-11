package handler

import (
	"encoding/json"
	"net/http"

	"github.com/idiey/collector-engine/internal/payload"
	"github.com/idiey/collector-engine/internal/store"
)

// LogsHandler handles POST /ingest/logs requests.
type LogsHandler struct {
	Store store.Store
}

// ServeHTTP decodes a LogPayload, validates required fields, and persists it.
func (h *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p payload.LogPayload

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if p.AgentID == "" {
		WriteError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if len(p.Entries) == 0 {
		WriteError(w, http.StatusBadRequest, "entries are required")
		return
	}

	if err := h.Store.SaveLogs(r.Context(), &p); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save logs")
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
