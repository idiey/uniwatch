# Sprint 3 — Log Pipeline + Full Estate Rollout

**Repo**: `idiey/uniwatch-hub`
**Goal**: Full log pipeline operational with archive engine.

Paste this prompt into Claude Code from the `uniwatch-hub` directory:

---

Read CLAUDE.md. Complete Sprint 3.

## Log Store Implementation

Implement the full PostgreSQL log store in hub/log-pipeline/internal/store/postgres.go:
- Bulk insert with 2-second or 500-row write buffer
- Query with all filters: service, level, trace_id, environment, from, to, full-text search
- Cursor-based pagination
- GET /api/logs/stats aggregate counts

## Archive Engine

Implement the nightly archive engine in hub/log-pipeline/internal/archive/archive.go:
- Query logs WHERE received_at < NOW() - 30 days AND archived = FALSE
- Serialise to NDJSON, compress with gzip
- Upload to DigitalOcean Spaces at /logs/{env}/{service}/{YYYY}/{MM}/{DD}.ndjson.gz
- On success: mark archived = TRUE, then DELETE
- Schedule at 02:00 SGT using a Go cron scheduler
- On failure: log CRITICAL, alert fires, do NOT delete

Run all tests. Build must pass.

---

Commit:
```
git add .
git commit -m "feat(log-pipeline): complete Sprint 3 - log pipeline and archive engine"
git push origin develop
```
