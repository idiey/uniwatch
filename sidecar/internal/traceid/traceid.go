// Package traceid extracts W3C traceparent trace IDs from log lines,
// or generates a new UUID v4 when none is present.
package traceid

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// traceparentRe matches the W3C traceparent header format:
// traceparent: 00-<trace_id(32 hex)>-<span_id(16 hex)>-<flags(2 hex)>
var traceparentRe = regexp.MustCompile(
	`traceparent:\s*00-([0-9a-fA-F]{32})-[0-9a-fA-F]{16}-[0-9a-fA-F]{2}`,
)

// Extract returns the 32-character hex trace ID found in line via the
// W3C traceparent header.  If no valid traceparent is present, it
// returns a freshly generated UUID v4 with dashes stripped (32 chars).
func Extract(line string) string {
	if m := traceparentRe.FindStringSubmatch(line); len(m) == 2 {
		return m[1]
	}
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
