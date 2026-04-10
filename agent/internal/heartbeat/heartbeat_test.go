package heartbeat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/idiey/uniwatch-agent/internal/config"
	"context"
)

// newTestConfig returns a minimal Config wired to the given server URL.
func newTestConfig(serverURL string, interval time.Duration) *config.Config {
	return &config.Config{
		AgentID: "test-agent-001",
		Hub: config.HubConfig{
			CollectorURL: serverURL,
			AgentKey:     "test-key-secret",
		},
		HeartbeatIntervalDuration: interval,
		TLSVerify:                 true,
	}
}

// TestFirstHeartbeatWithin500ms verifies that Start sends the first heartbeat
// within 500 ms of being called.
func TestFirstHeartbeatWithin500ms(t *testing.T) {
	received := make(chan struct{}, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL, 10*time.Second) // long interval — only first matters
	worker := New(cfg, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()

	select {
	case <-received:
		// pass
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first heartbeat did not arrive within 500ms")
	}
}

// TestSecondHeartbeatWithinInterval verifies that a second heartbeat arrives
// within HeartbeatInterval + 100ms tolerance.
func TestSecondHeartbeatWithinInterval(t *testing.T) {
	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	interval := 200 * time.Millisecond
	cfg := newTestConfig(srv.URL, interval)
	worker := New(cfg, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	go func() { _ = worker.Start(ctx) }()

	deadline := interval + 100*time.Millisecond + 500*time.Millisecond // first + interval + tolerance
	for {
		if count.Load() >= 2 {
			break
		}
		if time.Since(start) > deadline {
			t.Fatalf("second heartbeat did not arrive within %v (got %d heartbeats)", deadline, count.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestServerError500WorkerContinues verifies that a 500 response from the hub
// causes a warning log but the Worker does not crash and keeps sending.
func TestServerError500WorkerContinues(t *testing.T) {
	var count atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n == 1 {
			// First request returns 500.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	interval := 100 * time.Millisecond
	cfg := newTestConfig(srv.URL, interval)
	worker := New(cfg, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()

	// Wait for at least 2 requests to confirm worker continued after the 500.
	deadline := time.After(2 * time.Second)
	for {
		if count.Load() >= 2 {
			return // success — worker kept going
		}
		select {
		case <-deadline:
			t.Fatalf("worker stopped after 500 response (only %d requests sent)", count.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestPayloadFields verifies that the heartbeat JSON body contains the
// required fields with correct values.
func TestPayloadFields(t *testing.T) {
	type hbPayload struct {
		SchemaVersion string `json:"schema_version"`
		AgentID       string `json:"agent_id"`
		PayloadType   string `json:"payload_type"`
		Timestamp     string `json:"timestamp"`
		Status        string `json:"status"`
		AgentVersion  string `json:"agent_version"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}

	received := make(chan hbPayload, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p hbPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		select {
		case received <- p:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL, 10*time.Second)
	startTime := time.Now()
	worker := New(cfg, startTime)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()

	var p hbPayload
	select {
	case p = <-received:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no heartbeat received within 500ms")
	}

	if p.SchemaVersion != "1.0" {
		t.Errorf("schema_version: got %q, want %q", p.SchemaVersion, "1.0")
	}
	if p.AgentID != "test-agent-001" {
		t.Errorf("agent_id: got %q, want %q", p.AgentID, "test-agent-001")
	}
	if p.PayloadType != "heartbeat" {
		t.Errorf("payload_type: got %q, want %q", p.PayloadType, "heartbeat")
	}
	if p.Status != "alive" {
		t.Errorf("status: got %q, want %q", p.Status, "alive")
	}
	if p.Timestamp == "" {
		t.Error("timestamp must not be empty")
	}
	// Uptime should be >= 0 and very small (< 5s) for a fresh worker.
	if p.UptimeSeconds < 0 || p.UptimeSeconds > 5 {
		t.Errorf("uptime_seconds: got %d, expected 0–5", p.UptimeSeconds)
	}
}

// TestHeadersSet verifies that X-Agent-ID and X-Agent-Key headers are present.
// The key value is deliberately not logged or asserted in output — only presence checked.
func TestHeadersSet(t *testing.T) {
	headersCh := make(chan http.Header, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case headersCh <- r.Header.Clone():
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL, 10*time.Second)
	worker := New(cfg, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()

	var headers http.Header
	select {
	case headers = <-headersCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no heartbeat received within 500ms")
	}

	if headers.Get("X-Agent-ID") != "test-agent-001" {
		t.Errorf("X-Agent-ID header: got %q, want %q", headers.Get("X-Agent-ID"), "test-agent-001")
	}
	if headers.Get("X-Agent-Key") == "" {
		t.Error("X-Agent-Key header must be present")
	}
}

// TestStartReturnsNilOnContextCancel verifies that Start exits cleanly when
// the context is cancelled and returns nil.
func TestStartReturnsNilOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL, 50*time.Millisecond)
	worker := New(cfg, time.Now())

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.Start(ctx)
	}()

	// Allow at least one heartbeat to fire, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned non-nil error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start did not return after context cancellation")
	}
}
