package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var _ Store = (*RedisStore)(nil)

// RedisStore is a Redis-backed cache store implementation.
type RedisStore struct {
	client *redis.Client
}

// NewStoreFromConfig creates a cache store based on config.
// When caching is disabled, this returns a NoopStore.
func NewStoreFromConfig(cfg Config) (Store, error) {
	if !cfg.Enabled {
		return NewNoopStore(), nil
	}

	return NewRedisStore(cfg)
}

// NewRedisStore creates a Redis-backed store from cache config.
func NewRedisStore(cfg Config) (*RedisStore, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	}
	if cfg.ReadTimeout > 0 {
		opts.ReadTimeout = cfg.ReadTimeout
	}
	if cfg.WriteTimeout > 0 {
		opts.WriteTimeout = cfg.WriteTimeout
	}

	return &RedisStore{
		client: redis.NewClient(opts),
	}, nil
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	value, err := s.client.Get(ctx, key).Bytes()
	if err == nil {
		return value, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}

	return nil, err
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}

	return s.client.Close()
}
