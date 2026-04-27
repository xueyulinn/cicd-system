package cache

import (
	"context"
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

// Store is the minimal cache interface used by services.
// A Redis-backed implementation can satisfy this interface.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Ping(ctx context.Context) error
	Close() error
}

// NoopStore disables caching while keeping call sites simple.
type NoopStore struct{}

func NewNoopStore() Store {
	return NoopStore{}
}

func (NoopStore) Get(context.Context, string) ([]byte, error) {
	return nil, ErrCacheMiss
}

func (NoopStore) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (NoopStore) Ping(context.Context) error {
	return nil
}

func (NoopStore) Close() error {
	return nil
}

