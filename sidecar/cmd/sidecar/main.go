package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/idiey/uniwatch-sidecar/internal/apm"
	"github.com/idiey/uniwatch-sidecar/internal/logforwarder"
	"github.com/idiey/uniwatch-sidecar/internal/tailer"
	"github.com/rs/zerolog/log"
)

type nopBuffer struct{}

func (n nopBuffer) Enqueue(_ []byte) error { return nil }
func (n nopBuffer) Flush(_ context.Context, _ func(context.Context, []byte) error) error {
	return nil
}

func main() {
	collectorURL := os.Getenv("UW_COLLECTOR_URL")
	agentID := os.Getenv("UW_AGENT_ID")
	agentKey := os.Getenv("UW_AGENT_KEY")
	logFiles := splitCSV(os.Getenv("UW_LOG_FILES"))

	if collectorURL == "" || agentID == "" || agentKey == "" || len(logFiles) == 0 {
		log.Fatal().Msg("sidecar: set UW_COLLECTOR_URL, UW_AGENT_ID, UW_AGENT_KEY and UW_LOG_FILES")
	}

	service := getenvDefault("UW_SERVICE", "unknown-service")
	env := getenvDefault("UW_ENVIRONMENT", "prod")
	stateDir := getenvDefault("UW_STATE_DIR", ".state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		log.Fatal().Err(err).Str("dir", stateDir).Msg("sidecar: create state dir failed")
	}

	rawLines := make(chan string, 256)
	logLines := make(chan string, 256)
	spans := make(chan apm.Span, 256)

	forwarder := logforwarder.New(logforwarder.Config{
		CollectorURL: collectorURL,
		AgentID:      agentID,
		AgentKey:     agentKey,
		Service:      service,
		Environment:  env,
		FlushEvery:   5 * time.Second,
		MaxBatch:     100,
		TLSVerify:    true,
	}, logLines, nopBuffer{})

	emitter := apm.New(apm.Config{
		CollectorURL: collectorURL,
		AgentID:      agentID,
		AgentKey:     agentKey,
		Service:      service,
		Environment:  env,
		FlushEvery:   5 * time.Second,
		MaxBatch:     100,
		TLSVerify:    true,
	}, spans, nopBuffer{})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, len(logFiles)+3)
	go func() { errCh <- forwarder.Start(ctx) }()
	go func() { errCh <- emitter.Start(ctx) }()

	for _, path := range logFiles {
		p := path
		stateFile := filepath.Join(stateDir, strings.ReplaceAll(filepath.Base(p), ".", "_")+".state.json")
		t := tailer.New(p, stateFile, rawLines)
		go func() { errCh <- t.Start(ctx) }()
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case line := <-rawLines:
				logLines <- line
				if span, ok := parseSpan(line); ok {
					spans <- span
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			log.Error().Err(err).Msg("sidecar: worker exited with error")
		}
		stop()
	}
}

func parseSpan(line string) (apm.Span, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return apm.Span{}, false
	}

	spanID, _ := m["span_id"].(string)
	traceID, _ := m["trace_id"].(string)
	op, _ := m["operation"].(string)
	if spanID == "" || traceID == "" || op == "" {
		return apm.Span{}, false
	}

	span := apm.Span{
		SpanID:    spanID,
		TraceID:   traceID,
		Operation: op,
	}
	if v, ok := m["parent_id"].(string); ok {
		span.ParentID = v
	}
	if v, ok := m["status"].(string); ok {
		span.Status = v
	}
	if v, ok := m["start_time"].(string); ok {
		span.StartTime = v
	}
	if v, ok := m["duration_ms"].(float64); ok {
		span.DurationMs = v
	}
	return span, true
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
