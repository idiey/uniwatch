package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/idiey/uniwatch-agent/internal/buffer"
	"github.com/idiey/uniwatch-agent/internal/collector"
	"github.com/idiey/uniwatch-agent/internal/config"
	"github.com/idiey/uniwatch-agent/internal/heartbeat"
	"github.com/idiey/uniwatch-agent/internal/pusher"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	configPath := flag.String("config", "agent.yaml", "Path to agent YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("agent: load config failed")
	}

	setupLogger(cfg.Logging.File)

	bufDir := cfg.BufferDir
	if bufDir == "" {
		bufDir = filepath.Join(".", "buffer")
	}

	buf, err := buffer.New(bufDir, cfg.BufferMaxMB)
	if err != nil {
		log.Fatal().Err(err).Msg("agent: buffer init failed")
	}

	metricsOut := make(chan collector.MetricsPayload, 128)
	c := collector.New(cfg, log.Logger)
	p := pusher.New(cfg, metricsOut, buf)
	hb := heartbeat.New(cfg, time.Now().UTC())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 3)
	go func() { errCh <- c.Start(ctx, metricsOut) }()
	go func() { errCh <- p.Start(ctx) }()
	go func() { errCh <- hb.Start(ctx) }()

	select {
	case <-ctx.Done():
	case err = <-errCh:
		if err != nil {
			log.Error().Err(err).Msg("agent: worker exited with error")
		}
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if flushErr := buf.Flush(shutdownCtx, p.SendBuffered); flushErr != nil && flushErr != context.Canceled {
		log.Warn().Err(flushErr).Msg("agent: buffer flush failed during shutdown")
	}
}

func setupLogger(logFile string) {
	if logFile == "" {
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		return
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal().Err(err).Str("file", logFile).Msg("agent: open log file failed")
	}
	log.Logger = zerolog.New(f).With().Timestamp().Logger()
}
