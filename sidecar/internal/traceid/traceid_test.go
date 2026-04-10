package traceid_test

import (
	"regexp"
	"testing"

	"github.com/idiey/uniwatch-sidecar/internal/traceid"
)

var hexRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestExtract_ValidTraceparent(t *testing.T) {
	t.Parallel()

	line := `2026-04-10T08:00:00Z INFO request received traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`
	got := traceid.Extract(line)

	want := "4bf92f3577b34da6a3ce929d0e0e4736"
	if got != want {
		t.Errorf("Extract(%q) = %q; want %q", line, got, want)
	}
}

func TestExtract_NoTraceparent_GeneratesUUID(t *testing.T) {
	t.Parallel()

	line := `2026-04-10T08:00:00Z INFO no trace header here`
	got := traceid.Extract(line)

	if len(got) != 32 {
		t.Errorf("Extract(%q) len = %d; want 32", line, len(got))
	}
	if !hexRe.MatchString(got) {
		t.Errorf("Extract(%q) = %q; want 32-char lowercase hex", line, got)
	}
}

func TestExtract_MalformedTraceparent_GeneratesUUID(t *testing.T) {
	t.Parallel()

	// missing flags segment — should not match
	line := `traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7`
	got := traceid.Extract(line)

	if len(got) != 32 {
		t.Errorf("Extract(%q) len = %d; want 32", line, len(got))
	}
}

func TestExtract_DifferentCallsProduceDistinctIDs(t *testing.T) {
	t.Parallel()

	line := `no traceparent here`
	id1 := traceid.Extract(line)
	id2 := traceid.Extract(line)

	if id1 == id2 {
		t.Errorf("expected distinct UUIDs on successive calls, got same: %q", id1)
	}
}
