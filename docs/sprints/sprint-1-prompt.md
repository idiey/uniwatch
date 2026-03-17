# Sprint 1 — Hub Infrastructure

**Repo**: `idiey/uniwatch-hub`
**Goal**: Hub is fully provisioned and all services pass `/health`.

Paste this prompt into Claude Code from the `uniwatch-hub` directory:

---

Read CLAUDE.md first. Then complete Sprint 1.

Sprint 1 goal: Hub is fully provisioned and all services pass /health.

Work through these tasks in order:

## TASK 1 — Fix go.mod module paths

Update all go.mod files to replace "github.com/your-org" with "github.com/idiey".
Files to update:
  hub/collector-engine/go.mod
  hub/log-pipeline/go.mod
  hub/alert-engine/go.mod
  hub/api-server/go.mod
Update all import paths in .go files to match.

## TASK 2 — Complete collector-engine routing

In hub/collector-engine/cmd/main.go, wire up all ingest routes:
  POST /ingest/heartbeat → handler.HeartbeatHandler
  POST /ingest/metrics   → handler.MetricsHandler
  POST /ingest/logs      → handler.LogsHandler
  POST /ingest/apm       → handler.APMHandler
All routes must use the X-Agent-Key middleware for authentication.
Read the existing handler files in hub/collector-engine/internal/handler/ first.

## TASK 3 — Implement PostgreSQL store for collector-engine

Create hub/collector-engine/internal/store/postgres.go
Implement the Store interface from interfaces.go using lib/pq.
The store must:
  - Accept DATABASE_URL from environment
  - Save heartbeat records to a heartbeats table (create if missing)
  - Save metrics records to the metrics table (already created by migrations)
  - Save log lines to the logs table
  - Save APM spans to the apm_spans table
Write tests in postgres_test.go using testcontainers.

## TASK 4 — Complete log-pipeline query API

In hub/log-pipeline/internal/handler/query.go, implement:
  GET /api/logs with query params: service, level, trace_id, environment, from, to, search, limit, cursor
  GET /api/logs/:trace_id
  GET /api/logs/stats
Read hub/log-pipeline/internal/store/store.go for the interface to implement.
Implement the PostgreSQL store in hub/log-pipeline/internal/store/postgres.go.

## TASK 5 — Complete api-server auth flow

In hub/api-server/internal/auth/jwt.go (already exists), verify the JWT issue and validate functions are correct.
Add hub/api-server/internal/handler/login.go:
  POST /api/auth/login — accepts username + password, validates against users table, issues JWT cookie
Add hub/api-server/internal/middleware/auth.go:
  Middleware that validates JWT cookie on all /api/* routes except /api/auth/login

## TASK 6 — Wire api-server routes

In hub/api-server/cmd/main.go, register:
  POST /api/auth/login  → login handler (no auth)
  GET  /api/fleet       → fleet handler (stub returning mock data for now)
  GET  /health          → health handler

## TASK 7 — Run all tests

Run: make test-go
All tests must pass. Fix any failures before marking Sprint 1 complete.

## TASK 8 — Verify all services build

Run: make build
All 4 binaries must compile without errors.

---

After completing all tasks, commit with:
```
git add .
git commit -m "feat(hub): complete Sprint 1 - all hub services operational"
git push origin develop
```
