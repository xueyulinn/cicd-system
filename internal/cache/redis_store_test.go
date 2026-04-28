package cache

import (
	"testing"
	"time"
)

func TestNewStoreFromConfig_DisabledReturnsNoop(t *testing.T) {
	store, err := NewStoreFromConfig(Config{Enabled: false})
	if err != nil {
		t.Fatalf("NewStoreFromConfig returned error: %v", err)
	}

	if _, ok := store.(NoopStore); !ok {
		t.Fatalf("store type = %T, want cache.NoopStore", store)
	}
}

func TestNewRedisStore_InvalidURL(t *testing.T) {
	_, err := NewRedisStore(Config{
		Enabled:  true,
		RedisURL: "://bad-redis-url",
	})
	if err == nil {
		t.Fatal("NewRedisStore returned nil error, want parse failure")
	}
}

func TestNewRedisStore_AppliesTimeouts(t *testing.T) {
	cfg := Config{
		Enabled:      true,
		RedisURL:     "redis://localhost:6379/2",
		DialTimeout:  3 * time.Second,
		ReadTimeout:  800 * time.Millisecond,
		WriteTimeout: 900 * time.Millisecond,
	}

	store, err := NewRedisStore(cfg)
	if err != nil {
		t.Fatalf("NewRedisStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	opts := store.client.Options()
	if opts.DialTimeout != cfg.DialTimeout {
		t.Fatalf("DialTimeout = %v, want %v", opts.DialTimeout, cfg.DialTimeout)
	}
	if opts.ReadTimeout != cfg.ReadTimeout {
		t.Fatalf("ReadTimeout = %v, want %v", opts.ReadTimeout, cfg.ReadTimeout)
	}
	if opts.WriteTimeout != cfg.WriteTimeout {
		t.Fatalf("WriteTimeout = %v, want %v", opts.WriteTimeout, cfg.WriteTimeout)
	}
}
