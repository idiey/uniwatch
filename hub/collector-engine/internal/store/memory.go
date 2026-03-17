package store

import (
	"context"
	"sync"

	"github.com/idiey/collector-engine/internal/payload"
)

// MemoryStore is a thread-safe in-memory implementation of Store.
type MemoryStore struct {
	mu         sync.RWMutex
	heartbeats []*payload.HeartbeatPayload
	metrics    []*payload.MetricsPayload
	logs       []*payload.LogPayload
	apm        []*payload.APMPayload
}

// NewMemoryStore creates a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// SaveHeartbeat stores a heartbeat payload in memory.
func (m *MemoryStore) SaveHeartbeat(_ context.Context, p *payload.HeartbeatPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeats = append(m.heartbeats, p)
	return nil
}

// SaveMetrics stores a metrics payload in memory.
func (m *MemoryStore) SaveMetrics(_ context.Context, p *payload.MetricsPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, p)
	return nil
}

// SaveLogs stores a log payload in memory.
func (m *MemoryStore) SaveLogs(_ context.Context, p *payload.LogPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, p)
	return nil
}

// SaveAPM stores an APM payload in memory.
func (m *MemoryStore) SaveAPM(_ context.Context, p *payload.APMPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apm = append(m.apm, p)
	return nil
}

// LenHeartbeats returns the number of stored heartbeats.
func (m *MemoryStore) LenHeartbeats() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.heartbeats)
}

// LenMetrics returns the number of stored metrics payloads.
func (m *MemoryStore) LenMetrics() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.metrics)
}

// LenLogs returns the number of stored log payloads.
func (m *MemoryStore) LenLogs() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.logs)
}

// LenAPM returns the number of stored APM payloads.
func (m *MemoryStore) LenAPM() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.apm)
}
