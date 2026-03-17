package store

import (
	"context"

	"github.com/idiey/collector-engine/internal/payload"
)

// Store defines the persistence interface for collector payloads.
type Store interface {
	SaveHeartbeat(ctx context.Context, p *payload.HeartbeatPayload) error
	SaveMetrics(ctx context.Context, p *payload.MetricsPayload) error
	SaveLogs(ctx context.Context, p *payload.LogPayload) error
	SaveAPM(ctx context.Context, p *payload.APMPayload) error
}
