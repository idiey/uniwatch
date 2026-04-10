package logforwarder

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/idiey/uniwatch-sidecar/internal/scrubber"
	"github.com/idiey/uniwatch-sidecar/internal/traceid"
)

// LogEntry is one forwarded log line.
type LogEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	TraceID   string         `json:"trace_id"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// LogPayload is the batch envelope sent to hub ingest.
type LogPayload struct {
	SchemaVersion string     `json:"schema_version"`
	AgentID       string     `json:"agent_id"`
	Timestamp     string     `json:"timestamp"`
	PayloadType   string     `json:"payload_type"`
	Service       string     `json:"service"`
	Environment   string     `json:"environment"`
	Entries       []LogEntry `json:"entries"`
}

// Buffer is the persistence dependency used for failed sends.
type Buffer interface {
	Enqueue(payload []byte) error
	Flush(ctx context.Context, send func(context.Context, []byte) error) error
}

// Config contains runtime settings.
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

// Forwarder batches log entries and posts them to hub ingest.
type Forwarder struct {
	cfg    Config
	in     <-chan string
	buffer Buffer
	client *http.Client
}

// New creates a log forwarder with sane defaults.
func New(cfg Config, in <-chan string, buffer Buffer) *Forwarder {
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
	return &Forwarder{
		cfg:    cfg,
		in:     in,
		buffer: buffer,
		client: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: tlsCfg}},
	}
}

// Start runs the forwarder loop until context cancellation.
func (f *Forwarder) Start(ctx context.Context) error {
	ticker := time.NewTicker(f.cfg.FlushEvery)
	defer ticker.Stop()

	batch := make([]LogEntry, 0, f.cfg.MaxBatch)
	flush := func(entries []LogEntry) error {
		if len(entries) == 0 {
			return nil
		}
		payload := LogPayload{
			SchemaVersion: "1.0",
			AgentID:       f.cfg.AgentID,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			PayloadType:   "logs",
			Service:       f.cfg.Service,
			Environment:   f.cfg.Environment,
			Entries:       entries,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := f.sendRaw(ctx, raw); err != nil {
			return f.buffer.Enqueue(raw)
		}
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			_ = flush(batch)
			return nil
		case line := <-f.in:
			batch = append(batch, toEntry(line))
			if len(batch) >= f.cfg.MaxBatch {
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

func toEntry(line string) LogEntry {
	scrubbed := scrubber.Scrub(line)

	var raw map[string]any
	_ = json.Unmarshal([]byte(scrubbed), &raw)

	level, _ := raw["level"].(string)
	message, _ := raw["message"].(string)
	if message == "" {
		message = scrubbed
	}

	return LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		TraceID:   traceid.Extract(scrubbed),
		Fields:    raw,
	}
}

func (f *Forwarder) sendRaw(ctx context.Context, payload []byte) error {
	url := fmt.Sprintf("%s/ingest/logs", f.cfg.CollectorURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", f.cfg.AgentID)
	req.Header.Set("X-Agent-Key", f.cfg.AgentKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("logforwarder: status %d", resp.StatusCode)
	}
	return nil
}
