package handler

import (
	"encoding/json"
	"net/http"

	"github.com/idiey/collector-engine/internal/payload"
	"github.com/idiey/collector-engine/internal/store"
)

// APMHandler handles POST /ingest/apm requests.
type APMHandler struct {
	Store store.Store
}

// ServeHTTP decodes an APMPayload, validates required fields, and persists it.
func (h *APMHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p payload.APMPayload

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
	if len(p.Spans) == 0 {
		WriteError(w, http.StatusBadRequest, "spans are required")
		return
	}

	if err := h.Store.SaveAPM(r.Context(), &p); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save apm")
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
