package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/idiey/api-server/internal/handler"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	db, err := openDB()
	if err != nil {
		log.Fatal().Err(err).Msg("api-server: db init failed")
	}
	if db != nil {
		defer db.Close()
	}

	r := chi.NewRouter()
	r.Get("/health", handler.Health)
	r.Get("/api/fleet", handler.Fleet(db))

	addr := envDefault("API_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("api-server: listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("api-server: listen failed")
		}
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-stopCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("api-server: shutdown error")
	}
}

func openDB() (*sql.DB, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Warn().Msg("api-server: DATABASE_URL empty, /api/fleet returns empty list")
		return nil, nil
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
