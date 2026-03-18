-- UniWatch Hub Initial Schema (requires TimescaleDB)
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Default admin (password: admin -- CHANGE IN PRODUCTION)
INSERT INTO users (username, password)
VALUES ('admin', '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj3T0e4O5R8.')
ON CONFLICT (username) DO NOTHING;

CREATE TABLE IF NOT EXISTS heartbeats (
    id BIGSERIAL, agent_id TEXT NOT NULL, status TEXT NOT NULL,
    agent_version TEXT, uptime_seconds INT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, received_at)
);
SELECT create_hypertable('heartbeats', 'received_at', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS metrics (
    id BIGSERIAL, agent_id TEXT NOT NULL, system_id TEXT NOT NULL,
    platform TEXT, environment TEXT, cpu_usage_pct DOUBLE PRECISION,
    memory_used_pct DOUBLE PRECISION, data JSONB,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, received_at)
);
SELECT create_hypertable('metrics', 'received_at', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS logs (
    id BIGSERIAL, agent_id TEXT NOT NULL, service TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT 'production',
    level TEXT NOT NULL, message TEXT NOT NULL, trace_id TEXT, fields JSONB,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, received_at)
);
SELECT create_hypertable('logs', 'received_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_logs_service  ON logs (service, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_logs_level    ON logs (level, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_logs_trace    ON logs (trace_id) WHERE trace_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_logs_fts      ON logs USING gin(to_tsvector('english', message));

CREATE TABLE IF NOT EXISTS apm_spans (
    id BIGSERIAL, agent_id TEXT NOT NULL, service TEXT NOT NULL,
    span_id TEXT NOT NULL, trace_id TEXT NOT NULL, parent_id TEXT,
    operation TEXT NOT NULL, duration_ms DOUBLE PRECISION NOT NULL,
    status TEXT NOT NULL DEFAULT 'ok', tags JSONB,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, received_at)
);
SELECT create_hypertable('apm_spans', 'received_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_apm_trace ON apm_spans (trace_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_apm_svc   ON apm_spans (service, received_at DESC);
