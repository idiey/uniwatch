# Sprint 2 — Agent + Sidecar Implementation

**Repo**: `idiey/uniwatch-agent`
**Goal**: Agent and sidecar fully implemented — data flowing to hub.

Paste this prompt into Claude Code from the `uniwatch-agent` directory:

---

Read CLAUDE.md first. Then complete Sprint 2.

Sprint 2 goal: Agent and sidecar fully implemented — data flowing to hub.

Work through these tasks in order:

## TASK 1 — Fix go.mod module paths

Update agent/go.mod and sidecar/go.mod:
  Replace "github.com/your-org" with "github.com/idiey"
Update all import paths in .go files to match.

## TASK 2 — Implement agent/internal/collector

Create these files:
  collector.go    — Collector struct, Start/Stop methods, collects all metrics on 60s interval
  cpu.go          — CPU usage per core using gopsutil/v3/cpu
  memory.go       — Memory stats using gopsutil/v3/mem
  disk.go         — Disk usage per mount using gopsutil/v3/disk
  network.go      — Network bytes in/out using gopsutil/v3/net
  process.go      — Process list + watched service status using gopsutil/v3/process
Write table-driven tests for each collector achieving 80% coverage minimum.

## TASK 3 — Implement agent/internal/heartbeat

Create heartbeat.go:
  Worker struct with Start(ctx) and Stop() methods
  Sends heartbeat payload to POST /ingest/heartbeat every 30 seconds
  Uses the hub endpoint and API key from config
  Handles connection errors gracefully — logs warning, retries on next tick
Write tests with a mock HTTP server.

## TASK 4 — Implement agent/internal/pusher

Create pusher.go:
  Pusher struct — reads from a channel, sends to POST /ingest/metrics
  Implements exponential backoff: 5s, 10s, 30s, 60s cap
  On failure: writes payload to buffer (see Task 5)
  On 401 response: logs CRITICAL and stops — does not retry
  On 429 response: backs off 60 seconds
  TLS 1.3 minimum — honours tls_verify config
Write tests with a mock HTTP server covering all retry scenarios.

## TASK 5 — Implement agent/internal/buffer

Create buffer.go:
  Buffer struct — disk-persisted queue of JSON payloads
  Methods: Enqueue(payload), Dequeue() (payload, ok), Len() int, Flush(pusher)
  Max capacity from config (default 1000) — on overflow drop oldest, log WARNING
  Must survive process restart — reads existing buffer file on init
  Replay: oldest-first, batches of 50
Write tests covering overflow, persistence, and replay.

## TASK 6 — Wire agent/cmd/agent/main.go

Initialise in order:
  1. Load config (already implemented in internal/config)
  2. Set up zerolog file logger using config.Logging
  3. Start buffer
  4. Start pusher (reads from collector output channel)
  5. Start collector (pushes to channel)
  6. Start heartbeat worker
  7. Block on context cancellation
  8. Graceful shutdown: stop collector, flush buffer, stop pusher

## TASK 7 — Implement sidecar/internal/tailer

Create tailer.go:
  Tailer struct — tails a file line by line using os.File + bufio.Scanner
  Detects log rotation (file replaced) — reopens within 5 seconds
  Emits lines on a channel for downstream processing
Write tests using temporary files and simulated rotation.

## TASK 8 — Implement sidecar/internal/apm

Create emitter.go:
  Emitter struct — receives APM spans on a channel
  Batches spans, flushes every 5 seconds to POST /ingest/apm
  Uses buffer on failure (same pattern as agent pusher)
Write tests with mock HTTP server.

## TASK 9 — Implement sidecar/internal/logforwarder

Create forwarder.go:
  Forwarder struct — receives log lines on a channel
  Parses each line as JSON, extracts trace_id using sidecar/internal/traceid
  Applies scrubbing using sidecar/internal/scrubber
  Batches and flushes every 5 seconds or 100 lines to POST /ingest/logs
Write tests covering scrubbing, trace ID injection, and batching.

## TASK 10 — Wire sidecar/cmd/sidecar/main.go

Wire tailer → logforwarder and tailer → apm emitter pipeline.

## TASK 11 — Run all tests

Run: make test
All tests must pass with coverage >= 80% per package.

## TASK 12 — Build all platforms

Run: make build-all
All 6 binaries (linux-amd64, linux-arm64, windows-amd64 for agent and sidecar) must compile.

---

Commit with:
```
git add .
git commit -m "feat(agent): complete Sprint 2 - agent and sidecar fully implemented"
git push origin develop
```
