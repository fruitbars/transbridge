package translator

import (
	"context"
	"sync"
	"testing"
	"time"

	"transbridge/config"
)

func TestMergeRateLimitUsesModelOverrides(t *testing.T) {
	got := mergeRateLimit(
		config.RateLimitConfig{MaxConcurrent: 10, QPS: 20, QPM: 100},
		config.RateLimitConfig{QPS: 5},
	)

	if got.MaxConcurrent != 10 || got.QPS != 5 || got.QPM != 100 {
		t.Fatalf("unexpected merged rate limit: %+v", got)
	}
}

func TestUpstreamLimiterMaxConcurrent(t *testing.T) {
	limiter := newUpstreamLimiter(config.RateLimitConfig{MaxConcurrent: 1})
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := limiter.Acquire(ctx); err == nil {
		t.Fatal("expected second acquire to wait until context timeout")
	}
}

func TestUpstreamLimiterDoesNotReserveRateBeforeConcurrency(t *testing.T) {
	limiter := newUpstreamLimiter(config.RateLimitConfig{MaxConcurrent: 1, QPS: 10, QPM: 10})
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := limiter.Acquire(ctx); err == nil {
		t.Fatal("expected second acquire to time out waiting for concurrency")
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.perSec) != 1 || len(limiter.perMin) != 1 {
		t.Fatalf("timed-out acquire reserved rate slots: perSec=%d perMin=%d", len(limiter.perSec), len(limiter.perMin))
	}
}

func TestUpstreamLimiterSlidingWindow(t *testing.T) {
	limiter := newUpstreamLimiter(config.RateLimitConfig{QPS: 2})
	current := time.Unix(100, 0)
	limiter.now = func() time.Time { return current }
	limiter.newTimer = func(d time.Duration) waitTimer {
		current = current.Add(d)
		return fakeTimer{ch: closedTimeChannel(current)}
	}

	for i := 0; i < 3; i++ {
		release, err := limiter.Acquire(context.Background())
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		release()
	}

	if current.Sub(time.Unix(100, 0)) != time.Second {
		t.Fatalf("expected third acquire to wait 1s, waited %v", current.Sub(time.Unix(100, 0)))
	}
}

func TestRateLimitedTranslatorWrapsInner(t *testing.T) {
	inner := &stubTranslator{}
	wrapped := newRateLimitedTranslator(inner, config.RateLimitConfig{QPS: 1})

	if _, ok := wrapped.(*rateLimitedTranslator); !ok {
		t.Fatal("expected rate limited wrapper")
	}

	if _, err := wrapped.Translate(context.Background(), "{{input}}", "hello", "EN", "ZH"); err != nil {
		t.Fatalf("translate: %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("expected inner call, got %d", inner.calls)
	}
}

type fakeTimer struct {
	ch <-chan time.Time
}

func (t fakeTimer) C() <-chan time.Time {
	return t.ch
}

func (t fakeTimer) Stop() bool {
	return true
}

func closedTimeChannel(value time.Time) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- value
	close(ch)
	return ch
}

type stubTranslator struct {
	mu    sync.Mutex
	calls int
}

func (t *stubTranslator) Translate(ctx context.Context, promptTemplate, text, sourceLang, targetLang string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	return text, nil
}

func (t *stubTranslator) GetAPIURL() string {
	return "http://example.test/v1/chat/completions"
}

func (t *stubTranslator) GetModel() string {
	return "test-model"
}

func (t *stubTranslator) GetProvider() string {
	return "openai"
}

func (t *stubTranslator) Close() error {
	return nil
}
