package handler

import (
	"database/sql"
	"net/http"
	"time"
)

type FleetAgent struct {
	AgentID      string `json:"agent_id"`
	Status       string `json:"status"`
	LastSeenAt   string `json:"last_seen_at"`
	UptimeSecond int64  `json:"uptime_seconds"`
}

// Fleet returns latest heartbeat snapshot by agent from Postgres.
func Fleet(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeJSON(w, http.StatusOK, []FleetAgent{})
			return
		}

		const q = `
SELECT h.agent_id, h.status, h.received_at, COALESCE(h.uptime_seconds, 0)
FROM heartbeats h
JOIN (
  SELECT agent_id, MAX(received_at) AS max_received_at
  FROM heartbeats
  GROUP BY agent_id
) latest
ON latest.agent_id = h.agent_id AND latest.max_received_at = h.received_at
ORDER BY h.received_at DESC;
`
		rows, err := db.QueryContext(r.Context(), q)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query fleet failed"})
			return
		}
		defer rows.Close()

		out := make([]FleetAgent, 0)
		for rows.Next() {
			var item FleetAgent
			var lastSeen time.Time
			if err := rows.Scan(&item.AgentID, &item.Status, &lastSeen, &item.UptimeSecond); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "scan fleet row failed"})
				return
			}
			item.LastSeenAt = lastSeen.UTC().Format(time.RFC3339)
			out = append(out, item)
		}
		if err := rows.Err(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read fleet rows failed"})
			return
		}

		writeJSON(w, http.StatusOK, out)
	}
}
