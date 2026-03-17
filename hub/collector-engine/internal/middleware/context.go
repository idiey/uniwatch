package middleware

import (
	"context"
	"net/http"
)

// contextKey is the typed key used for context values in this package.
type contextKey string

const agentIDKey contextKey = "agent_id"

// AgentIDFromContext retrieves the agent ID stored in context.
// Returns empty string if not present.
func AgentIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(agentIDKey).(string)
	return v
}

// InjectAgentID reads the X-Agent-ID header and stores it in the request context.
func InjectAgentID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID := r.Header.Get("X-Agent-ID")
		if agentID != "" {
			ctx := context.WithValue(r.Context(), agentIDKey, agentID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
