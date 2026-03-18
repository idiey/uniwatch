package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/idiey/api-server/internal/auth"
)

type LoginHandler struct{ db *sql.DB }

func NewLoginHandler(ctx context.Context) (*LoginHandler, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return &LoginHandler{}, nil
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &LoginHandler{db: db}, nil
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Username == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if h.db == nil {
		WriteError(w, http.StatusServiceUnavailable, "database not configured")
		return
	}
	var userID int
	var hash string
	err := h.db.QueryRowContext(r.Context(),
		"SELECT id, password FROM users WHERE username = $1", req.Username).Scan(&userID, &hash)
	if err == sql.ErrNoRows {
		WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("login db error")
		WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	tokenStr, err := auth.Issue(userID, req.Username)
	if err != nil {
		log.Error().Err(err).Msg("issue token")
		WriteError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	auth.SetCookie(w, tokenStr)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
