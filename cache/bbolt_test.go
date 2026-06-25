package cache

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestBboltCacheSetGetExpireAndClear(t *testing.T) {
	ctx := context.Background()
	c, err := NewBboltCache(BboltCacheOptions{
		Path:       filepath.Join(t.TempDir(), "cache.db"),
		DefaultTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("new bbolt cache: %v", err)
	}
	defer c.Close(ctx)

	if err := c.Set(ctx, "key", "value", time.Hour); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	value, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if value != "value" {
		t.Fatalf("expected value, got %q", value)
	}

	if err := c.Set(ctx, "expired", "value", time.Nanosecond); err != nil {
		t.Fatalf("set expiring cache: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := c.Get(ctx, "expired"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected cache miss, got %v", err)
	}

	if err := c.Clear(ctx); err != nil {
		t.Fatalf("clear cache: %v", err)
	}
	if _, err := c.Get(ctx, "key"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected cache miss after clear, got %v", err)
	}
}
