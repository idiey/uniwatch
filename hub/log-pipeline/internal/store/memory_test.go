package store_test

import (
	"context"
	"testing"

	"github.com/idiey/log-pipeline/internal/store"
)

func TestMemoryLogStore_QueryByService(t *testing.T) {
	s := store.NewMemoryLogStore()
	s.Add(store.LogRow{Service: "api", Level: "info", Message: "a"})
	s.Add(store.LogRow{Service: "api", Level: "error", Message: "b"})
	s.Add(store.LogRow{Service: "worker", Level: "info", Message: "c"})
	rows, _, err := s.Query(context.Background(), store.LogFilter{Service: "api"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2, got %d", len(rows))
	}
}

func TestMemoryLogStore_Pagination(t *testing.T) {
	s := store.NewMemoryLogStore()
	for i := 0; i < 5; i++ {
		s.Add(store.LogRow{Service: "api", Level: "info", Message: "msg"})
	}
	rows, cursor, err := s.Query(context.Background(), store.LogFilter{Limit: 3})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3, got %d", len(rows))
	}
	if cursor == "" {
		t.Fatal("expected cursor")
	}
	rows2, _, err := s.Query(context.Background(), store.LogFilter{Limit: 3, Cursor: cursor})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(rows2) != 2 {
		t.Fatalf("expected 2 on page 2, got %d", len(rows2))
	}
}

func TestMemoryLogStore_GetByTraceID(t *testing.T) {
	s := store.NewMemoryLogStore()
	s.Add(store.LogRow{TraceID: "t1", Level: "info", Message: "a"})
	s.Add(store.LogRow{TraceID: "t2", Level: "info", Message: "b"})
	s.Add(store.LogRow{TraceID: "t1", Level: "error", Message: "c"})
	rows, err := s.GetByTraceID(context.Background(), "t1")
	if err != nil {
		t.Fatalf("GetByTraceID: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2, got %d", len(rows))
	}
}

func TestMemoryLogStore_Stats(t *testing.T) {
	s := store.NewMemoryLogStore()
	s.Add(store.LogRow{Service: "api", Level: "info"})
	s.Add(store.LogRow{Service: "api", Level: "error"})
	s.Add(store.LogRow{Service: "worker", Level: "warn"})
	stats, err := s.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Total != 3 {
		t.Fatalf("expected 3 total, got %d", stats.Total)
	}
	if stats.Errors != 1 {
		t.Fatalf("expected 1 error, got %d", stats.Errors)
	}
	if stats.Warnings != 1 {
		t.Fatalf("expected 1 warning, got %d", stats.Warnings)
	}
}
