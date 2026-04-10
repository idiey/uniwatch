package pusher

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/idiey/uniwatch-agent/internal/collector"
	"github.com/idiey/uniwatch-agent/internal/config"
	"github.com/rs/zerolog/log"
)

var (
	// ErrUnauthorized indicates the hub rejected the API key and retries must stop.
	ErrUnauthorized = errors.New("pusher: unauthorized (401)")
	errRateLimited  = errors.New("pusher: rate limited (429)")
	errRetryable    = errors.New("pusher: retryable server error")
)

// Buffer is the disk queue dependency required by the pusher.
type Buffer interface {
	Enqueue(payload []byte) error
	Flush(ctx context.Context, send func(context.Context, []byte) error) error
}

// Pusher consumes metrics payloads and forwards them to the hub collector.
type Pusher struct {
	cfg    *config.Config
	in     <-chan collector.MetricsPayload
	buffer Buffer
	client *http.Client

	maxAttempts      int
	backoffs         []time.Duration
	rateLimitBackoff time.Duration
	replayEvery      time.Duration
}

// New creates a metrics pusher with sane production defaults.
func New(cfg *config.Config, in <-chan collector.MetricsPayload, buffer Buffer) *Pusher {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}
	if !cfg.TLSVerify {
		//nolint:gosec // explicit opt-out from config
		tlsCfg.InsecureSkipVerify = true
	}

	return &Pusher{
		cfg:              cfg,
		in:               in,
		buffer:           buffer,
		client:           &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: tlsCfg}},
		maxAttempts:      4,
		backoffs:         []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second},
		rateLimitBackoff: 60 * time.Second,
		replayEvery:      30 * time.Second,
	}
}

// Start reads payloads from the input channel and pushes them to the hub.
// Unauthorized responses stop the worker immediately.
func (p *Pusher) Start(ctx context.Context) error {
	if err := p.replayOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		if errors.Is(err, ErrUnauthorized) {
			return err
		}
		log.Warn().Err(err).Msg("pusher: initial replay failed")
	}

	ticker := time.NewTicker(p.replayEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := p.replayOnce(ctx); err != nil {
				if errors.Is(err, ErrUnauthorized) {
					return err
				}
				if !errors.Is(err, context.Canceled) {
					log.Warn().Err(err).Msg("pusher: replay failed")
				}
			}
		case payload := <-p.in:
			raw, err := json.Marshal(payload)
			if err != nil {
				log.Error().Err(err).Msg("pusher: marshal metrics payload")
				continue
			}

			err = p.deliverWithRetry(ctx, raw)
			if err == nil {
				continue
			}
			if errors.Is(err, ErrUnauthorized) {
				return err
			}

			if qErr := p.buffer.Enqueue(raw); qErr != nil {
				log.Error().Err(qErr).Msg("pusher: enqueue failed payload")
			}
		}
	}
}

func (p *Pusher) replayOnce(ctx context.Context) error {
	return p.buffer.Flush(ctx, func(c context.Context, payload []byte) error {
		return p.sendRaw(c, payload)
	})
}

func (p *Pusher) deliverWithRetry(ctx context.Context, payload []byte) error {
	attempts := p.maxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		err := p.sendRaw(ctx, payload)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrUnauthorized) {
			log.Error().Err(err).Msg("pusher: unauthorized response, stopping")
			return err
		}

		// Final attempt exhausted.
		if attempt == attempts-1 {
			return err
		}

		wait := p.backoffForAttempt(attempt)
		if errors.Is(err, errRateLimited) {
			wait = p.rateLimitBackoff
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil
}

func (p *Pusher) backoffForAttempt(attempt int) time.Duration {
	if len(p.backoffs) == 0 {
		return 0
	}
	if attempt < 0 {
		return p.backoffs[0]
	}
	if attempt >= len(p.backoffs) {
		return p.backoffs[len(p.backoffs)-1]
	}
	return p.backoffs[attempt]
}

func (p *Pusher) sendRaw(ctx context.Context, payload []byte) error {
	url := fmt.Sprintf("%s/ingest/metrics", p.cfg.Hub.CollectorURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pusher: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", p.cfg.AgentID)
	req.Header.Set("X-Agent-Key", p.cfg.Hub.AgentKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("pusher: transport error: %w", err)
	}
	_ = resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusTooManyRequests:
		return errRateLimited
	default:
		return fmt.Errorf("%w: status=%d", errRetryable, resp.StatusCode)
	}
}

// SendBuffered exposes single-send behavior for buffer replay callers.
func (p *Pusher) SendBuffered(ctx context.Context, payload []byte) error {
	return p.sendRaw(ctx, payload)
}
