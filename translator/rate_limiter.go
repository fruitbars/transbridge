package translator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"transbridge/config"
)

// LimiterStats 描述某个 model 当前的限流状态。
// 上游模型的并发和 QPS/QPM 各自独立，都需要观察是否接近上限。
type LimiterStats struct {
	MaxConcurrent int `json:"max_concurrent"`
	InFlight      int `json:"in_flight"` // 当前有多少个请求正占着 semaphore
	Waiting       int `json:"waiting"`   // 当前有多少 goroutine 正等着 semaphore/rate（近似值）
	QPSLimit      int `json:"qps_limit"`
	QPSUsed       int `json:"qps_used"` // 最近 1s 内已消费的请求数
	QPMLimit      int `json:"qpm_limit"`
	QPMUsed       int `json:"qpm_used"` // 最近 60s 内已消费的请求数
}

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

// Stats 返回当前 limiter 的实时使用情况。
func (t *rateLimitedTranslator) Stats() LimiterStats {
	if t.limiter == nil {
		return LimiterStats{}
	}
	return t.limiter.Stats()
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
	sem        chan struct{}
	maxConc    int
	maxWaiting int   // 0 = 不限
	waiting    int64 // atomic：正在等 Acquire 的 goroutine 数（近似）

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

// DefaultMaxWaitingMultiplier 决定等待队列上限：waiting > maxConcurrent * this 时直接拒绝。
// 20 意味着 maxConcurrent=10 时最多允许 200 个 goroutine 排队。这个值只影响防溢出兜底，
// 正常场景不会触发。设为 0 关闭上限（不推荐，OOM 风险）。
const DefaultMaxWaitingMultiplier = 20

// ErrQueueFull 等待队列已满，调用方应该拒绝该请求或延后重试。
var ErrQueueFull = errors.New("rate limiter queue full: too many pending requests")

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
		limiter.maxConc = cfg.MaxConcurrent
		limiter.maxWaiting = cfg.MaxConcurrent * DefaultMaxWaitingMultiplier
	}
	return limiter
}

// Stats 返回一份快照。qps_used / qpm_used 会同时触发一次窗口清理，
// 因此调用是有一点点副作用的（不会影响其他状态，只是把过期时间戳丢掉）。
func (l *upstreamLimiter) Stats() LimiterStats {
	s := LimiterStats{
		MaxConcurrent: l.maxConc,
		Waiting:       int(atomic.LoadInt64(&l.waiting)),
	}
	if l.sem != nil {
		s.InFlight = len(l.sem)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.perSec = pruneWindow(l.perSec, now, time.Second)
	l.perMin = pruneWindow(l.perMin, now, time.Minute)
	s.QPSLimit = l.qps
	s.QPSUsed = len(l.perSec)
	s.QPMLimit = l.qpm
	s.QPMUsed = len(l.perMin)
	return s
}

func (l *upstreamLimiter) Acquire(ctx context.Context) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// 队列过长直接拒绝，防止 goroutine 堆积压爆内存或让请求排上千秒。
	if l.maxWaiting > 0 && atomic.LoadInt64(&l.waiting) >= int64(l.maxWaiting) {
		return nil, ErrQueueFull
	}
	atomic.AddInt64(&l.waiting, 1)
	defer atomic.AddInt64(&l.waiting, -1)
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
