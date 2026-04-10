// Package buffer provides a WAL-style disk-persisted queue for agent payloads.
// Entries are stored as newline-delimited JSON in a single queue.wal file.
package buffer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
)

const (
	walFileName = "queue.wal"
	flushBatch  = 50
)

// Buffer is a WAL-backed queue that persists JSON payloads to disk.
type Buffer struct {
	dir      string
	maxBytes int64
	mu       sync.Mutex
}

// New creates a new Buffer rooted at dir. maxMB is the maximum WAL file size in
// mebibytes before the oldest 10% of entries are dropped. The directory is
// created if it does not already exist, and any existing WAL file is loaded
// automatically.
func New(dir string, maxMB int) (*Buffer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("buffer: create dir %q: %w", dir, err)
	}

	b := &Buffer{
		dir:      dir,
		maxBytes: int64(maxMB) * 1024 * 1024,
	}

	// Log entry count if an existing WAL is present.
	if _, err := os.Stat(b.walPath()); err == nil {
		n, err := b.count()
		if err != nil {
			return nil, fmt.Errorf("buffer: read existing WAL: %w", err)
		}
		log.Info().Str("dir", dir).Int("entries", n).Msg("buffer: loaded existing WAL")
	}

	return b, nil
}

// walPath returns the absolute path to the WAL file.
func (b *Buffer) walPath() string {
	return filepath.Join(b.dir, walFileName)
}

// count returns the number of non-empty lines in the WAL file.
// The caller must hold b.mu or be in a single-goroutine context.
func (b *Buffer) count() (int, error) {
	lines, err := b.readLines()
	if err != nil {
		return 0, err
	}
	return len(lines), nil
}

// readLines returns all non-empty lines from the WAL file.
// Returns nil, nil when the file does not exist.
// The caller must hold b.mu or be in a single-goroutine context.
func (b *Buffer) readLines() ([][]byte, error) {
	f, err := os.Open(b.walPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("buffer: open WAL: %w", err)
	}

	var lines [][]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) > 0 {
			cp := make([]byte, len(line))
			copy(cp, line)
			lines = append(lines, cp)
		}
	}
	scanErr := sc.Err()
	if closeErr := f.Close(); closeErr != nil && scanErr == nil {
		scanErr = fmt.Errorf("buffer: close WAL: %w", closeErr)
	}
	return lines, scanErr
}

// writeLines atomically rewrites the WAL file with the given lines.
// An empty/nil slice removes the WAL file entirely.
// The caller must hold b.mu.
func (b *Buffer) writeLines(lines [][]byte) error {
	if len(lines) == 0 {
		if err := os.Remove(b.walPath()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("buffer: remove WAL: %w", err)
		}
		return nil
	}

	tmpPath := b.walPath() + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("buffer: open tmp WAL: %w", err)
	}

	w := bufio.NewWriter(f)
	for _, line := range lines {
		if _, err := w.Write(line); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("buffer: write line: %w", err)
		}
		if err := w.WriteByte('\n'); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("buffer: write newline: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("buffer: flush tmp WAL: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("buffer: sync tmp WAL: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("buffer: close tmp WAL: %w", err)
	}
	// On Windows os.Rename over an existing file can fail; remove first.
	if err := os.Remove(b.walPath()); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("buffer: remove old WAL before rename: %w", err)
	}
	if err := os.Rename(tmpPath, b.walPath()); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("buffer: rename tmp WAL: %w", err)
	}
	return nil
}

// Enqueue appends a JSON payload as a new line in the WAL file.
// If appending the payload would cause the file to exceed maxBytes, the oldest
// 10% of entries are dropped first and a WARNING is logged via zerolog.
func (b *Buffer) Enqueue(payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.maxBytes > 0 {
		info, err := os.Stat(b.walPath())
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("buffer: stat WAL: %w", err)
		}

		currentSize := int64(0)
		if err == nil {
			currentSize = info.Size()
		}

		if currentSize+int64(len(payload))+1 > b.maxBytes {
			lines, err := b.readLines()
			if err != nil {
				return fmt.Errorf("buffer: read lines for trim: %w", err)
			}

			dropCount := len(lines) / 10
			if dropCount < 1 {
				dropCount = 1
			}

			log.Warn().
				Int("dropped", dropCount).
				Int("remaining", len(lines)-dropCount).
				Str("dir", b.dir).
				Msg("buffer: WAL near capacity — dropped oldest entries")

			if err := b.writeLines(lines[dropCount:]); err != nil {
				return fmt.Errorf("buffer: rewrite after trim: %w", err)
			}
		}
	}

	// Append new entry.
	f, err := os.OpenFile(b.walPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("buffer: open WAL for append: %w", err)
	}
	line := append(bytes.TrimSpace(payload), '\n')
	_, writeErr := f.Write(line)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("buffer: write payload: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("buffer: close WAL after enqueue: %w", closeErr)
	}
	return nil
}

// Dequeue reads and removes the oldest entry from the WAL. Returns the entry
// bytes and true if an entry was available, or nil and false when the queue is
// empty.
func (b *Buffer) Dequeue() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lines, err := b.readLines()
	if err != nil {
		log.Error().Err(err).Msg("buffer: dequeue read WAL")
		return nil, false
	}
	if len(lines) == 0 {
		return nil, false
	}

	first := lines[0]
	if err := b.writeLines(lines[1:]); err != nil {
		log.Error().Err(err).Msg("buffer: dequeue rewrite WAL")
		return nil, false
	}
	return first, true
}

// Len returns the number of entries currently in the queue.
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	n, err := b.count()
	if err != nil {
		log.Error().Err(err).Msg("buffer: len count WAL")
		return 0
	}
	return n
}

// Flush drains the queue oldest-first in batches of 50, calling send for each
// entry. It stops early on context cancellation or if send returns an error,
// leaving remaining entries in the queue. Returns the first error encountered.
func (b *Buffer) Flush(ctx context.Context, send func(context.Context, []byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b.mu.Lock()
		lines, err := b.readLines()
		if err != nil {
			b.mu.Unlock()
			return fmt.Errorf("buffer: flush read WAL: %w", err)
		}
		if len(lines) == 0 {
			b.mu.Unlock()
			return nil
		}

		batchSize := flushBatch
		if batchSize > len(lines) {
			batchSize = len(lines)
		}
		batch := lines[:batchSize]
		remaining := lines[batchSize:]
		b.mu.Unlock()

		sent := 0
		var sendErr error
		for _, entry := range batch {
			select {
			case <-ctx.Done():
				sendErr = ctx.Err()
			default:
			}
			if sendErr != nil {
				break
			}
			if err := send(ctx, entry); err != nil {
				sendErr = err
				break
			}
			sent++
		}

		// Persist: keep the unsent portion of the batch plus all remaining.
		b.mu.Lock()
		unsent := make([][]byte, 0, len(batch)-sent+len(remaining))
		unsent = append(unsent, batch[sent:]...)
		unsent = append(unsent, remaining...)
		if writeErr := b.writeLines(unsent); writeErr != nil {
			b.mu.Unlock()
			return fmt.Errorf("buffer: flush rewrite WAL: %w", writeErr)
		}
		b.mu.Unlock()

		if sendErr != nil {
			return sendErr
		}

		// All done if this was the last (partial) batch.
		if len(lines) <= flushBatch {
			return nil
		}
	}
}
