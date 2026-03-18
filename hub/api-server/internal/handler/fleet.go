package handler

import "net/http"

type FleetEntry struct {
	AgentID      string `json:"agent_id"`
	Hostname     string `json:"hostname"`
	Status       string `json:"status"`
	LastSeen     string `json:"last_seen"`
	AgentVersion string `json:"agent_version"`
}

// Fleet handles GET /api/fleet. Sprint 1: mock data. Sprint 3: wires to DB.
func Fleet(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": []FleetEntry{
			{AgentID: "agent-001", Hostname: "prod-web-01", Status: "ok",
				LastSeen: "2026-03-18T00:00:00Z", AgentVersion: "1.0.0"},
		},
	})
}
