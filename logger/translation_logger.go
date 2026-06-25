package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// TranslationRecord 表示一条翻译记录
type TranslationRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	SourceText  string    `json:"source_text"`
	TargetText  string    `json:"target_text"`
	SourceLang  string    `json:"source_lang"`
	TargetLang  string    `json:"target_lang"`
	APIURL      string    `json:"api_url"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	CacheKey    string    `json:"cache_key"`
	CacheHit    bool      `json:"cache_hit"`
	ProcessTime float64   `json:"process_time_ms"`
}

// TranslationLogger 翻译日志记录器
type TranslationLogger struct {
	enabled   bool
	logger    *lumberjack.Logger
	queue     chan TranslationRecord
	wg        sync.WaitGroup
	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
}

// LoggerOptions 日志记录器选项
type LoggerOptions struct {
	Enabled     bool
	LogFilePath string
	MaxSize     int // 单位：MB
	MaxAge      int // 单位：天
	MaxBackups  int // 最大备份数量
	QueueSize   int // 异步队列大小
}

// NewTranslationLogger 创建一个新的翻译日志记录器
func NewTranslationLogger(opts LoggerOptions) (*TranslationLogger, error) {
	// 设置默认值
	if opts.LogFilePath == "" {
		opts.LogFilePath = "translation.log"
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 1000
	}

	// 确保目录存在
	dir := filepath.Dir(opts.LogFilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// 初始化 lumberjack logger
	fileLogger := &lumberjack.Logger{
		Filename:   opts.LogFilePath,
		MaxSize:    opts.MaxSize,    // 每个日志文件最大尺寸，单位：MB
		MaxAge:     opts.MaxAge,     // 保留旧文件的最大天数
		MaxBackups: opts.MaxBackups, // 保留旧文件的最大个数
		Compress:   true,            // 是否压缩旧文件
		LocalTime:  true,            // 使用本地时间
	}

	logger := &TranslationLogger{
		enabled: opts.Enabled,
		logger:  fileLogger,
		queue:   make(chan TranslationRecord, opts.QueueSize),
	}

	// 启动异步处理协程
	if logger.enabled {
		logger.wg.Add(1)
		go logger.processLogs()
	}

	return logger, nil
}

// processLogs 异步处理日志队列
func (l *TranslationLogger) processLogs() {
	defer l.wg.Done()

	for record := range l.queue {
		if err := l.writeLog(record); err != nil {
			fmt.Printf("Error writing log: %v\n", err)
		}
	}
}

// LogTranslation 记录一条翻译
func (l *TranslationLogger) LogTranslation(record TranslationRecord) error {
	if !l.enabled {
		return nil
	}

	// 设置时间戳
	record.Timestamp = time.Now()

	// 将记录放入队列
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return fmt.Errorf("logger is closed")
	}
	select {
	case l.queue <- record:
		// 成功入队
		return nil
	default:
		// 队列已满，返回错误
		return fmt.Errorf("log queue is full")
	}
}

// writeLog 实际写入日志文件
func (l *TranslationLogger) writeLog(record TranslationRecord) error {
	// 序列化记录
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// 写入日志
	if _, err := l.logger.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	return nil
}

// Close 关闭日志记录器
func (l *TranslationLogger) Close() error {
	if !l.enabled {
		return nil
	}

	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		close(l.queue)
		l.mu.Unlock()
	})
	l.wg.Wait()

	// 关闭日志文件
	return l.logger.Close()
}
