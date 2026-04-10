package tailer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newFastTailer creates a Tailer with reduced poll and rotation-check intervals
// so tests do not need to wait the production 5-second window.
func newFastTailer(path, stateFile string, lines chan<- string) *Tailer {
	t := New(path, stateFile, lines)
	t.pollInterval = 100 * time.Millisecond
	t.rotationCheck = 300 * time.Millisecond
	return t
}

// writeLines appends lines to a file, each terminated by a newline.
func writeLines(tb testing.TB, path string, lines []string) {
	tb.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		tb.Fatalf("writeLines open: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			tb.Errorf("writeLines close: %v", err)
		}
	}()
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			tb.Fatalf("writeLines write: %v", err)
		}
	}
}

// collectN reads exactly n lines from ch within the given timeout, failing the
// test if fewer than n lines arrive.
func collectN(tb testing.TB, ch <-chan string, n int, timeout time.Duration) []string {
	tb.Helper()
	collected := make([]string, 0, n)
	deadline := time.After(timeout)
	for len(collected) < n {
		select {
		case line := <-ch:
			collected = append(collected, line)
		case <-deadline:
			tb.Fatalf("timeout waiting for lines: got %d, want %d", len(collected), n)
		}
	}
	return collected
}

// TestNewLinesReceived verifies that lines already in the file plus lines
// appended while the tailer is running are all forwarded to the channel.
func TestNewLinesReceived(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")
	stateFile := filepath.Join(dir, "tailer.state")

	// Pre-write some lines before starting the tailer.
	writeLines(t, logFile, []string{"line1", "line2", "line3"})

	ch := make(chan string, 16)
	tail := newFastTailer(logFile, stateFile, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- tail.Start(ctx)
	}()

	// Collect the pre-written lines.
	got := collectN(t, ch, 3, 5*time.Second)
	want := []string{"line1", "line2", "line3"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("line[%d]: got %q, want %q", i, got[i], w)
		}
	}

	// Append more lines while the tailer is running.
	writeLines(t, logFile, []string{"line4", "line5"})

	got2 := collectN(t, ch, 2, 5*time.Second)
	want2 := []string{"line4", "line5"}
	for i, w := range want2 {
		if got2[i] != w {
			t.Errorf("live line[%d]: got %q, want %q", i, got2[i], w)
		}
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Start did not return after ctx cancel")
	}
}

// TestRotationDetection verifies that after a log rotation (file truncation),
// the tailer reopens from the beginning and delivers the new lines.
func TestRotationDetection(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")
	stateFile := filepath.Join(dir, "tailer.state")

	// Write initial lines so the tailer has a non-zero offset.
	writeLines(t, logFile, []string{"before-rotation-1", "before-rotation-2"})

	ch := make(chan string, 32)
	tail := newFastTailer(logFile, stateFile, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- tail.Start(ctx)
	}()

	// Drain the initial lines so the tailer records a non-zero offset.
	collectN(t, ch, 2, 5*time.Second)

	// Wait a moment for the offset to be saved, then simulate log rotation by
	// truncating the file (size becomes 0, which is less than the saved offset).
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(logFile, []byte{}, 0o600); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// Wait for the rotation check interval to fire (rotationCheck = 300ms).
	time.Sleep(500 * time.Millisecond)

	// Write new lines to the rotated (now empty) file.
	writeLines(t, logFile, []string{"after-rotation-1", "after-rotation-2"})

	got := collectN(t, ch, 2, 5*time.Second)
	want := []string{"after-rotation-1", "after-rotation-2"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("post-rotation line[%d]: got %q, want %q", i, got[i], w)
		}
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Start did not return after ctx cancel")
	}
}

// TestStateFilePersistence verifies that the tailer resumes from the saved
// offset on a restart, not re-reading previously seen lines.
func TestStateFilePersistence(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")
	stateFile := filepath.Join(dir, "tailer.state")

	writeLines(t, logFile, []string{"old-line-1", "old-line-2"})

	ch := make(chan string, 16)
	tail := newFastTailer(logFile, stateFile, ch)

	ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- tail.Start(ctx1)
	}()

	// Drain old lines so the tailer advances and saves the offset.
	collectN(t, ch, 2, 5*time.Second)

	// Give the tailer a moment to persist the offset before stopping it.
	time.Sleep(300 * time.Millisecond)
	cancel1()
	select {
	case err := <-errCh1:
		if err != nil {
			t.Errorf("first Start returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first Start did not return after ctx cancel")
	}

	// Append new lines after the first tailer has stopped.
	writeLines(t, logFile, []string{"new-line-1", "new-line-2"})

	// Start a second tailer — it should resume from the saved offset and skip
	// the old lines.
	ch2 := make(chan string, 16)
	tail2 := newFastTailer(logFile, stateFile, ch2)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- tail2.Start(ctx2)
	}()

	got := collectN(t, ch2, 2, 5*time.Second)
	want := []string{"new-line-1", "new-line-2"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("resumed line[%d]: got %q, want %q", i, got[i], w)
		}
	}

	// Verify old lines are NOT re-delivered.
	select {
	case extra := <-ch2:
		t.Errorf("unexpected extra line received: %q", extra)
	case <-time.After(400 * time.Millisecond):
		// Good — no old lines re-delivered.
	}

	cancel2()
}

// TestContextCancelReturnsNil verifies that Start returns nil when ctx is cancelled.
func TestContextCancelReturnsNil(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")
	stateFile := filepath.Join(dir, "tailer.state")

	writeLines(t, logFile, []string{"hello"})

	ch := make(chan string, 4)
	tail := newFastTailer(logFile, stateFile, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- tail.Start(ctx)
	}()

	// Let it run briefly, then cancel.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error on cancel, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Start did not return after ctx cancel")
	}
}

// TestMissingStateFileStartsFromZero verifies that a missing state file causes
// the tailer to start from the beginning of the file.
func TestMissingStateFileStartsFromZero(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")
	stateFile := filepath.Join(dir, "no-such-state.json") // intentionally absent

	writeLines(t, logFile, []string{"first", "second"})

	ch := make(chan string, 8)
	tail := newFastTailer(logFile, stateFile, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = tail.Start(ctx)
	}()

	got := collectN(t, ch, 2, 4*time.Second)
	if got[0] != "first" || got[1] != "second" {
		t.Errorf("unexpected lines: %v", got)
	}

	cancel()
}

// TestInvalidStateFileIgnored verifies that a corrupt state file is gracefully
// ignored and the tailer starts from offset 0.
func TestInvalidStateFileIgnored(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")
	stateFile := filepath.Join(dir, "tailer.state")

	// Write an invalid JSON state file.
	if err := os.WriteFile(stateFile, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write bad state: %v", err)
	}

	writeLines(t, logFile, []string{"alpha", "beta"})

	ch := make(chan string, 8)
	tail := newFastTailer(logFile, stateFile, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = tail.Start(ctx)
	}()

	got := collectN(t, ch, 2, 4*time.Second)
	if got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("unexpected lines: %v", got)
	}

	cancel()
}
