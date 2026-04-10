package scrubber_test

import (
	"testing"

	"github.com/idiey/uniwatch-sidecar/internal/scrubber"
)

func TestScrub(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "authorization bearer token",
			input: `GET /api/v1/users HTTP/1.1\nAuthorization: Bearer eyJhbGciOiJSUzI1NiJ9.secret.sig`,
			want:  `GET /api/v1/users HTTP/1.1\nAuthorization: Bearer [REDACTED]`,
		},
		{
			name:  "authorization basic token",
			input: `POST /login HTTP/1.1\nAuthorization: Basic dXNlcjpwYXNz`,
			want:  `POST /login HTTP/1.1\nAuthorization: Basic [REDACTED]`,
		},
		{
			name:  "credit card number with dashes",
			input: `User paid with card 4111-1111-1111-1111 today`,
			want:  `User paid with card [CARD-REDACTED] today`,
		},
		{
			name:  "credit card number plain 16 digits",
			input: `card=4111111111111111 charged`,
			want:  `card=[CARD-REDACTED] charged`,
		},
		{
			name:  "credit card number with spaces",
			input: `card 4111 1111 1111 1111 charged`,
			want:  `card [CARD-REDACTED] charged`,
		},
		{
			name:  "api_key in query string",
			input: `GET /data?api_key=supersecretkey123&format=json`,
			want:  `GET /data?api_key=[REDACTED]&format=json`,
		},
		{
			name:  "jwt token in log line",
			input: `token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c`,
			want:  `token: [JWT-REDACTED]`,
		},
		{
			name:  "password field in JSON",
			input: `{"username":"alice","password":"s3cr3tP@ss!"}`,
			want:  `{"username":"alice","password":"[REDACTED]"}`,
		},
		{
			name:  "secret field in JSON",
			input: `{"client_id":"abc","secret":"my-api-secret-value"}`,
			want:  `{"client_id":"abc","secret":"[REDACTED]"}`,
		},
		{
			name:  "private_key field in JSON",
			input: `{"private_key":"-----BEGIN RSA PRIVATE KEY-----"}`,
			want:  `{"private_key":"[REDACTED]"}`,
		},
		{
			name:  "no sensitive data unchanged",
			input: `GET /health HTTP/1.1 200 OK`,
			want:  `GET /health HTTP/1.1 200 OK`,
		},
		{
			name:  "multiple rules in one line",
			input: `Authorization: Bearer tok.en.abc {"password":"hunter2"} api_key=xyz123`,
			want:  `Authorization: Bearer [REDACTED] {"password":"[REDACTED]"} api_key=[REDACTED]`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrubber.Scrub(tc.input)
			if got != tc.want {
				t.Errorf("Scrub(%q)\n got:  %q\n want: %q", tc.input, got, tc.want)
			}
		})
	}
}
