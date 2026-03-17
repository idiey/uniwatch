# UniWatch Architecture Review

## Overview

UniWatch is a self-hosted observability platform composed of two repositories:

| Repository | Role | Deployment |
|---|---|---|
| `idiey/uniwatch-hub` | Server-side ingestion, storage, alerting, and dashboard | You → DigitalOcean |
| `idiey/uniwatch-agent` | Lightweight agent + log sidecar installed on monitored hosts | GitHub Actions on `git tag v*.*.*` |

---

## Repository: `idiey/uniwatch-hub`

### Service Architecture

The hub is decomposed into four independent Go microservices, each with its own `go.mod` and HTTP server, coordinated via a `go.work` workspace.

```
collector-engine   :  POST /ingest/{heartbeat,metrics,logs,apm}
log-pipeline       :  GET  /api/logs
alert-engine       :  rule evaluation + notification dispatch
api-server         :  JWT auth + future REST API surface
```

#### `hub/collector-engine`

- **Responsibility**: Primary ingest gateway. Accepts heartbeat, metrics, log, and APM payloads from agents.
- **Auth**: `X-Agent-Key` header validated by `middleware/auth.go`.
- **Storage**: Abstracted behind `store/interfaces.go`; current implementation is in-memory (`store/memory.go`) suitable for dev/test. Production path points to TimescaleDB/PostgreSQL via the migration set.
- **Strengths**: Clean separation of handler, middleware, payload types, and store layers. The interface-backed store makes swapping to a real DB straightforward.
- **Gaps**:
  - No rate limiting or per-agent throttle — a misbehaving agent can flood the ingest path.
  - No payload size cap enforced in middleware; large payloads could exhaust memory.
  - In-memory store has no persistence across restarts; production wiring to PostgreSQL is not yet implemented.

#### `hub/log-pipeline`

- **Responsibility**: Query layer for stored logs (`GET /api/logs`).
- **Store**: `store/store.go` defines a PostgreSQL interface, but the implementation is a skeleton.
- **Gaps**:
  - `handler/query.go` is a skeleton; filtering, pagination, and full-text search (indexes exist in migration `002`) are not yet wired.
  - No streaming or cursor-based pagination design defined — important at scale.

#### `hub/alert-engine`

- **Responsibility**: Evaluates alert rules against ingested data; dispatches notifications.
- **Status**: Both `evaluator/evaluator.go` and `notifier/notifier.go` are skeletons.
- **Migration `004`**: 8 alert rules are seeded. The evaluation loop that reads and acts on these rules has not been implemented yet.
- **Gaps**:
  - No scheduler or ticker driving periodic rule evaluation.
  - No deduplication / alert suppression logic.
  - Notifier channels (Slack, email, PagerDuty, etc.) are not implemented.

#### `hub/api-server`

- **Responsibility**: JWT issuance and validation; future REST API surface for the dashboard.
- **JWT**: `auth/jwt.go` and tests exist — this is the most complete non-ingest service.
- **Gaps**:
  - Only `/health` is wired. No routes for users, agents, dashboards, or alert rule management.
  - No RBAC model defined; `005_create_users.sql` creates `users` + `audit_log` but no roles table.

### Database / Migrations

| File | Table | Notes |
|---|---|---|
| `001_create_metrics.sql` | `metrics` | TimescaleDB hypertable + indexes |
| `002_create_logs.sql` | `logs` | 7 indexes + full-text search column |
| `003_create_apm.sql` | `apm_spans` | APM trace storage |
| `004_create_alerts.sql` | `alert_rules`, `alerts` | 8 seeded rules |
| `005_create_users.sql` | `users`, `audit_log` | Auth + audit |

**Strengths**: TimescaleDB hypertable on `metrics` is the right choice for time-series cardinality. FTS on `logs` enables `to_tsvector` queries. All tables are created before they are needed.

**Gaps**:
- No `DOWN` migration scripts; rollbacks require manual intervention.
- `run.sh` migration runner does not appear to support idempotent re-runs (no schema version table referenced in the file listing).
- Foreign key relationships between `alert_rules`, `alerts`, and `users` are not described — the on-delete behaviour needs review.
- No partitioning strategy documented for `logs` and `apm_spans` beyond TimescaleDB's own chunking.

### Dashboard (`dashboard/`)

- React 18 + Vite + Tailwind + Recharts.
- `vite.config.js` proxies `/api` to the Go API server during development.
- `App.jsx` has all 5 routes wired to view stubs.
- `ProtectedRoute.jsx` and `api/client.js` provide the auth skeleton.
- **All 6 view components are Sprint 4 stubs**: `FleetOverview`, `SystemDrilldown`, `LogExplorer`, `ApmExplorer`, `AlertInbox`, `Login`.
- `hooks/` directory is empty.

**Gaps**:
- No token refresh / silent re-auth logic in `api/client.js`.
- No error boundary components.
- No loading/skeleton states defined.
- Recharts is included but no chart components built yet.

### Infrastructure (`infra/`)

- **DigitalOcean**: Droplet provisioning + Ubuntu hardening + Keepalived HA with floating IP failover.
- **Nginx**: TLS 1.3, security headers, reverse proxy to Go services.
- **Systemd**: Unit files for all 5 services.
- **Scripts**: `deploy.sh` (rolling deploy) + `install-agent.sh`.

**Strengths**: HA via Keepalived floating IP is a practical, low-cost approach for a two-node setup.

**Gaps**:
- `keepalived-primary.conf` implies a secondary node config exists elsewhere — the secondary config is not in the file listing.
- No firewall (`ufw`/`iptables`) rules for inter-service communication are visible.
- Systemd units for `collector-engine`, `log-pipeline`, `alert-engine`, `api-server` are present but the unit for `dashboard` (static file serving / Nginx config) is not listed.
- No health-check restart policies (`Restart=on-failure`, `RestartSec`) verified in unit files.

---

## Repository: `idiey/uniwatch-agent`

### Agent Architecture

Two independent binaries in one repo, sharing a `go.work` workspace:

```
agent/    — system metrics collector + heartbeat + push
sidecar/  — log tailer + APM span emitter + forwarder
```

#### `agent/`

- **Config**: `config/config.go` — YAML loader with env-variable resolution and validation. Tests cover 3 cases. Well structured.
- **Sprint 2 gaps** (all stubs):
  - `collector/` — CPU, memory, disk, network collection
  - `heartbeat/` — 30-second heartbeat worker
  - `pusher/` — HTTP push with retry
  - `buffer/` — disk buffer + replay on reconnect

#### `sidecar/`

- **Scrubber** (`scrubber/scrubber.go`): 7 scrubbing rules (PII/secret redaction). 4 tests. Functional and production-relevant.
- **TraceID** (`traceid/traceid.go`): Read or generate W3C-style trace IDs. 2 tests. Correct approach for distributed tracing correlation.
- **Sprint 2 gaps** (all stubs):
  - `tailer/` — log file tailing (likely `fsnotify` or `tail` library)
  - `apm/` — APM span emitter
  - `logforwarder/` — batched log forwarder

### CI / Release

- `ci.yml`: lint, test, build, security scan on every push.
- `release.yml`: auto-publishes binary on `git tag v*.*.*` via GitHub Releases.
- `install-agent.sh`: pulls from GitHub Releases — correctly decoupled from source.

---

## Cross-Cutting Concerns

### Security

| Concern | Status |
|---|---|
| Agent → Hub auth (`X-Agent-Key`) | Implemented in middleware |
| User → Dashboard auth (JWT) | JWT logic implemented, routes not wired |
| TLS termination | Nginx config present |
| Secret management | `.env.example` pattern; no vault/secrets manager |
| PII scrubbing in logs | 7 rules in sidecar scrubber |
| Ubuntu hardening | `harden.sh` present |

**Recommendations**:
1. The `X-Agent-Key` is a shared secret. Consider per-agent keys stored in the `users` / agent table so a compromised host can be revoked individually.
2. JWT secret must not be committed; confirm it is sourced from env only.
3. Introduce a secrets manager (e.g., DigitalOcean Secrets, Vault, or `age`-encrypted `.env`) before production.

### Observability of the Hub Itself

The hub has `/health` endpoints per service but no internal metrics (Prometheus, OpenTelemetry) or structured access logs emitted by the Go services. This means the hub is blind to its own performance — ironic for an observability platform. Add at minimum:
- Prometheus `/metrics` endpoint per service.
- Structured JSON logging with request duration and status.

### Inter-Service Communication

There is no message broker (Kafka, NATS, Redis Streams) between `collector-engine` and `alert-engine`/`log-pipeline`. The current implied flow is:

```
agent → collector-engine (HTTP) → PostgreSQL ← alert-engine (polling) ← log-pipeline (query)
```

For Sprint 1–2 scale this is fine. At higher ingest rates, consider a queue between the collector and downstream consumers to decouple ingest throughput from processing latency.

### Configuration & Environment

- Both repos use `.env.example` patterns.
- The agent uses YAML with env-variable interpolation (`config.go`), which is idiomatic.
- The hub services do not appear to have a unified config loader — each service likely reads env vars directly. A shared internal config package (in `go.work`) would reduce duplication.

### Testing Coverage

| Component | Tests |
|---|---|
| `collector-engine/handler` | Yes (`heartbeat_test.go`) |
| `api-server/auth` | Yes (`jwt_test.go`) |
| `agent/config` | Yes (3 tests) |
| `sidecar/scrubber` | Yes (4 tests) |
| `sidecar/traceid` | Yes (2 tests) |
| All Sprint 2 components | No tests (stubs) |
| Dashboard views | No tests (stubs) |

All tested components are Sprint 1 deliverables. Sprint 2 implementation should be accompanied by tests from the start given the CI gate in both repos.

---

## Sprint Readiness Assessment

### Sprint 2 (Next)

The critical path for a working end-to-end demo is:

1. **Agent**: implement `collector` → `pusher` (with retry) → `heartbeat` worker. `buffer` can follow.
2. **Sidecar**: implement `tailer` → `logforwarder`. APM span emitter is lower priority.
3. **Hub collector-engine**: wire `store/memory.go` or a real PostgreSQL store so ingested data persists.
4. **Hub log-pipeline**: implement `query.go` (at minimum: filter by agent ID + time range + limit).
5. **Hub alert-engine**: implement a basic evaluation tick (check heartbeat age → fire "agent offline" alert).

### Sprint 3

- `api-server`: add routes for listing agents, metrics, alerts; integrate with JWT auth.
- Dashboard: implement `Login.jsx`, `FleetOverview.jsx`.

### Sprint 4

- Dashboard: remaining views (`LogExplorer`, `ApmExplorer`, `AlertInbox`, `SystemDrilldown`).
- `hooks/` and chart components.

---

## Summary of Architectural Strengths

- Clean microservice decomposition with interface-backed stores — easy to test and swap.
- TimescaleDB for metrics is the right data model choice.
- Scrubber and trace-ID generation are well-implemented and tested.
- HA infrastructure design (Keepalived + floating IP) is pragmatic for the target deployment size.
- CI/CD pipeline in both repos enforces lint, tests, and security scan before merge.

## Top Recommendations

1. **Implement per-agent keys** rather than a single `X-Agent-Key` to allow individual host revocation.
2. **Add Prometheus metrics + structured logging** to all hub services before Sprint 3.
3. **Add DOWN migrations** to the migration set for safe rollbacks.
4. **Define the alert evaluation tick** in Sprint 2 — the seeded rules in `004` are wasted until an evaluator runs.
5. **Implement token refresh** in `api/client.js` before the dashboard login flow is built.
6. **Add a schema version/migration table** to `run.sh` to prevent double-applying migrations.
