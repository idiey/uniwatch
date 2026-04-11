package handler

import (
	"encoding/json"
	"net/http"

	"github.com/idiey/collector-engine/internal/payload"
	"github.com/idiey/collector-engine/internal/store"
)

// MetricsHandler handles POST /ingest/metrics requests.
type MetricsHandler struct {
	Store store.Store
}

// ServeHTTP decodes a MetricsPayload, validates required fields, and persists it.
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p payload.MetricsPayload

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
	if p.PayloadType == "" {
		WriteError(w, http.StatusBadRequest, "payload_type is required")
		return
	}

	if err := h.Store.SaveMetrics(r.Context(), &p); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save metrics")
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
