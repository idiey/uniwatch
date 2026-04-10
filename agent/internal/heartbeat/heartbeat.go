package heartbeat

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/idiey/uniwatch-agent/internal/config"
	"github.com/rs/zerolog/log"
)

// Version is the agent version string, set at build time via -ldflags.
var Version = "dev"

// payload is the JSON body sent to the hub on each heartbeat.
type payload struct {
	SchemaVersion string `json:"schema_version"`
	AgentID       string `json:"agent_id"`
	PayloadType   string `json:"payload_type"`
	Timestamp     string `json:"timestamp"`
	Status        string `json:"status"`
	AgentVersion  string `json:"agent_version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// Worker sends periodic heartbeats to the hub collector.
type Worker struct {
	cfg       *config.Config
	startTime time.Time
	client    *http.Client
}

// New creates a Worker. startTime is used to compute uptime_seconds.
func New(cfg *config.Config, startTime time.Time) *Worker {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if !cfg.TLSVerify {
		log.Warn().Msg("heartbeat: TLS verification is DISABLED — connections are not secure")
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // intentional, controlled by config
	}

	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	return &Worker{
		cfg:       cfg,
		startTime: startTime,
		client:    client,
	}
}

// Start sends the first heartbeat immediately (within 500ms), then every
// cfg.HeartbeatIntervalDuration. On send failure it logs a warning and
// continues. Blocks until ctx is cancelled, then returns nil.
func (w *Worker) Start(ctx context.Context) error {
	// Send the first heartbeat right away.
	w.send(ctx)

	ticker := time.NewTicker(w.cfg.HeartbeatIntervalDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.send(ctx)
		}
	}
}

// send builds and dispatches a single heartbeat POST. Errors are logged as
// warnings so the caller (Start) can continue running.
func (w *Worker) send(ctx context.Context) {
	now := time.Now().UTC()
	uptime := int64(now.Sub(w.startTime).Seconds())

	p := payload{
		SchemaVersion: "1.0",
		AgentID:       w.cfg.AgentID,
		PayloadType:   "heartbeat",
		Timestamp:     now.Format(time.RFC3339),
		Status:        "alive",
		AgentVersion:  Version,
		UptimeSeconds: uptime,
	}

	body, err := json.Marshal(p)
	if err != nil {
		log.Warn().Err(err).Msg("heartbeat: failed to marshal payload")
		return
	}

	url := fmt.Sprintf("%s/ingest/heartbeat", w.cfg.Hub.CollectorURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Warn().Err(err).Msg("heartbeat: failed to create request")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", w.cfg.AgentID)
	req.Header.Set("X-Agent-Key", w.cfg.Hub.AgentKey) // value never logged

	log.Debug().
		Str("agent_id", w.cfg.AgentID).
		Int64("uptime_seconds", uptime).
		Msg("heartbeat: sending heartbeat")

	resp, err := w.client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("heartbeat: send failed")
		return
	}

	if err = resp.Body.Close(); err != nil {
		log.Warn().Err(err).Msg("heartbeat: failed to close response body")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().
			Int("status_code", resp.StatusCode).
			Msg("heartbeat: hub returned non-2xx response")
		return
	}

	log.Debug().
		Str("agent_id", w.cfg.AgentID).
		Int("status_code", resp.StatusCode).
		Msg("heartbeat: delivered successfully")
}
