package apm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Span represents a single APM span.
type Span struct {
	SpanID     string            `json:"span_id"`
	TraceID    string            `json:"trace_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Operation  string            `json:"operation"`
	StartTime  string            `json:"start_time,omitempty"`
	DurationMs float64           `json:"duration_ms,omitempty"`
	Status     string            `json:"status,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// Payload is the APM batch envelope sent to hub ingest.
type Payload struct {
	SchemaVersion string `json:"schema_version"`
	AgentID       string `json:"agent_id"`
	Timestamp     string `json:"timestamp"`
	PayloadType   string `json:"payload_type"`
	Service       string `json:"service"`
	Environment   string `json:"environment"`
	Spans         []Span `json:"spans"`
}

// Buffer is the persistence dependency used when hub calls fail.
type Buffer interface {
	Enqueue(payload []byte) error
	Flush(ctx context.Context, send func(context.Context, []byte) error) error
}

// Config contains emitter runtime settings.
type Config struct {
	CollectorURL string
	AgentID      string
	AgentKey     string
	Service      string
	Environment  string
	FlushEvery   time.Duration
	MaxBatch     int
	TLSVerify    bool
}

// Emitter batches APM spans and ships them to the hub.
type Emitter struct {
	cfg    Config
	in     <-chan Span
	buffer Buffer
	client *http.Client
}

// New creates a span emitter with defaulted batching knobs.
func New(cfg Config, in <-chan Span, buffer Buffer) *Emitter {
	if cfg.FlushEvery <= 0 {
		cfg.FlushEvery = 5 * time.Second
	}
	if cfg.MaxBatch <= 0 {
		cfg.MaxBatch = 100
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS13}
	if !cfg.TLSVerify {
		//nolint:gosec // explicit runtime toggle
		tlsCfg.InsecureSkipVerify = true
	}

	return &Emitter{
		cfg:    cfg,
		in:     in,
		buffer: buffer,
		client: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: tlsCfg}},
	}
}

// Start runs the emitter loop until context cancellation.
func (e *Emitter) Start(ctx context.Context) error {
	ticker := time.NewTicker(e.cfg.FlushEvery)
	defer ticker.Stop()

	batch := make([]Span, 0, e.cfg.MaxBatch)
	flush := func(spans []Span) error {
		if len(spans) == 0 {
			return nil
		}
		payload := Payload{
			SchemaVersion: "1.0",
			AgentID:       e.cfg.AgentID,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			PayloadType:   "apm",
			Service:       e.cfg.Service,
			Environment:   e.cfg.Environment,
			Spans:         spans,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := e.sendRaw(ctx, raw); err != nil {
			return e.buffer.Enqueue(raw)
		}
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			_ = flush(batch)
			return nil
		case sp := <-e.in:
			batch = append(batch, sp)
			if len(batch) >= e.cfg.MaxBatch {
				if err := flush(batch); err != nil {
					return err
				}
				batch = batch[:0]
			}
		case <-ticker.C:
			if err := flush(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
}

func (e *Emitter) sendRaw(ctx context.Context, payload []byte) error {
	url := fmt.Sprintf("%s/ingest/apm", e.cfg.CollectorURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", e.cfg.AgentID)
	req.Header.Set("X-Agent-Key", e.cfg.AgentKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("apm emitter: status %d", resp.StatusCode)
	}
	return nil
}
