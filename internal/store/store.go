// Package store is the data-access layer for the report database.
// Execution and reporting services call its methods instead of using the DB directly.
package store

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

// Store provides transactional access to the report database via a MySQL connection pool.
type Store struct {
	db *sql.DB
}

// New creates a Store that uses the given MySQL connection DSN.
// Example: user:pass@tcp(host:3306)/dbname?parseTime=true&loc=UTC
func New(ctx context.Context, connURL string) (*Store, error) {
	db, err := sql.Open("mysql", connURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close the connection pool. Call when shutting down the service.
func (s *Store) Close() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
