package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type PostgresLogStore struct{ db *sql.DB }

func NewPostgresLogStore(ctx context.Context, databaseURL string) (*PostgresLogStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &PostgresLogStore{db: db}, nil
}

func (s *PostgresLogStore) Query(ctx context.Context, f LogFilter) ([]LogRow, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var cursorID int64
	if f.Cursor != "" {
		cursorID, _ = strconv.ParseInt(f.Cursor, 10, 64)
	}
	var conds []string
	var args []interface{}
	i := 1
	add := func(cond string, val interface{}) {
		conds = append(conds, fmt.Sprintf(cond, i))
		args = append(args, val)
		i++
	}
	if cursorID > 0 {
		add("id > $%d", cursorID)
	}
	if f.Service != "" {
		add("service = $%d", f.Service)
	}
	if f.Level != "" {
		add("level = $%d", f.Level)
	}
	if f.TraceID != "" {
		add("trace_id = $%d", f.TraceID)
	}
	if f.Environment != "" {
		add("environment = $%d", f.Environment)
	}
	if !f.From.IsZero() {
		add("received_at >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("received_at <= $%d", f.To)
	}
	if f.Search != "" {
		add("message ILIKE $%d", "%"+f.Search+"%")
	}
	q := "SELECT id, agent_id, service, environment, level, message, COALESCE(trace_id,''), fields, received_at FROM logs"
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY id ASC LIMIT $%d", i)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var result []LogRow
	for rows.Next() {
		var row LogRow
		var fields []byte
		if err := rows.Scan(&row.ID, &row.AgentID, &row.Service, &row.Environment,
			&row.Level, &row.Message, &row.TraceID, &fields, &row.ReceivedAt); err != nil {
			return nil, "", fmt.Errorf("scan: %w", err)
		}
		row.Fields = json.RawMessage(fields)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(result) > limit {
		result = result[:limit]
		nextCursor = strconv.FormatInt(result[len(result)-1].ID, 10)
	}
	return result, nextCursor, nil
}

func (s *PostgresLogStore) GetByTraceID(ctx context.Context, traceID string) ([]LogRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_id, service, environment, level, message, COALESCE(trace_id,''), fields, received_at
		 FROM logs WHERE trace_id = $1 ORDER BY id ASC`, traceID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var result []LogRow
	for rows.Next() {
		var row LogRow
		var fields []byte
		if err := rows.Scan(&row.ID, &row.AgentID, &row.Service, &row.Environment,
			&row.Level, &row.Message, &row.TraceID, &fields, &row.ReceivedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		row.Fields = json.RawMessage(fields)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *PostgresLogStore) Stats(ctx context.Context) (LogStats, error) {
	var stats LogStats
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE level='error'),
		 COUNT(*) FILTER (WHERE level IN ('warn','warning')),
		 COUNT(*) FILTER (WHERE level='info') FROM logs`).
		Scan(&stats.Total, &stats.Errors, &stats.Warnings, &stats.Info)
	if err != nil {
		return stats, fmt.Errorf("stats: %w", err)
	}
	svcRows, err := s.db.QueryContext(ctx, "SELECT DISTINCT service FROM logs ORDER BY service")
	if err != nil {
		return stats, err
	}
	defer svcRows.Close()
	for svcRows.Next() {
		var svc string
		if err := svcRows.Scan(&svc); err != nil {
			return stats, err
		}
		stats.Services = append(stats.Services, svc)
	}
	return stats, svcRows.Err()
}
