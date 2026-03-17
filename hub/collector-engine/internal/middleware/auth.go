package middleware

import (
	"encoding/json"
	"net/http"
	"os"
)

// AgentAuth is a Chi middleware that validates the X-Agent-Key header
// against the AGENT_API_KEY environment variable.
func AgentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := os.Getenv("AGENT_API_KEY")
		provided := r.Header.Get("X-Agent-Key")

		if expected == "" || provided != expected {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			enc := json.NewEncoder(w)
			if err := enc.Encode(map[string]string{"error": "unauthorized"}); err != nil {
				return
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}
