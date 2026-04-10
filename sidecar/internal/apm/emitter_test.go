package apm

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestEmitter_FlushesBatch(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest/apm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		posts.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	in := make(chan Span, 4)
	buf := &testBuffer{}
	e := New(Config{
		CollectorURL: srv.URL,
		AgentID:      "agent-1",
		AgentKey:     "key-1",
		Service:      "svc",
		Environment:  "prod",
		FlushEvery:   30 * time.Millisecond,
		MaxBatch:     2,
	}, in, buf)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = e.Start(ctx) }()
	in <- Span{SpanID: "s1", TraceID: "t1", Operation: "op"}
	in <- Span{SpanID: "s2", TraceID: "t2", Operation: "op"}

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if posts.Load() >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected at least one flush, got %d", posts.Load())
}

func TestEmitter_FailureBuffersPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	in := make(chan Span, 2)
	buf := &testBuffer{}
	e := New(Config{
		CollectorURL: srv.URL,
		AgentID:      "agent-1",
		AgentKey:     "key-1",
		Service:      "svc",
		Environment:  "prod",
		FlushEvery:   20 * time.Millisecond,
		MaxBatch:     1,
	}, in, buf)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = e.Start(ctx) }()

	in <- Span{SpanID: "s1", TraceID: "t1", Operation: "op"}

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&buf.enqueued) >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected buffered payload on failure, got %d", atomic.LoadInt32(&buf.enqueued))
}
