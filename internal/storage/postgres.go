// Package storage provides database persistence layer implementations.
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antst/tg-throttle-bot/internal/storage/sqlc"
)

// PostgresStore wraps sqlc Queries and provides database connection management
type PostgresStore struct {
	*sqlc.Queries
	Pool *pgxpool.Pool // Export pool so adapter can access it
}

// NewPostgresStore creates a new PostgreSQL store with connection pooling.
// Validates the connection by pinging the database before returning.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStore{
		Queries: sqlc.New(pool),
		Pool:    pool,
	}, nil
}

// Close closes the database connection pool.
func (s *PostgresStore) Close() {
	s.Pool.Close()
}

// Ping checks if the database connection is alive.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.Pool.Ping(ctx)
}

// BeginTx starts a new transaction and returns a PostgresStore instance bound to that transaction.
func (s *PostgresStore) BeginTx(ctx context.Context) (*PostgresStore, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &PostgresStore{
		Queries: s.WithTx(tx),
		Pool:    s.Pool,
	}, nil
}

// ============================================================================
// Multi-Window Storage Adapter (Feature 006)
// ============================================================================
// The PostgresStore automatically implements MultiWindowStorage through embedding
// sqlc.Queries, which contains all the generated query methods. This file provides
// any additional adapter methods needed for the ratelimit.MultiWindowStorage interface.
