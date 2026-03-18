package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/idiey/collector-engine/internal/handler"
	"github.com/idiey/collector-engine/internal/payload"
	"github.com/idiey/collector-engine/internal/store"
)

func TestHeartbeatHandler_Valid(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.HeartbeatHandler{Store: s}
	body, _ := json.Marshal(payload.HeartbeatPayload{AgentID: "a1", Status: "ok"})
	req := httptest.NewRequest(http.MethodPost, "/ingest/heartbeat", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if s.LenHeartbeats() != 1 {
		t.Fatalf("expected 1 stored, got %d", s.LenHeartbeats())
	}
}

func TestHeartbeatHandler_MissingAgentID(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.HeartbeatHandler{Store: s}
	body, _ := json.Marshal(payload.HeartbeatPayload{Status: "ok"})
	req := httptest.NewRequest(http.MethodPost, "/ingest/heartbeat", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHeartbeatHandler_MissingStatus(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.HeartbeatHandler{Store: s}
	body, _ := json.Marshal(payload.HeartbeatPayload{AgentID: "a1"})
	req := httptest.NewRequest(http.MethodPost, "/ingest/heartbeat", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestMetricsHandler_Valid(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.MetricsHandler{Store: s}
	body, _ := json.Marshal(payload.MetricsPayload{AgentID: "a1", SystemID: "s1"})
	req := httptest.NewRequest(http.MethodPost, "/ingest/metrics", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestMetricsHandler_MissingAgentID(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.MetricsHandler{Store: s}
	body, _ := json.Marshal(payload.MetricsPayload{})
	req := httptest.NewRequest(http.MethodPost, "/ingest/metrics", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestLogsHandler_Valid(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.LogsHandler{Store: s}
	body, _ := json.Marshal(payload.LogPayload{
		AgentID: "a1", Service: "api",
		Entries: []payload.LogEntry{{Level: "info", Message: "ok"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/ingest/logs", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestLogsHandler_MissingAgentID(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.LogsHandler{Store: s}
	body, _ := json.Marshal(payload.LogPayload{Service: "api"})
	req := httptest.NewRequest(http.MethodPost, "/ingest/logs", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPMHandler_Valid(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.APMHandler{Store: s}
	body, _ := json.Marshal(payload.APMPayload{
		AgentID: "a1", Service: "api",
		Spans: []payload.APMSpan{{SpanID: "s1", TraceID: "t1", Operation: "GET /"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/ingest/apm", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestAPMHandler_MissingAgentID(t *testing.T) {
	s := store.NewMemoryStore()
	h := &handler.APMHandler{Store: s}
	body, _ := json.Marshal(payload.APMPayload{Service: "api"})
	req := httptest.NewRequest(http.MethodPost, "/ingest/apm", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
