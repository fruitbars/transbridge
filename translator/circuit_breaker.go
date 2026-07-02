package translator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// circuitBreakerTranslator 在底层 translator 连续失败时快速失败，避免堆积无效请求。
// 简化版：只有 closed（正常）和 open（熔断）两个状态，open 后固定时间自动恢复。
type circuitBreakerTranslator struct {
	inner Translator

	mu               sync.Mutex
	consecutiveFails int32
	openUntil        time.Time
	failureThreshold int32         // 连续失败几次后熔断，默认 5
	openDuration     time.Duration // 熔断持续时间，默认 30s
}

// newCircuitBreakerTranslator 包装一个 translator，加熔断保护。
// failureThreshold: 连续失败几次后熔断（0 = 默认 5）
// openDuration: 熔断后多久自动恢复（0 = 默认 30s）
func newCircuitBreakerTranslator(inner Translator, failureThreshold int, openDuration time.Duration) Translator {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if openDuration <= 0 {
		openDuration = 30 * time.Second
	}
	return &circuitBreakerTranslator{
		inner:            inner,
		failureThreshold: int32(failureThreshold),
		openDuration:     openDuration,
	}
}

func (t *circuitBreakerTranslator) Translate(ctx context.Context, promptTemplate, text, sourceLang, targetLang string) (string, error) {
	// 检查熔断状态
	t.mu.Lock()
	if time.Now().Before(t.openUntil) {
		// 仍在熔断期，快速失败
		fails := atomic.LoadInt32(&t.consecutiveFails)
		nextRetry := t.openUntil
		t.mu.Unlock()
		return "", fmt.Errorf("circuit breaker open: upstream failed %d times consecutively, retry after %s", fails, nextRetry.Format("15:04:05"))
	}
	// 熔断期已过，允许通过
	t.mu.Unlock()

	result, err := t.inner.Translate(ctx, promptTemplate, text, sourceLang, targetLang)
	if err != nil {
		// 失败：递增计数，判断是否触发熔断
		fails := atomic.AddInt32(&t.consecutiveFails, 1)
		if fails >= t.failureThreshold {
			t.mu.Lock()
			t.openUntil = time.Now().Add(t.openDuration)
			t.mu.Unlock()
		}
		return "", err
	}
	// 成功：重置计数
	atomic.StoreInt32(&t.consecutiveFails, 0)
	return result, nil
}

func (t *circuitBreakerTranslator) GetAPIURL() string {
	return t.inner.GetAPIURL()
}

func (t *circuitBreakerTranslator) GetModel() string {
	return t.inner.GetModel()
}

func (t *circuitBreakerTranslator) GetProvider() string {
	return t.inner.GetProvider()
}

func (t *circuitBreakerTranslator) Close() error {
	return t.inner.Close()
}

// CircuitState 返回熔断器状态快照。
func (t *circuitBreakerTranslator) CircuitState() (isOpen bool, consecutiveFails int, openUntil time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	isOpen = time.Now().Before(t.openUntil)
	return isOpen, int(atomic.LoadInt32(&t.consecutiveFails)), t.openUntil
}
