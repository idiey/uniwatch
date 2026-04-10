package logforwarder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestForwarder_BatchesAndPosts(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest/logs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		posts.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	lines := make(chan string, 4)
	f := New(Config{
		CollectorURL: srv.URL,
		AgentID:      "agent-1",
		AgentKey:     "key-1",
		Service:      "svc",
		Environment:  "prod",
		FlushEvery:   30 * time.Millisecond,
		MaxBatch:     2,
	}, lines, &testBuffer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = f.Start(ctx) }()

	lines <- `{"level":"info","message":"a"}`
	lines <- `{"level":"info","message":"b"}`

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if posts.Load() >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected at least one log flush, got %d", posts.Load())
}

func TestForwarder_ScrubsAndExtractsTraceID(t *testing.T) {
	var captured LogPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	lines := make(chan string, 2)
	f := New(Config{
		CollectorURL: srv.URL,
		AgentID:      "agent-1",
		AgentKey:     "key-1",
		Service:      "svc",
		Environment:  "prod",
		FlushEvery:   20 * time.Millisecond,
		MaxBatch:     1,
	}, lines, &testBuffer{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = f.Start(ctx) }()

	lines <- `{"level":"info","message":"Authorization: Bearer secret-token traceparent: 00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"}`

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(captured.Entries) == 1 {
			msg := captured.Entries[0].Message
			if strings.Contains(msg, "secret-token") {
				t.Fatalf("expected scrubbed token, got %q", msg)
			}
			if captured.Entries[0].TraceID != "0123456789abcdef0123456789abcdef" {
				t.Fatalf("unexpected trace id: %s", captured.Entries[0].TraceID)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected one captured entry")
}

func TestForwarder_FailureBuffersPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	lines := make(chan string, 1)
	buf := &testBuffer{}
	f := New(Config{
		CollectorURL: srv.URL,
		AgentID:      "agent-1",
		AgentKey:     "key-1",
		Service:      "svc",
		Environment:  "prod",
		FlushEvery:   20 * time.Millisecond,
		MaxBatch:     1,
	}, lines, buf)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = f.Start(ctx) }()

	lines <- `{"level":"error","message":"boom"}`

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&buf.enqueued) >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected failed payload buffered, got %d", atomic.LoadInt32(&buf.enqueued))
}
