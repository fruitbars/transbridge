package cache

import "time"

// MemoryCacheOptions 内存缓存选项
type MemoryCacheOptions struct {
	MaxSize    int           // 最大缓存条目数
	DefaultTTL time.Duration // 默认过期时间
	Permanent  bool          // 是否永久存储
}

// RedisCacheOptions Redis缓存选项
type RedisCacheOptions struct {
	Host       string
	Port       int
	Password   string
	DB         int
	DefaultTTL time.Duration // 默认过期时间
	Permanent  bool          // 是否永久存储
}

// BboltCacheOptions bbolt 缓存选项
type BboltCacheOptions struct {
	Path       string
	DefaultTTL time.Duration
	Permanent  bool
}
