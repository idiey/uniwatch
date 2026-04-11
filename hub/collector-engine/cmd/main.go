package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/idiey/collector-engine/internal/handler"
	"github.com/idiey/collector-engine/internal/middleware"
	"github.com/idiey/collector-engine/internal/store"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx := context.Background()
	st, err := buildStore(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("collector-engine: init store failed")
	}

	r := chi.NewRouter()
	r.Use(middleware.InjectAgentID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Group(func(pr chi.Router) {
		pr.Use(middleware.AgentAuth)
		pr.Post("/ingest/heartbeat", (&handler.HeartbeatHandler{Store: st}).ServeHTTP)
		pr.Post("/ingest/metrics", (&handler.MetricsHandler{Store: st}).ServeHTTP)
		pr.Post("/ingest/logs", (&handler.LogsHandler{Store: st}).ServeHTTP)
		pr.Post("/ingest/apm", (&handler.APMHandler{Store: st}).ServeHTTP)
	})

	addr := envDefault("COLLECTOR_ADDR", ":9090")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("collector-engine: listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("collector-engine: listen failed")
		}
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-stopCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("collector-engine: shutdown error")
	}
}

func buildStore(ctx context.Context) (store.Store, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Warn().Msg("collector-engine: DATABASE_URL empty, using in-memory store")
		return store.NewMemoryStore(), nil
	}
	return store.NewPostgresStore(ctx, databaseURL)
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
