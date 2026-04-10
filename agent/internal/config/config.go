package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// HubConfig holds the hub connection settings.
type HubConfig struct {
	CollectorURL    string `yaml:"collector_url"`
	LogPipelineURL  string `yaml:"log_pipeline_url"`
	AgentKey        string `yaml:"agent_key"`
}

// LoggingConfig holds logging output settings.
type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// Config is the top-level agent configuration.
type Config struct {
	AgentID            string        `yaml:"agent_id"`
	Hub                HubConfig     `yaml:"hub"`
	CollectInterval    string        `yaml:"collect_interval"`
	HeartbeatInterval  string        `yaml:"heartbeat_interval"`
	LogFiles           []string      `yaml:"log_files"`
	MonitorServices    []string      `yaml:"monitor_services"`
	DiskAlertThreshold int           `yaml:"disk_alert_threshold"`
	BufferDir          string        `yaml:"buffer_dir"`
	BufferMaxMB        int           `yaml:"buffer_max_mb"`
	Logging            LoggingConfig `yaml:"logging"`
	TLSVerify          bool          `yaml:"tls_verify"`

	// Parsed durations — populated by Load.
	CollectIntervalDuration   time.Duration `yaml:"-"`
	HeartbeatIntervalDuration time.Duration `yaml:"-"`
}

// applyDefaults sets field defaults before YAML unmarshaling overwrites them.
func applyDefaults(c *Config) {
	c.CollectInterval = "60s"
	c.HeartbeatInterval = "30s"
	c.DiskAlertThreshold = 85
	c.BufferMaxMB = 100
	c.TLSVerify = true
}

// Load reads the YAML file at path, applies env-var overrides, validates the
// result and returns a ready-to-use Config.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	applyDefaults(cfg)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %q: %w", path, err)
	}

	if err = yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	// Override agent_key from environment — never log the value.
	if v := os.Getenv("UW_AGENT_KEY"); v != "" {
		log.Debug().Msg("config: UW_AGENT_KEY env var present — overriding hub.agent_key")
		cfg.Hub.AgentKey = v
	}

	if err = validate(cfg); err != nil {
		return nil, err
	}

	cfg.CollectIntervalDuration, err = time.ParseDuration(cfg.CollectInterval)
	if err != nil {
		return nil, fmt.Errorf("config: invalid collect_interval %q: %w", cfg.CollectInterval, err)
	}

	cfg.HeartbeatIntervalDuration, err = time.ParseDuration(cfg.HeartbeatInterval)
	if err != nil {
		return nil, fmt.Errorf("config: invalid heartbeat_interval %q: %w", cfg.HeartbeatInterval, err)
	}

	log.Info().
		Str("agent_id", cfg.AgentID).
		Str("collector_url", cfg.Hub.CollectorURL).
		Str("collect_interval", cfg.CollectInterval).
		Str("heartbeat_interval", cfg.HeartbeatInterval).
		Msg("config: loaded successfully")

	return cfg, nil
}

// validate checks required fields. It never logs the agent_key.
func validate(cfg *Config) error {
	if cfg.AgentID == "" {
		return errors.New("config: agent_id is required")
	}
	if cfg.Hub.CollectorURL == "" {
		return errors.New("config: hub.collector_url is required")
	}
	if cfg.Hub.AgentKey == "" {
		return errors.New("config: hub.agent_key is required (set UW_AGENT_KEY env var or provide in config file)")
	}
	return nil
}
