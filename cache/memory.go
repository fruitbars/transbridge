package cache

import (
	"context"
	"sync"
	"time"
)

// memoryItem 内部缓存项结构
type memoryItem struct {
	data       string     // 存储的数据
	expireTime *time.Time // 过期时间，nil表示永不过期
}

// MemoryCache 实现了内存缓存
type MemoryCache struct {
	sync.RWMutex
	data       map[string]memoryItem
	maxSize    int
	defaultTTL time.Duration
	permanent  bool
	stop       chan struct{} // 用于停止清理 goroutine
	closeOnce  sync.Once
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache(opts MemoryCacheOptions) *MemoryCache {
	if opts.MaxSize <= 0 {
		opts.MaxSize = 10000000
	}

	if opts.DefaultTTL <= 0 && !opts.Permanent {
		opts.DefaultTTL = time.Hour
	}

	cache := &MemoryCache{
		data:       make(map[string]memoryItem),
		maxSize:    opts.MaxSize,
		defaultTTL: opts.DefaultTTL,
		permanent:  opts.Permanent,
		stop:       make(chan struct{}),
	}

	// 如果不是永久存储，启动清理过期数据的 goroutine
	if !opts.Permanent {
		go cache.cleanExpired()
	}

	return cache
}

func (c *MemoryCache) cleanExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Lock()
			now := time.Now()
			for key, item := range c.data {
				if item.expireTime != nil && item.expireTime.Before(now) {
					delete(c.data, key)
				}
			}
			c.Unlock()
		case <-c.stop:
			return
		}
	}
}

func (c *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	c.RLock()
	item, ok := c.data[key]
	if !ok {
		c.RUnlock()
		return "", ErrCacheMiss
	}

	// 检查是否过期
	if item.expireTime != nil && item.expireTime.Before(time.Now()) {
		c.RUnlock()
		c.Lock()
		if current, exists := c.data[key]; exists && current.expireTime != nil && current.expireTime.Before(time.Now()) {
			delete(c.data, key)
		}
		c.Unlock()
		return "", ErrCacheMiss
	}

	c.RUnlock()
	return item.data, nil
}

func (c *MemoryCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.Lock()
	defer c.Unlock()

	// 如果达到最大容量，删除一个随机条目
	if len(c.data) >= c.maxSize {
		for k := range c.data {
			delete(c.data, k)
			break
		}
	}

	var expireTime *time.Time

	// 处理过期时间
	if ttl < 0 || c.permanent {
		// 永不过期
		expireTime = nil
	} else if ttl == 0 {
		// 使用默认过期时间
		if c.defaultTTL > 0 {
			t := time.Now().Add(c.defaultTTL)
			expireTime = &t
		} else {
			expireTime = nil // 默认过期时间为0也表示永久
		}
	} else {
		// 使用指定的过期时间
		t := time.Now().Add(ttl)
		expireTime = &t
	}

	c.data[key] = memoryItem{
		data:       value,
		expireTime: expireTime,
	}
	return nil
}

func (c *MemoryCache) Clear(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()
	c.data = make(map[string]memoryItem)
	return nil
}

// Close 实现 Cache 接口
func (c *MemoryCache) Close(ctx context.Context) error {
	c.closeOnce.Do(func() {
		close(c.stop) // 停止清理 goroutine
	})
	c.Lock()
	c.data = nil // 清空数据
	c.Unlock()
	return nil
}
