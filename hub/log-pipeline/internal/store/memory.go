package store

import (
	"context"
	"strconv"
	"strings"
	"sync"
)

type MemoryLogStore struct {
	mu   sync.RWMutex
	rows []LogRow
	next int64
}

func NewMemoryLogStore() *MemoryLogStore { return &MemoryLogStore{} }

func (m *MemoryLogStore) Add(row LogRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	row.ID = m.next
	m.rows = append(m.rows, row)
}

func (m *MemoryLogStore) Query(_ context.Context, f LogFilter) ([]LogRow, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var cursorID int64
	if f.Cursor != "" {
		cursorID, _ = strconv.ParseInt(f.Cursor, 10, 64)
	}
	var result []LogRow
	for _, row := range m.rows {
		if cursorID > 0 && row.ID <= cursorID {
			continue
		}
		if f.Service != "" && row.Service != f.Service {
			continue
		}
		if f.Level != "" && row.Level != f.Level {
			continue
		}
		if f.TraceID != "" && row.TraceID != f.TraceID {
			continue
		}
		if f.Environment != "" && row.Environment != f.Environment {
			continue
		}
		if f.Search != "" && !strings.Contains(row.Message, f.Search) {
			continue
		}
		result = append(result, row)
		if len(result) >= limit {
			break
		}
	}
	nextCursor := ""
	if len(result) == limit {
		nextCursor = strconv.FormatInt(result[len(result)-1].ID, 10)
	}
	return result, nextCursor, nil
}

func (m *MemoryLogStore) GetByTraceID(_ context.Context, traceID string) ([]LogRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []LogRow
	for _, row := range m.rows {
		if row.TraceID == traceID {
			result = append(result, row)
		}
	}
	return result, nil
}

func (m *MemoryLogStore) Stats(_ context.Context) (LogStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := LogStats{}
	svcSet := map[string]struct{}{}
	for _, row := range m.rows {
		stats.Total++
		switch row.Level {
		case "error":
			stats.Errors++
		case "warn", "warning":
			stats.Warnings++
		case "info":
			stats.Info++
		}
		svcSet[row.Service] = struct{}{}
	}
	for svc := range svcSet {
		stats.Services = append(stats.Services, svc)
	}
	return stats, nil
}
