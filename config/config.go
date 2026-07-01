// config/config.go
package config

import (
	"gopkg.in/yaml.v2"
	"os"
)

type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Providers []ProviderConfig `yaml:"providers"`
	Cache     CacheConfig      `yaml:"cache"`
	Prompt    PromptConfig     `yaml:"prompt"`
	OpenAI    OpenAIConfig     `yaml:"openai"`   // 新增 OpenAI 配置
	TransAPI  TransAPI         `yaml:"transapi"` // 新增认证配置
	Log       LogConfig        `yaml:"log"`      // 新增日志配置
	Storage   StorageConfig    `yaml:"storage"`  // 管理后台持久化配置
	Admin     AdminConfig      `yaml:"admin"`    // 本地管理后台配置
	OCR       OCRConfig        `yaml:"ocr"`      // /ocr/translate 相关配置
}

// OCRConfig /ocr/translate 接口的可选配置。默认全为空/零，接口本身仍可用。
// debug_log_path 非空时，服务端把每次请求的 request / response / trace 以 JSONL 写盘，
// 供调用方回收数据后离线分析（policy 决策分布、模型翻译质量等）。
type OCRConfig struct {
	MaxElements       int    `yaml:"max_elements"`           // /ocr/translate 单请求 elements 上限，0 或未设 = 2000
	DebugLogPath      string `yaml:"debug_log_path"`         // JSONL 落盘路径，空 = 关闭调试日志
	DebugLogMaxSizeMB int    `yaml:"debug_log_max_size_mb"`  // 单文件上限，默认 100
	DebugLogMaxFiles  int    `yaml:"debug_log_max_files"`    // 保留历史文件数，默认 5
}

// LogConfig 日志配置
type LogConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用日志
	FilePath   string `yaml:"file_path"`   // 日志文件路径
	MaxSize    int    `yaml:"max_size"`    // 日志文件最大大小（MB）
	MaxAge     int    `yaml:"max_age"`     // 旧文件保留最大天数
	MaxBackups int    `yaml:"max_backups"` // 最大备份文件数
	QueueSize  int    `yaml:"queue_size"`  // 异步队列大小
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type ProviderConfig struct {
	Provider  string          `yaml:"provider"`
	APIURL    string          `yaml:"api_url"`
	APIKey    string          `yaml:"api_key"`
	Timeout   int             `yaml:"timeout"`
	IsDefault bool            `yaml:"is_default"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Models    []ModelConfig   `yaml:"models"`
}

type ModelConfig struct {
	Name        string          `yaml:"name"`
	Weight      int             `yaml:"weight"`
	TopP        int             `yaml:"top_p"`
	MaxTokens   int             `yaml:"max_tokens"`
	Temperature float32         `yaml:"temperature"`
	Timeout     *int            `yaml:"timeout,omitempty"`
	RateLimit   RateLimitConfig `yaml:"rate_limit"`
}

// RateLimitConfig 控制单个上游模型的调用限制。0 表示不限制。
type RateLimitConfig struct {
	MaxConcurrent int `yaml:"max_concurrent"`
	QPS           int `yaml:"qps"`
	QPM           int `yaml:"qpm"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled bool         `yaml:"enabled"`
	Types   []string     `yaml:"types"`  // 支持的缓存类型：["memory", "redis", "bbolt"]
	Memory  MemoryConfig `yaml:"memory"` // 内存缓存特定配置
	Redis   RedisConfig  `yaml:"redis"`  // Redis缓存特定配置
	Bbolt   BboltConfig  `yaml:"bbolt"`  // bbolt缓存特定配置
}

// MemoryConfig 内存缓存特定配置
type MemoryConfig struct {
	TTL     TTL `yaml:"ttl"`      // 缓存过期时间
	MaxSize int `yaml:"max_size"` // 内存缓存最大条目数
}

// RedisConfig Redis缓存特定配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	TTL      TTL    `yaml:"ttl"` // Redis缓存特定的TTL
}

// BboltConfig bbolt缓存特定配置
type BboltConfig struct {
	Path string `yaml:"path"` // 数据库文件路径
	TTL  TTL    `yaml:"ttl"`  // bbolt缓存特定的TTL
}

type PromptConfig struct {
	Template string `yaml:"template"`
}

// OpenAIConfig OpenAI 兼容接口配置
type OpenAIConfig struct {
	CompatibleAPI struct {
		Enabled    bool     `yaml:"enabled"`     // 是否启用 OpenAI 兼容接口
		Path       string   `yaml:"path"`        // API 路径前缀
		AuthTokens []string `yaml:"auth_tokens"` // 认证令牌列表
	} `yaml:"compatible_api"`
}

type StorageConfig struct {
	Enabled  bool   `yaml:"enabled"`   // 是否启用存储
	Type     string `yaml:"type"`      // 存储类型，如 "sqlite"
	Path     string `yaml:"path"`      // SQLite 数据库文件路径
	LogLevel string `yaml:"log_level"` // 日志级别：none, error, warn, info, debug
}

type AdminConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Path      string `yaml:"path"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	LocalOnly bool   `yaml:"local_only"`
}

type TransAPI struct {
	Tokens []string `yaml:"tokens"` // API 密钥列表
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
