# UniWatch Agent — Claude Code Instructions

## What This Repo Is
Two Go binaries that run on monitored servers and report to the UniWatch Hub.
- uniwatch-agent   — collects server metrics + heartbeat
- uniwatch-sidecar — observes HTTP traffic + forwards logs + manages trace IDs

Hub repo: github.com/idiey/uniwatch-hub
Do NOT add hub or dashboard code to this repo.

## Tech Stack
- Go 1.22 — both binaries
- zerolog — structured logging
- gopsutil/v3 — system metrics (CPU, memory, disk, network, processes)
- yaml.v3 — config file parsing
- google/uuid — trace ID generation

## Go Module Structure
Two modules in a Go workspace:
- agent/   — Binary 1: server metrics + heartbeat
- sidecar/ — Binary 2: HTTP observer + log forwarder

## What Is Already Built
agent/internal/config/
  config.go       YAML loader, env resolver, full validation
  config_test.go  3 passing tests

sidecar/internal/scrubber/
  scrubber.go     7 scrubbing rules — auth headers, credit cards, tokens
  scrubber_test.go  4 passing tests

sidecar/internal/traceid/
  traceid.go      Read trace ID from log or generate UUID
  traceid_test.go  2 passing tests

## What Is Empty — Sprint 2 Work
agent/internal/collector/   CPU, memory, disk, network, process collection
agent/internal/heartbeat/   30-second heartbeat worker
agent/internal/pusher/      HTTP push to hub + exponential backoff retry
agent/internal/buffer/      Disk-persisted buffer + replay on reconnect
sidecar/internal/tailer/    Access log + app log file tailers
sidecar/internal/apm/       APM span batcher + emitter
sidecar/internal/logforwarder/  Log line batcher + forwarder

## Coding Rules — MUST follow
1. gofmt all files before committing
2. zerolog for all logging — never fmt.Printf
3. context.Context as first arg on all functions calling external services
4. Handle ALL errors — never _ for error returns
5. Minimum 80% test coverage per package
6. Never hardcode secrets — env vars only
7. API key must never appear in logs, stdout, or ps output

## Git Rules
- Branch from develop: git checkout -b feature/<short-desc>
- Commit format: feat(scope): description
  Valid scopes: agent | sidecar
- Never push directly to main or develop

## Key Data Contracts

### Heartbeat payload — POST /ingest/heartbeat
```json
{
  "schema_version": "1.0",
  "agent_id": "<agent.id from config>",
  "payload_type": "heartbeat",
  "timestamp": "<ISO 8601 UTC>",
  "status": "alive",
  "agent_version": "<Version var>",
  "uptime_seconds": 0
}
```

### Metrics payload — POST /ingest/metrics
```json
{
  "schema_version": "1.0",
  "agent_id": "<id>",
  "system_id": "<id>",
  "platform": "<platform>",
  "environment": "<env>",
  "timestamp": "<ISO 8601 UTC>",
  "payload_type": "metrics",
  "host": { "hostname": "", "os": "", "uptime_seconds": 0, "agent_version": "" },
  "cpu": { "cores": 0, "usage_total_pct": 0.0, "usage_per_core": [] },
  "memory": { "total_bytes": 0, "used_bytes": 0, "free_bytes": 0, "used_pct": 0.0 },
  "disk": [{ "mount": "/", "total_bytes": 0, "used_bytes": 0, "free_bytes": 0, "used_pct": 0.0 }],
  "network": [{ "interface": "eth0", "bytes_in": 0, "bytes_out": 0, "packets_in": 0, "packets_out": 0 }],
  "processes": {
    "total_running": 0,
    "watched": [{ "name": "", "status": "running|stopped", "pid": 0, "cpu_pct": 0.0, "mem_mb": 0 }]
  }
}
```

## Hub Endpoints the Agent Calls
```
POST https://<hub>/ingest/heartbeat
POST https://<hub>/ingest/metrics
Headers: X-Agent-ID: <agent_id>  X-Agent-Key: <api_key>
```

## Retry Logic
```
Attempt 1 fail → wait 5s
Attempt 2 fail → wait 10s
Attempt 3 fail → wait 30s
Attempt 4+ fail → wait 60s (cap)
While retrying: keep collecting, buffer to disk
```

## Acceptance Criteria (ENG-SPEC-002)
| ID    | Criterion |
|-------|-----------|
| AC-01 | First heartbeat sent within 500ms of startup |
| AC-02 | Hub receives heartbeat every 30 seconds |
| AC-03 | Hub receives metrics every 60 seconds |
| AC-04 | CPU metric accurate within ±2% |
| AC-05 | Memory metric accurate within ±2% |
| AC-06 | Disk metric accurate within ±1% |
| AC-07 | Stopped service reported within 60 seconds of process kill |
| AC-08 | Restarted service reported within 60 seconds of restart |
| AC-09 | Buffer grows on disk when hub offline |
| AC-10 | Buffer replays on hub reconnect — no gaps |
| AC-11 | CPU usage < 2% sustained |
| AC-12 | RAM usage < 128MB |
| AC-13 | Auto-restarts within 10 seconds of crash |
| AC-14 | API key never in logs or ps output |
| AC-15 | TLS verify cannot be disabled without explicit config |

## Sprint 2 — What To Build
See docs/sprints/sprint-2-prompt.md
