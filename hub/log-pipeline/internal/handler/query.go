package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/idiey/log-pipeline/internal/store"
)

type QueryHandler struct{ Store store.LogStore }

func (h *QueryHandler) Routes(r chi.Router) {
	r.Get("/api/logs", h.List)
	r.Get("/api/logs/stats", h.Stats)
	r.Get("/api/logs/{trace_id}", h.ByTraceID)
}

func (h *QueryHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	var from, to time.Time
	if s := q.Get("from"); s != "" {
		from, _ = time.Parse(time.RFC3339, s)
	}
	if s := q.Get("to"); s != "" {
		to, _ = time.Parse(time.RFC3339, s)
	}
	filter := store.LogFilter{
		Service: q.Get("service"), Level: q.Get("level"),
		TraceID: q.Get("trace_id"), Environment: q.Get("environment"),
		Search: q.Get("search"), Cursor: q.Get("cursor"),
		Limit: limit, From: from, To: to,
	}
	rows, nextCursor, err := h.Store.Query(r.Context(), filter)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "query failed")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": rows, "next_cursor": nextCursor})
}

func (h *QueryHandler) ByTraceID(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "trace_id")
	if traceID == "" {
		WriteError(w, http.StatusBadRequest, "trace_id required")
		return
	}
	rows, err := h.Store.GetByTraceID(r.Context(), traceID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "query failed")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": rows})
}

func (h *QueryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.Store.Stats(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "stats failed")
		return
	}
	WriteJSON(w, http.StatusOK, stats)
}
