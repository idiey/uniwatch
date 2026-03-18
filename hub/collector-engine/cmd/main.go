package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/idiey/collector-engine/internal/handler"
	"github.com/idiey/collector-engine/internal/middleware"
	"github.com/idiey/collector-engine/internal/store"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var s store.Store
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pg, err := store.NewPostgresStore(ctx, dbURL)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to postgres")
		}
		s = pg
		log.Info().Msg("using PostgreSQL store")
	} else {
		s = store.NewMemoryStore()
		log.Warn().Msg("DATABASE_URL not set - using in-memory store")
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"collector-engine"}`))
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.AgentAuth)
		r.Use(middleware.InjectAgentID)
		r.Post("/ingest/heartbeat", (&handler.HeartbeatHandler{Store: s}).ServeHTTP)
		r.Post("/ingest/metrics",   (&handler.MetricsHandler{Store: s}).ServeHTTP)
		r.Post("/ingest/logs",      (&handler.LogsHandler{Store: s}).ServeHTTP)
		r.Post("/ingest/apm",       (&handler.APMHandler{Store: s}).ServeHTTP)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	srv := &http.Server{
		Addr: ":" + port, Handler: r,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	go func() {
		log.Info().Str("port", port).Msg("collector-engine starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("forced shutdown")
	}
	log.Info().Msg("collector-engine stopped")
}
