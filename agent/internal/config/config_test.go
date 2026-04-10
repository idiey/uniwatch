package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTemp writes content to a temp YAML file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

const validYAML = `
agent_id: myimu-be-prod
hub:
  collector_url: http://hub:9090
  log_pipeline_url: http://hub:9091
  agent_key: secret-key
collect_interval: 60s
heartbeat_interval: 30s
disk_alert_threshold: 85
buffer_max_mb: 100
tls_verify: true
`

func TestLoad_ValidConfig(t *testing.T) {
	path := writeTemp(t, validYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"agent_id", cfg.AgentID, "myimu-be-prod"},
		{"hub.collector_url", cfg.Hub.CollectorURL, "http://hub:9090"},
		{"hub.log_pipeline_url", cfg.Hub.LogPipelineURL, "http://hub:9091"},
		{"hub.agent_key", cfg.Hub.AgentKey, "secret-key"},
		{"collect_interval", cfg.CollectInterval, "60s"},
		{"heartbeat_interval", cfg.HeartbeatInterval, "30s"},
		{"disk_alert_threshold", cfg.DiskAlertThreshold, 85},
		{"buffer_max_mb", cfg.BufferMaxMB, 100},
		{"tls_verify", cfg.TLSVerify, true},
		{"collect_interval_duration", cfg.CollectIntervalDuration, 60 * time.Second},
		{"heartbeat_interval_duration", cfg.HeartbeatIntervalDuration, 30 * time.Second},
	}

	for _, tc := range checks {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Minimal YAML — only required fields, no intervals or thresholds.
	minimal := `
agent_id: test-agent
hub:
  collector_url: http://hub:9090
  agent_key: secret
`
	path := writeTemp(t, minimal)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.CollectInterval != "60s" {
		t.Errorf("default collect_interval: got %q, want %q", cfg.CollectInterval, "60s")
	}
	if cfg.HeartbeatInterval != "30s" {
		t.Errorf("default heartbeat_interval: got %q, want %q", cfg.HeartbeatInterval, "30s")
	}
	if cfg.DiskAlertThreshold != 85 {
		t.Errorf("default disk_alert_threshold: got %d, want 85", cfg.DiskAlertThreshold)
	}
	if cfg.BufferMaxMB != 100 {
		t.Errorf("default buffer_max_mb: got %d, want 100", cfg.BufferMaxMB)
	}
	if !cfg.TLSVerify {
		t.Error("default tls_verify: got false, want true")
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "missing agent_id",
			yaml: `
hub:
  collector_url: http://hub:9090
  agent_key: secret
`,
			wantErr: "agent_id is required",
		},
		{
			name: "missing hub.collector_url",
			yaml: `
agent_id: test-agent
hub:
  agent_key: secret
`,
			wantErr: "hub.collector_url is required",
		},
		{
			name: "missing hub.agent_key",
			yaml: `
agent_id: test-agent
hub:
  collector_url: http://hub:9090
`,
			wantErr: "hub.agent_key is required",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, tc.yaml)
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsString(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoad_EnvVarOverridesAgentKey(t *testing.T) {
	yaml := `
agent_id: test-agent
hub:
  collector_url: http://hub:9090
  agent_key: original-key
`
	path := writeTemp(t, yaml)

	t.Setenv("UW_AGENT_KEY", "env-override-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hub.AgentKey != "env-override-key" {
		t.Errorf("agent_key: got %q, want %q", cfg.Hub.AgentKey, "env-override-key")
	}
}

func TestLoad_EnvVarProvidesAgentKeyWhenMissingInYAML(t *testing.T) {
	yaml := `
agent_id: test-agent
hub:
  collector_url: http://hub:9090
`
	path := writeTemp(t, yaml)

	t.Setenv("UW_AGENT_KEY", "env-only-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hub.AgentKey != "env-only-key" {
		t.Errorf("agent_key from env: got %q, want %q", cfg.Hub.AgentKey, "env-only-key")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "invalid collect_interval",
			yaml: `
agent_id: test-agent
hub:
  collector_url: http://hub:9090
  agent_key: secret
collect_interval: not-a-duration
`,
			wantErr: "collect_interval",
		},
		{
			name: "invalid heartbeat_interval",
			yaml: `
agent_id: test-agent
hub:
  collector_url: http://hub:9090
  agent_key: secret
heartbeat_interval: badval
`,
			wantErr: "heartbeat_interval",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, tc.yaml)
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsString(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/agent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, ":: not valid yaml ::")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml, got nil")
	}
}

// containsString is a simple substring check to avoid importing strings in test.
func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findSub(s, sub))
}

func findSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
