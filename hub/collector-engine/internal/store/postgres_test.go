package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/idiey/collector-engine/internal/payload"
	"github.com/idiey/collector-engine/internal/store"
)

func newTestStore(t *testing.T) *store.PostgresStore {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set - skipping postgres integration test")
	}
	s, err := store.NewPostgresStore(context.Background(), url)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	return s
}

func TestPostgresStore_SaveHeartbeat(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveHeartbeat(context.Background(), &payload.HeartbeatPayload{
		AgentID: "test-agent", Status: "ok", AgentVersion: "1.0.0", UptimeSeconds: 3600,
	}); err != nil {
		t.Fatalf("SaveHeartbeat: %v", err)
	}
}

func TestPostgresStore_SaveMetrics(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveMetrics(context.Background(), &payload.MetricsPayload{
		AgentID: "test-agent", SystemID: "sys-1",
		CPU: payload.CPUInfo{UsagePct: 12.5}, Memory: payload.MemInfo{UsedPct: 45.0},
	}); err != nil {
		t.Fatalf("SaveMetrics: %v", err)
	}
}

func TestPostgresStore_SaveLogs(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveLogs(context.Background(), &payload.LogPayload{
		AgentID: "test-agent", Service: "api",
		Entries: []payload.LogEntry{{Level: "info", Message: "test", TraceID: "abc"}},
	}); err != nil {
		t.Fatalf("SaveLogs: %v", err)
	}
}

func TestPostgresStore_SaveAPM(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveAPM(context.Background(), &payload.APMPayload{
		AgentID: "test-agent", Service: "api",
		Spans: []payload.APMSpan{
			{SpanID: "s1", TraceID: "t1", Operation: "GET /health", DurationMs: 5.2, Status: "ok"},
		},
	}); err != nil {
		t.Fatalf("SaveAPM: %v", err)
	}
}
