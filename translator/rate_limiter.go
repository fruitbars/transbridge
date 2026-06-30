package translator

import (
	"context"
	"sync"
	"time"

	"transbridge/config"
)

type rateLimitedTranslator struct {
	inner   Translator
	limiter *upstreamLimiter
}

func newRateLimitedTranslator(inner Translator, cfg config.RateLimitConfig) Translator {
	if cfg.MaxConcurrent <= 0 && cfg.QPS <= 0 && cfg.QPM <= 0 {
		return inner
	}
	return &rateLimitedTranslator{
		inner:   inner,
		limiter: newUpstreamLimiter(cfg),
	}
}

func (t *rateLimitedTranslator) Translate(ctx context.Context, promptTemplate, text, sourceLang, targetLang string) (string, error) {
	release, err := t.limiter.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	return t.inner.Translate(ctx, promptTemplate, text, sourceLang, targetLang)
}

func (t *rateLimitedTranslator) GetAPIURL() string {
	return t.inner.GetAPIURL()
}

func (t *rateLimitedTranslator) GetModel() string {
	return t.inner.GetModel()
}

func (t *rateLimitedTranslator) GetProvider() string {
	return t.inner.GetProvider()
}

func (t *rateLimitedTranslator) Close() error {
	return t.inner.Close()
}

func (t *rateLimitedTranslator) Unwrap() Translator {
	return t.inner
}

type upstreamLimiter struct {
	sem chan struct{}

	mu       sync.Mutex
	qps      int
	qpm      int
	perSec   []time.Time
	perMin   []time.Time
	now      func() time.Time
	newTimer func(time.Duration) waitTimer
}

type waitTimer interface {
	C() <-chan time.Time
	Stop() bool
}

type realTimer struct {
	timer *time.Timer
}

func (t realTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t realTimer) Stop() bool {
	return t.timer.Stop()
}

func newUpstreamLimiter(cfg config.RateLimitConfig) *upstreamLimiter {
	limiter := &upstreamLimiter{
		qps: cfg.QPS,
		qpm: cfg.QPM,
		now: time.Now,
		newTimer: func(d time.Duration) waitTimer {
			return realTimer{timer: time.NewTimer(d)}
		},
	}
	if cfg.MaxConcurrent > 0 {
		limiter.sem = make(chan struct{}, cfg.MaxConcurrent)
	}
	return limiter
}

func (l *upstreamLimiter) Acquire(ctx context.Context) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	release := func() {}
	if l.sem != nil {
		select {
		case l.sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		release = func() { <-l.sem }
	}
	if err := l.acquireRate(ctx); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func (l *upstreamLimiter) acquireRate(ctx context.Context) error {
	for {
		wait := l.reserve()
		if wait <= 0 {
			return nil
		}
		timer := l.newTimer(wait)
		select {
		case <-timer.C():
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

func (l *upstreamLimiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.perSec = pruneWindow(l.perSec, now, time.Second)
	l.perMin = pruneWindow(l.perMin, now, time.Minute)

	wait := maxWait(l.perSec, l.qps, now, time.Second)
	if minuteWait := maxWait(l.perMin, l.qpm, now, time.Minute); minuteWait > wait {
		wait = minuteWait
	}
	if wait > 0 {
		return wait
	}

	if l.qps > 0 {
		l.perSec = append(l.perSec, now)
	}
	if l.qpm > 0 {
		l.perMin = append(l.perMin, now)
	}
	return 0
}

func pruneWindow(events []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	idx := 0
	for idx < len(events) && !events[idx].After(cutoff) {
		idx++
	}
	if idx == 0 {
		return events
	}
	copy(events, events[idx:])
	return events[:len(events)-idx]
}

func maxWait(events []time.Time, limit int, now time.Time, window time.Duration) time.Duration {
	if limit <= 0 || len(events) < limit {
		return 0
	}
	wait := events[0].Add(window).Sub(now)
	if wait < 0 {
		return 0
	}
	return wait
}
