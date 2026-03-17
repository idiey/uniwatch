# UniWatch Hub — Claude Code Instructions

## What This Repo Is
The server-side hub for UniWatch Unified Observability Platform.
Receives metrics, logs, and APM data from agents deployed on monitored servers.
Serves the React dashboard. Runs on DigitalOcean SGP1 (Active/Passive HA).

## Companion Repo
Agent and sidecar binaries live at: github.com/idiey/uniwatch-agent
Do NOT add agent or sidecar code to this repo.

## Tech Stack
- Go 1.22 — all backend services
- React 18 + Tailwind CSS + Recharts + Vite — dashboard
- PostgreSQL + TimescaleDB — metrics and log storage
- chi — HTTP router for all Go services
- zerolog — structured logging (never use fmt.Printf for logs)
- jwt (golang-jwt/jwt) — dashboard authentication

## Go Module Structure
This is a Go workspace. Four modules:
- hub/collector-engine  — port 9090 — accepts agent ingest
- hub/log-pipeline      — port 9091 — log query API
- hub/alert-engine      — port 9092 — alert rules + notifications
- hub/api-server        — port 8080 — dashboard REST API

## Coding Rules — MUST follow
1. gofmt all Go files before committing
2. Use zerolog for all logging — structured key-value pairs only
3. Every function calling external services takes context.Context as first arg
4. Handle ALL errors explicitly — never use _ for error returns
5. Minimum 80% test coverage per package — CI will fail below this
6. Never hardcode secrets — environment variables only
7. All API calls in dashboard go through dashboard/src/api/client.js
8. React: functional components + hooks only — no class components
9. Tailwind utility classes only — no custom CSS files

## Git Rules — MUST follow
- Branch from develop: git checkout -b feature/<short-desc>
- Commit format: feat(scope): description
  Valid scopes: hub | dashboard | infra | log-pipeline | alert-engine | api-server | collector-engine | migrations
- Never push directly to main or develop

## Environment Variables
Copy .env.example to .env.local and fill in values.
Required for local dev:
  DATABASE_URL=postgres://postgres:dev@localhost:5432/uniwatch?sslmode=disable
  JWT_SECRET=any-random-string-for-dev

## Local Dev Setup
1. docker run -d --name uniwatch-db -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=uniwatch -p 5432:5432 timescale/timescaledb:latest-pg16
2. cd hub/migrations && ./run.sh --env dev
3. make dev
4. Dashboard at http://localhost:5173

## Sprint Status
Sprint 1 — Hub Infrastructure (current)
Sprint 2 — Agent + Sidecar (agent repo)
Sprint 3 — Log Pipeline + APM Store
Sprint 4 — Dashboard + Alert Engine

## What Is Complete
- All database migrations (hub/migrations/)
- Collector engine ingest handlers (POST /ingest/*)
- Middleware: X-Agent-Key auth, request context
- Payload type definitions
- In-memory store for dev/testing
- JWT auth skeleton
- React app scaffold with routing
- All 5 view stubs

## Sprint 1 — What To Build
See docs/sprints/sprint-1-prompt.md
