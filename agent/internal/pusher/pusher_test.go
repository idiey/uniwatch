package pusher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/idiey/uniwatch-agent/internal/collector"
	"github.com/idiey/uniwatch-agent/internal/config"
)

type testBuffer struct {
	enqueued int32
}

func (b *testBuffer) Enqueue(_ []byte) error {
	atomic.AddInt32(&b.enqueued, 1)
	return nil
}

func (b *testBuffer) Flush(_ context.Context, _ func(context.Context, []byte) error) error {
	return nil
}

func testCfg(url string) *config.Config {
	return &config.Config{
		AgentID: "agent-001",
		Hub: config.HubConfig{
			CollectorURL: url,
			AgentKey:     "key-001",
		},
		TLSVerify: true,
	}
}

func samplePayload() collector.MetricsPayload {
	return collector.MetricsPayload{
		SchemaVersion: "1.0",
		AgentID:       "agent-001",
		PayloadType:   "metrics",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
}

func TestStart_DeliversMetrics(t *testing.T) {
	var got atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest/metrics" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Agent-ID") == "" || r.Header.Get("X-Agent-Key") == "" {
			t.Fatalf("missing auth headers")
		}
		got.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	in := make(chan collector.MetricsPayload, 1)
	buf := &testBuffer{}
	p := New(testCfg(srv.URL), in, buf)
	p.backoffs = []time.Duration{5 * time.Millisecond}
	p.rateLimitBackoff = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Start(ctx) }()

	in <- samplePayload()

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if got.Load() == 1 {
			cancel()
			_ = <-errCh
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected one delivered payload, got %d", got.Load())
}

func TestStart_UnauthorizedStops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	in := make(chan collector.MetricsPayload, 1)
	buf := &testBuffer{}
	p := New(testCfg(srv.URL), in, buf)
	p.backoffs = []time.Duration{5 * time.Millisecond}
	p.rateLimitBackoff = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Start(ctx) }()

	in <- samplePayload()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected unauthorized error, got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected pusher to stop on unauthorized")
	}
}

func TestStart_FailedDeliveryBuffersPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	in := make(chan collector.MetricsPayload, 1)
	buf := &testBuffer{}
	p := New(testCfg(srv.URL), in, buf)
	p.backoffs = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond}
	p.rateLimitBackoff = 5 * time.Millisecond
	p.maxAttempts = 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Start(ctx) }()

	in <- samplePayload()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&buf.enqueued) == 1 {
			cancel()
			_ = <-errCh
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected failed payload to be buffered, got %d", atomic.LoadInt32(&buf.enqueued))
}

func TestSendRaw_ValidJSONPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Fatalf("invalid json body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := New(testCfg(srv.URL), make(chan collector.MetricsPayload), &testBuffer{})
	raw, _ := json.Marshal(samplePayload())
	if err := p.sendRaw(context.Background(), raw); err != nil {
		t.Fatalf("sendRaw failed: %v", err)
	}
}
