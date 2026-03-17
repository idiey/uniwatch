package handler

import (
	"encoding/json"
	"net/http"

	"github.com/idiey/collector-engine/internal/payload"
	"github.com/idiey/collector-engine/internal/store"
)

// HeartbeatHandler handles POST /ingest/heartbeat requests.
type HeartbeatHandler struct {
	Store store.Store
}

// ServeHTTP decodes a HeartbeatPayload, validates required fields, and persists it.
func (h *HeartbeatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p payload.HeartbeatPayload

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
	if p.Status == "" {
		WriteError(w, http.StatusBadRequest, "status is required")
		return
	}

	if err := h.Store.SaveHeartbeat(r.Context(), &p); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save heartbeat")
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
