// Package scrubber redacts PII and sensitive values from log lines.
package scrubber

import "regexp"

// rule pairs a compiled regex with its replacement string.
type rule struct {
	re          *regexp.Regexp
	replacement string
}

var rules = []rule{
	// Rule 1: Authorization Bearer header
	{
		re:          regexp.MustCompile(`(?i)(Authorization:\s*Bearer\s+)\S+`),
		replacement: `$1[REDACTED]`,
	},
	// Rule 2: Authorization Basic header
	{
		re:          regexp.MustCompile(`(?i)(Authorization:\s*Basic\s+)\S+`),
		replacement: `$1[REDACTED]`,
	},
	// Rule 3: Credit card numbers (16 digits, optionally space/dash separated)
	{
		re:          regexp.MustCompile(`\b(\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4})\b`),
		replacement: `[CARD-REDACTED]`,
	},
	// Rule 4: api_key in query strings
	{
		re:          regexp.MustCompile(`(api_key=)[^&\s]+`),
		replacement: `$1[REDACTED]`,
	},
	// Rule 5: JWT tokens (three base64url segments separated by dots)
	{
		re:          regexp.MustCompile(`\b[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\b`),
		replacement: `[JWT-REDACTED]`,
	},
	// Rule 6: password field in JSON
	{
		re:          regexp.MustCompile(`("password"\s*:\s*")[^"]*"`),
		replacement: `$1[REDACTED]"`,
	},
	// Rule 7: secret / private_key fields in JSON
	{
		re:          regexp.MustCompile(`("(?:secret|private_key)"\s*:\s*")[^"]*"`),
		replacement: `$1[REDACTED]"`,
	},
}

// Scrub applies all PII-scrubbing rules to line and returns the sanitised result.
func Scrub(line string) string {
	for _, r := range rules {
		line = r.re.ReplaceAllString(line, r.replacement)
	}
	return line
}
