// Package tailer reads new lines from a log file as they are appended,
// with log rotation detection and byte-offset persistence.
package tailer

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/rs/zerolog"
)

const (
	defaultPollInterval  = 500 * time.Millisecond
	defaultRotationCheck = 5 * time.Second
)

// state is the on-disk JSON schema for the persisted byte offset.
type state struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
}

// Tailer reads new lines from a log file as they are appended.
type Tailer struct {
	path          string
	stateFile     string
	lines         chan<- string
	log           zerolog.Logger
	pollInterval  time.Duration
	rotationCheck time.Duration
}

// New creates a Tailer for the given file.
// stateFile is where byte offset is persisted (e.g., /var/lib/uniwatch/tailer.state).
// lines is the channel where new log lines are sent.
func New(path, stateFile string, lines chan<- string) *Tailer {
	return &Tailer{
		path:          path,
		stateFile:     stateFile,
		lines:         lines,
		log:           zerolog.New(os.Stderr).With().Timestamp().Str("component", "tailer").Str("file", path).Logger(),
		pollInterval:  defaultPollInterval,
		rotationCheck: defaultRotationCheck,
	}
}

// loadOffset reads the saved byte offset from stateFile.
// Returns 0 if the file does not exist or cannot be parsed.
func (t *Tailer) loadOffset() int64 {
	data, err := os.ReadFile(t.stateFile)
	if err != nil {
		// File may not exist yet — that is fine.
		return 0
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		t.log.Warn().Err(err).Msg("failed to parse state file; starting from offset 0")
		return 0
	}

	if s.Path != t.path {
		t.log.Warn().
			Str("state_path", s.Path).
			Msg("state file is for a different path; starting from offset 0")
		return 0
	}

	return s.Offset
}

// saveOffset persists the current byte offset to stateFile.
func (t *Tailer) saveOffset(offset int64) error {
	s := state{Path: t.path, Offset: offset}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(t.stateFile, data, 0o600)
}

// Start tails the file, sending new lines to the lines channel.
// Resumes from saved byte offset if stateFile exists.
// Detects log rotation: if the file is smaller than the last known offset
// (truncated/replaced), it reopens from the beginning.
// Checks for rotation every 5 seconds.
// Blocks until ctx is cancelled.
func (t *Tailer) Start(ctx context.Context) error {
	offset := t.loadOffset()

	f, err := t.openAt(offset)
	if err != nil {
		return err
	}
	// currentFile holds the currently open file handle so the defer always
	// closes whichever handle is active at exit.
	currentFile := f
	defer func() {
		if closeErr := currentFile.Close(); closeErr != nil {
			t.log.Error().Err(closeErr).Msg("error closing log file on exit")
		}
	}()

	scanner := bufio.NewScanner(f)

	lastRotationCheck := time.Now()

	for {
		// Check for context cancellation first.
		select {
		case <-ctx.Done():
			if saveErr := t.saveOffset(offset); saveErr != nil {
				t.log.Error().Err(saveErr).Msg("failed to save offset on shutdown")
			}
			return nil
		default:
		}

		// Rotation check at configured interval.
		if time.Since(lastRotationCheck) >= t.rotationCheck {
			lastRotationCheck = time.Now()
			rotated, reopenErr := t.checkRotation(offset)
			if reopenErr != nil {
				t.log.Error().Err(reopenErr).Msg("rotation check failed; continuing with current file")
			} else if rotated {
				t.log.Info().Msg("log rotation detected; reopening file from beginning")
				if closeErr := currentFile.Close(); closeErr != nil {
					t.log.Error().Err(closeErr).Msg("error closing rotated file")
				}
				offset = 0
				newFile, openErr := t.openAt(0)
				if openErr != nil {
					return openErr
				}
				currentFile = newFile
				scanner = bufio.NewScanner(newFile)
			}
		}

		// Drain all available lines.
		linesRead := 0
		for scanner.Scan() {
			line := scanner.Text()
			offset += int64(len(line)) + 1 // +1 for the newline byte consumed by Scanner
			linesRead++

			select {
			case t.lines <- line:
			default:
				preview := line
				if len(preview) > 80 {
					preview = preview[:80]
				}
				t.log.Warn().Str("line", preview).Msg("lines channel full; dropping line")
			}
		}

		if scanErr := scanner.Err(); scanErr != nil {
			t.log.Error().Err(scanErr).Msg("scanner error; will retry")
		}

		// Persist offset after each batch.
		if linesRead > 0 {
			if saveErr := t.saveOffset(offset); saveErr != nil {
				t.log.Error().Err(saveErr).Msg("failed to save offset")
			}
		}

		// Reopen at the current offset each poll cycle. bufio.Scanner buffers
		// reads internally, so after reaching EOF its internal state is
		// exhausted — creating a new scanner on the same handle does not
		// reliably pick up newly appended data. Reopening is the safest fix.
		if closeErr := currentFile.Close(); closeErr != nil {
			t.log.Error().Err(closeErr).Msg("error closing file before reopen")
		}
		newFile, openErr := t.openAt(offset)
		if openErr != nil {
			t.log.Error().Err(openErr).Msg("failed to reopen file; will retry after poll interval")
			// currentFile is closed; open a fresh handle for the defer.
			if f2, err2 := os.Open(t.path); err2 == nil {
				currentFile = f2
			}
		} else {
			currentFile = newFile
		}
		scanner = bufio.NewScanner(currentFile)

		// Wait before polling again, but respect context cancellation.
		select {
		case <-ctx.Done():
			if saveErr := t.saveOffset(offset); saveErr != nil {
				t.log.Error().Err(saveErr).Msg("failed to save offset on shutdown")
			}
			return nil
		case <-time.After(t.pollInterval):
		}
	}
}

// openAt opens t.path and seeks to the given byte offset.
func (t *Tailer) openAt(offset int64) (*os.File, error) {
	f, err := os.Open(t.path)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			if closeErr := f.Close(); closeErr != nil {
				t.log.Error().Err(closeErr).Msg("error closing file after seek failure")
			}
			return nil, err
		}
	}
	return f, nil
}

// checkRotation stats the file and returns true if its current size is less
// than the known offset, indicating truncation or replacement.
func (t *Tailer) checkRotation(offset int64) (bool, error) {
	info, err := os.Stat(t.path)
	if err != nil {
		return false, err
	}
	return info.Size() < offset, nil
}
