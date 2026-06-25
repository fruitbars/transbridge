package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMemoryCacheExpiredGetDeletesSafely(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(MemoryCacheOptions{
		MaxSize:    10,
		DefaultTTL: time.Millisecond,
	})
	defer c.Close(ctx)

	if err := c.Set(ctx, "key", "value", time.Millisecond); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Get(ctx, "key")
			if !errors.Is(err, ErrCacheMiss) {
				t.Errorf("expected cache miss, got %v", err)
			}
		}()
	}
	wg.Wait()
}
