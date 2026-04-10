package collector

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/idiey/uniwatch-agent/internal/config"
	"github.com/rs/zerolog"
)

// newTestConfig returns a minimal Config suitable for unit tests.
func newTestConfig() *config.Config {
	return &config.Config{
		AgentID:                 "test-agent-001",
		CollectIntervalDuration: 100 * time.Millisecond,
		DiskAlertThreshold:      85,
		MonitorServices:         []string{},
	}
}

func newTestLogger(buf *bytes.Buffer) zerolog.Logger {
	return zerolog.New(buf)
}

// TestCollect_ValidPayload verifies that Collect returns a well-formed
// MetricsPayload with non-zero CPU cores and non-zero total memory.
func TestCollect_ValidPayload(t *testing.T) {
	cfg := newTestConfig()
	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	payload, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	if payload.SchemaVersion != "1.0" {
		t.Errorf("schema_version: got %q, want %q", payload.SchemaVersion, "1.0")
	}
	if payload.PayloadType != "metrics" {
		t.Errorf("payload_type: got %q, want %q", payload.PayloadType, "metrics")
	}
	if payload.AgentID != cfg.AgentID {
		t.Errorf("agent_id: got %q, want %q", payload.AgentID, cfg.AgentID)
	}
	if payload.Timestamp == "" {
		t.Error("timestamp must not be empty")
	}
	if payload.CPU.Cores <= 0 {
		t.Errorf("cpu.cores: got %d, want > 0", payload.CPU.Cores)
	}
	if payload.Memory.TotalBytes <= 0 {
		t.Errorf("memory.total_bytes: got %d, want > 0", payload.Memory.TotalBytes)
	}
}

// TestCollect_DiskInfoPresent verifies that at least one disk entry is returned
// with non-zero total bytes.
func TestCollect_DiskInfoPresent(t *testing.T) {
	cfg := newTestConfig()
	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	payload, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	if len(payload.Disk) == 0 {
		t.Error("disk: expected at least one entry")
		return
	}
	for _, d := range payload.Disk {
		if d.MountPoint == "" {
			t.Error("disk: mount_point must not be empty")
		}
		if d.TotalBytes <= 0 {
			t.Errorf("disk[%s]: total_bytes must be > 0, got %d", d.MountPoint, d.TotalBytes)
		}
	}
}

// TestCollect_NetworkSkipsLoopback verifies that loopback interfaces are excluded.
func TestCollect_NetworkSkipsLoopback(t *testing.T) {
	cfg := newTestConfig()
	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	payload, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	for _, n := range payload.Network {
		lower := strings.ToLower(n.Interface)
		if lower == "lo" || lower == "lo0" || strings.HasPrefix(lower, "loopback") {
			t.Errorf("network: loopback interface %q should be excluded", n.Interface)
		}
	}
}

// TestStart_SendsAtLeastTwoPayloads verifies that Start delivers ≥2 payloads
// within 2*interval + 100ms tolerance.
func TestStart_SendsAtLeastTwoPayloads(t *testing.T) {
	interval := 150 * time.Millisecond
	cfg := newTestConfig()
	cfg.CollectIntervalDuration = interval

	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	out := make(chan MetricsPayload, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := c.Start(ctx, out); err != nil {
			// err only returned on unexpected path; context cancel returns nil
		}
	}()

	deadline := 2*interval + 100*time.Millisecond
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	var received int
	for received < 2 {
		select {
		case <-out:
			received++
		case <-timer.C:
			t.Fatalf("Start: expected ≥2 payloads within %v, got %d", deadline, received)
		}
	}
}

// TestStart_ReturnsNilOnContextCancel verifies Start exits cleanly on cancel.
func TestStart_ReturnsNilOnContextCancel(t *testing.T) {
	cfg := newTestConfig()
	cfg.CollectIntervalDuration = 50 * time.Millisecond

	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	out := make(chan MetricsPayload, 10)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Start(ctx, out)
	}()

	// Allow first collection then cancel.
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned non-nil error on cancel: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start did not return after context cancellation")
	}
}

// TestDiskAlertThreshold_LogsWarning verifies that a zerolog WARNING is
// emitted when a disk mount exceeds the configured threshold.
// We use a threshold of 0 so that any mount with >0% usage triggers it.
func TestDiskAlertThreshold_LogsWarning(t *testing.T) {
	cfg := newTestConfig()
	cfg.DiskAlertThreshold = 0 // any non-zero usage triggers warning

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	_, err := collectDisk(context.Background(), cfg, log)
	if err != nil {
		t.Fatalf("collectDisk returned error: %v", err)
	}

	logOutput := buf.String()
	// With threshold=0, at least one mount should exceed it and emit a warning.
	// The log line must contain "exceeds alert threshold".
	if !strings.Contains(logOutput, "exceeds alert threshold") {
		t.Errorf("expected disk alert warning in log output, got: %q", logOutput)
	}
}

// TestWatchedProcesses_StatusField verifies that watched processes have a
// valid status field ("running" or "stopped") and that the list length matches
// MonitorServices.
func TestWatchedProcesses_StatusField(t *testing.T) {
	// Use a process that is virtually guaranteed to exist (the test binary itself)
	// and one that is guaranteed not to exist.
	watchList := []string{"nonexistent-proc-xyz-12345", "go"}

	info, err := collectProcesses(context.Background(), watchList)
	if err != nil {
		t.Fatalf("collectProcesses returned error: %v", err)
	}

	if len(info.Watched) != len(watchList) {
		t.Fatalf("watched length: got %d, want %d", len(info.Watched), len(watchList))
	}

	for _, wp := range info.Watched {
		if wp.Status != "running" && wp.Status != "stopped" {
			t.Errorf("process %q: invalid status %q, must be 'running' or 'stopped'", wp.Name, wp.Status)
		}
	}

	// The nonexistent process must be stopped.
	for _, wp := range info.Watched {
		if wp.Name == "nonexistent-proc-xyz-12345" && wp.Status != "stopped" {
			t.Errorf("nonexistent process should be 'stopped', got %q", wp.Status)
		}
	}
}

// TestWatchedProcesses_TotalRunningCount verifies TotalRunning matches the
// actual count of "running" entries.
func TestWatchedProcesses_TotalRunningCount(t *testing.T) {
	watchList := []string{"nonexistent-proc-xyz-12345", "go"}

	info, err := collectProcesses(context.Background(), watchList)
	if err != nil {
		t.Fatalf("collectProcesses returned error: %v", err)
	}

	var manualCount int
	for _, wp := range info.Watched {
		if wp.Status == "running" {
			manualCount++
		}
	}

	if info.TotalRunning != manualCount {
		t.Errorf("TotalRunning: got %d, want %d", info.TotalRunning, manualCount)
	}
}

// TestCollect_PayloadTimestampIsUTC verifies the timestamp is RFC3339 and ends with Z (UTC).
func TestCollect_PayloadTimestampIsUTC(t *testing.T) {
	cfg := newTestConfig()
	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	payload, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	if !strings.HasSuffix(payload.Timestamp, "Z") {
		t.Errorf("timestamp %q should end with Z (UTC)", payload.Timestamp)
	}
}

// TestCollect_EmptyWatchedList verifies Collect succeeds with no MonitorServices.
func TestCollect_EmptyWatchedList(t *testing.T) {
	cfg := newTestConfig()
	cfg.MonitorServices = []string{}
	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	payload, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	if len(payload.Processes.Watched) != 0 {
		t.Errorf("watched: expected 0, got %d", len(payload.Processes.Watched))
	}
	if payload.Processes.TotalRunning != 0 {
		t.Errorf("total_running: expected 0, got %d", payload.Processes.TotalRunning)
	}
}

// TestStart_ImmediateFirstPayload verifies that Start emits the first payload
// within 500ms (i.e. no initial wait before first collection).
func TestStart_ImmediateFirstPayload(t *testing.T) {
	cfg := newTestConfig()
	cfg.CollectIntervalDuration = 10 * time.Second // long — only care about first

	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	out := make(chan MetricsPayload, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = c.Start(ctx, out)
	}()

	select {
	case <-out:
		// pass — first payload arrived promptly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start: first payload not received within 500ms")
	}
}

// TestCollect_MemoryFieldsConsistent verifies basic memory field relationships.
func TestCollect_MemoryFieldsConsistent(t *testing.T) {
	cfg := newTestConfig()
	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	payload, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	m := payload.Memory
	if m.TotalBytes <= 0 {
		t.Errorf("memory.total_bytes must be > 0")
	}
	if m.UsedPct < 0 || m.UsedPct > 100 {
		t.Errorf("memory.used_pct out of range: %f", m.UsedPct)
	}
}

// TestStart_PayloadChannelReceiveCount uses an atomic counter to confirm
// at least 2 payloads are delivered without relying on a channel select loop.
func TestStart_PayloadChannelReceiveCount(t *testing.T) {
	interval := 100 * time.Millisecond
	cfg := newTestConfig()
	cfg.CollectIntervalDuration = interval

	var buf bytes.Buffer
	log := newTestLogger(&buf)
	c := New(cfg, log)

	out := make(chan MetricsPayload, 20)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = c.Start(ctx, out)
	}()

	// Wait for 2*interval + buffer.
	time.Sleep(2*interval + 150*time.Millisecond)
	cancel()

	var count int32
	for {
		select {
		case <-out:
			atomic.AddInt32(&count, 1)
		default:
			goto done
		}
	}
done:
	if count < 2 {
		t.Errorf("expected ≥2 payloads, got %d", count)
	}
}
