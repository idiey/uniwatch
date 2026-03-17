package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	// PostgreSQL driver
	_ "github.com/lib/pq"

	"github.com/idiey/collector-engine/internal/payload"
)

// PostgresStore is a PostgreSQL-backed implementation of Store.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore opens a PostgreSQL connection and creates tables if they do not exist.
// Schema for heartbeats:
//
//	CREATE TABLE IF NOT EXISTS heartbeats (
//	    id             SERIAL PRIMARY KEY,
//	    agent_id       TEXT NOT NULL,
//	    status         TEXT NOT NULL,
//	    agent_version  TEXT,
//	    uptime_seconds INT,
//	    received_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
//	);
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if err = createTables(ctx, db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

func createTables(ctx context.Context, db *sql.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS heartbeats (
    id             SERIAL PRIMARY KEY,
    agent_id       TEXT NOT NULL,
    status         TEXT NOT NULL,
    agent_version  TEXT,
    uptime_seconds INT,
    received_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS metrics (
    id              BIGSERIAL PRIMARY KEY,
    agent_id        TEXT NOT NULL,
    system_id       TEXT NOT NULL,
    platform        TEXT,
    environment     TEXT,
    cpu_usage_pct   DOUBLE PRECISION,
    memory_used_pct DOUBLE PRECISION,
    data            JSONB,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS logs (
    id          BIGSERIAL PRIMARY KEY,
    agent_id    TEXT NOT NULL,
    service     TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT 'production',
    level       TEXT NOT NULL,
    message     TEXT NOT NULL,
    trace_id    TEXT,
    fields      JSONB,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS apm_spans (
    id          BIGSERIAL PRIMARY KEY,
    agent_id    TEXT NOT NULL,
    service     TEXT NOT NULL,
    span_id     TEXT NOT NULL,
    trace_id    TEXT NOT NULL,
    parent_id   TEXT,
    operation   TEXT NOT NULL,
    duration_ms DOUBLE PRECISION NOT NULL,
    status      TEXT NOT NULL DEFAULT 'ok',
    tags        JSONB,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

// SaveHeartbeat inserts a heartbeat record into PostgreSQL.
func (s *PostgresStore) SaveHeartbeat(ctx context.Context, p *payload.HeartbeatPayload) error {
	const q = `INSERT INTO heartbeats (agent_id, status, agent_version, uptime_seconds)
	           VALUES ($1, $2, $3, $4)`
	_, err := s.db.ExecContext(ctx, q, p.AgentID, p.Status, p.AgentVersion, p.UptimeSeconds)
	if err != nil {
		return fmt.Errorf("insert heartbeat: %w", err)
	}
	return nil
}

// SaveMetrics inserts a simplified metrics record into PostgreSQL.
func (s *PostgresStore) SaveMetrics(ctx context.Context, p *payload.MetricsPayload) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}
	const q = `INSERT INTO metrics (agent_id, system_id, platform, environment, cpu_usage_pct, memory_used_pct, data)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err = s.db.ExecContext(ctx, q,
		p.AgentID, p.SystemID, p.Platform, p.Environment,
		p.CPU.UsagePct, p.Memory.UsedPct, data,
	)
	if err != nil {
		return fmt.Errorf("insert metrics: %w", err)
	}
	return nil
}

// SaveLogs inserts each log entry from the payload into PostgreSQL.
func (s *PostgresStore) SaveLogs(ctx context.Context, p *payload.LogPayload) error {
	const q = `INSERT INTO logs (agent_id, service, environment, level, message, trace_id, fields)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, entry := range p.Entries {
		fields, err := json.Marshal(entry.Fields)
		if err != nil {
			return fmt.Errorf("marshal log fields: %w", err)
		}
		_, err = s.db.ExecContext(ctx, q,
			p.AgentID, p.Service, p.Environment,
			entry.Level, entry.Message, entry.TraceID, fields,
		)
		if err != nil {
			return fmt.Errorf("insert log entry: %w", err)
		}
	}
	return nil
}

// SaveAPM inserts each APM span from the payload into PostgreSQL.
func (s *PostgresStore) SaveAPM(ctx context.Context, p *payload.APMPayload) error {
	const q = `INSERT INTO apm_spans (agent_id, service, span_id, trace_id, parent_id, operation, duration_ms, status, tags)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	for _, span := range p.Spans {
		tags, err := json.Marshal(span.Tags)
		if err != nil {
			return fmt.Errorf("marshal span tags: %w", err)
		}
		_, err = s.db.ExecContext(ctx, q,
			p.AgentID, p.Service, span.SpanID, span.TraceID, span.ParentID,
			span.Operation, span.DurationMs, span.Status, tags,
		)
		if err != nil {
			return fmt.Errorf("insert apm span: %w", err)
		}
	}
	return nil
}
