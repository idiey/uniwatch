# Sprint 4 — Dashboard + Alert Engine

**Repo**: `idiey/uniwatch-hub`
**Goal**: Full platform operational — dashboard, alerts, all API routes.

Paste this prompt into Claude Code from the `uniwatch-hub` directory:

---

Read CLAUDE.md. Complete Sprint 4.

## BACKEND — Alert Engine

Implement hub/alert-engine/internal/evaluator/evaluator.go:
- Rule evaluation loop every 30 seconds
- Query metrics store for each active rule
- Create alert if threshold breached and no active alert exists
- Auto-resolve when threshold clears
- No duplicate notifications for same active alert

Implement hub/alert-engine/internal/notifier/notifier.go:
- Slack webhook POST with colour-coded attachment
- Email via SMTP
- SMS via webhook at T+15min for unacknowledged CRITICAL
- Retry 3x on failure, then log ERROR

## BACKEND — API Server Routes

Implement hub/api-server/internal/handler/ — all routes:
- GET  /api/fleet                              — all systems with latest health status
- GET  /api/systems/:id                        — drilldown metrics for one system
- GET  /api/alerts                             — list active + recent alerts
- POST /api/alerts/:id/acknowledge             — acknowledge an alert
- POST /api/alerts/:id/silence                 — silence an alert
- POST /api/alerts/:id/resolve                 — resolve an alert
- GET  /api/alert-rules                        — list all rules
- POST /api/alert-rules                        — create rule
- PUT  /api/alert-rules/:id                    — update rule
- DELETE /api/alert-rules/:id                  — delete rule

## FRONTEND — Views

Implement all views in dashboard/src/views/:

**FleetOverview.jsx**
- Summary bar (total systems, healthy, warning, critical counts)
- System table with RAG status colours
- 30-second auto-refresh

**SystemDrilldown.jsx**
- CPU, memory, disk gauges
- Recharts line graph (CPU + memory over time)
- Error rate bar chart
- Watched services table

**LogExplorer.jsx**
- Filter bar: service, level, trace_id, environment, from/to, full-text search
- Results table (timestamp, level, service, message)
- Expandable row for full log JSON
- trace_id cell links to ApmExplorer

**ApmExplorer.jsx**
- Endpoint rankings table (p50, p95, p99, error rate)
- Span tree for a selected trace
- Correlated log panel

**AlertInbox.jsx**
- Active alerts list with severity colours
- Acknowledge / Silence / Resolve action buttons
- Alert history tab

**Login.jsx**
- Username + password fields
- TOTP MFA second step
- Redirects to FleetOverview on success

## FRONTEND — Hooks

Implement all hooks in dashboard/src/hooks/:

**useFleet.js**   — polls /api/fleet every 30s, returns { systems, loading, error }
**useLogs.js**    — queries /api/logs with filter params, returns { logs, stats, fetchMore }
**useAPM.js**     — queries /api/apm/endpoints and traces, returns { endpoints, trace }
**useAlerts.js**  — polls /api/alerts every 30s, returns { alerts, acknowledge, silence, resolve }

Run all tests. Build must pass.

---

Commit:
```
git add .
git commit -m "feat(dashboard): complete Sprint 4 - full platform operational"
git push origin develop
```
