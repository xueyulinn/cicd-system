package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the only package that talks to the report database.
// Execution and report services call its methods instead of using the DB directly.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store that uses the given PostgreSQL connection URL.
// The URL should be in the form: postgres://user:pass@host:port/dbname?sslmode=disable
func New(ctx context.Context, connURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close closes the connection pool. Call when shutting down the service.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
