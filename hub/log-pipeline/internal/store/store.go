package store

import (
	"context"
	"encoding/json"
	"time"
)

type LogFilter struct {
	Service     string
	Level       string
	TraceID     string
	Environment string
	From        time.Time
	To          time.Time
	Search      string
	Limit       int
	Cursor      string
}

type LogRow struct {
	ID          int64           `json:"id"`
	AgentID     string          `json:"agent_id"`
	Service     string          `json:"service"`
	Environment string          `json:"environment"`
	Level       string          `json:"level"`
	Message     string          `json:"message"`
	TraceID     string          `json:"trace_id"`
	Fields      json.RawMessage `json:"fields"`
	ReceivedAt  time.Time       `json:"received_at"`
}

type LogStats struct {
	Total    int64    `json:"total"`
	Errors   int64    `json:"errors"`
	Warnings int64    `json:"warnings"`
	Info     int64    `json:"info"`
	Services []string `json:"services"`
}

type LogStore interface {
	Query(ctx context.Context, f LogFilter) (rows []LogRow, nextCursor string, err error)
	GetByTraceID(ctx context.Context, traceID string) ([]LogRow, error)
	Stats(ctx context.Context) (LogStats, error)
}
