package ocr

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// ElementTrace 单个 element 在服务端走过的分支和产出关键指标。
// 一次 OCR 请求会产生 N 条 ElementTrace，配合完整 request/response，
// 用于离线分析 policy 决策分布、内联标签 round-trip 成功率等。
type ElementTrace struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Route            string `json:"route"` // skip_type / reference / table_html / language_skip / caption / markdown / html_reject / text / unknown
	Translated       bool   `json:"translated"`
	Reason           string `json:"reason,omitempty"`
	Error            string `json:"error,omitempty"`
	ElapsedMs        int64  `json:"elapsed_ms"`
	CellsTotal       int    `json:"cells_total,omitempty"`
	CellsTranslated  int    `json:"cells_translated,omitempty"`
	CellsSkipped     int    `json:"cells_skipped,omitempty"`
	PlaceholderCount int    `json:"placeholder_count,omitempty"`
}

// DebugRecord 一次 OCR 请求的完整可回放记录。
type DebugRecord struct {
	Timestamp    string         `json:"ts"`
	RequestID    string         `json:"request_id"`
	SourceLang   string         `json:"source_lang,omitempty"`
	TargetLang   string         `json:"target_lang"`
	ElapsedMs    int64          `json:"elapsed_ms"`
	ElementCount int            `json:"element_count"`
	Request      *OCRRequest    `json:"request,omitempty"`
	Response     *OCRResponse   `json:"response,omitempty"`
	Trace        []ElementTrace `json:"trace,omitempty"`
}

// DebugLogger 异步 JSONL 写盘 + 尺寸触发的日志滚动。零值为 nil-safe：所有方法都容忍 receiver 为 nil。
type DebugLogger struct {
	path         string
	maxSizeBytes int64
	maxFiles     int

	ch   chan DebugRecord
	done chan struct{}

	closeOnce sync.Once

	mu   sync.Mutex
	file *os.File
	size int64
}

// NewDebugLogger 打开或创建 path 指向的 JSONL 文件，启动后台写入 goroutine。
// path 为空返回 (nil, nil)，调用方无需感知调试模式是否开启。
// maxSizeMB / maxFiles 若 ≤ 0，采用默认 100 / 5。
func NewDebugLogger(path string, maxSizeMB, maxFiles int) (*DebugLogger, error) {
	if path == "" {
		return nil, nil
	}
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if maxFiles <= 0 {
		maxFiles = 5
	}
	l := &DebugLogger{
		path:         path,
		maxSizeBytes: int64(maxSizeMB) * 1024 * 1024,
		maxFiles:     maxFiles,
		ch:           make(chan DebugRecord, 128),
		done:         make(chan struct{}),
	}
	if err := l.openFile(); err != nil {
		return nil, fmt.Errorf("open ocr debug log: %w", err)
	}
	go l.loop()
	return l, nil
}

// Log 非阻塞投递一条记录。当内部队列满时，静默丢弃以避免拖慢 handler；
// 这优先保证正常翻译请求不被调试功能影响。
func (l *DebugLogger) Log(rec DebugRecord) {
	if l == nil {
		return
	}
	select {
	case l.ch <- rec:
	default:
		atomic.AddInt64(&debugDroppedCounter, 1)
	}
}

// Close 关闭 channel 并等待剩余记录落盘。多次调用安全（后续调用是 no-op）。
func (l *DebugLogger) Close() {
	if l == nil {
		return
	}
	l.closeOnce.Do(func() {
		close(l.ch)
		<-l.done
		l.mu.Lock()
		if l.file != nil {
			l.file.Close()
			l.file = nil
		}
		l.mu.Unlock()
	})
}

func (l *DebugLogger) loop() {
	defer close(l.done)
	for rec := range l.ch {
		l.write(rec)
	}
}

func (l *DebugLogger) write(rec DebugRecord) {
	data, err := json.Marshal(rec)
	if err != nil {
		log.Printf("ocr debug: marshal record: %v", err)
		return
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	if l.size+int64(len(data)) > l.maxSizeBytes {
		if err := l.rotateLocked(); err != nil {
			log.Printf("ocr debug: rotate: %v", err)
			return
		}
	}
	n, err := l.file.Write(data)
	if err != nil {
		log.Printf("ocr debug: write: %v", err)
		return
	}
	l.size += int64(n)
}

// rotateLocked 假设 mu 已被持有。
// path.jsonl -> path.jsonl.1；已有 path.jsonl.N 依次后推；超过 maxFiles 的最老文件被删。
func (l *DebugLogger) rotateLocked() error {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	// 先删掉最老的
	oldest := fmt.Sprintf("%s.%d", l.path, l.maxFiles)
	_ = os.Remove(oldest)
	// 从大到小依次改名
	for i := l.maxFiles - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", l.path, i)
		dst := fmt.Sprintf("%s.%d", l.path, i+1)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return err
			}
		}
	}
	// 当前文件 -> .1
	if _, err := os.Stat(l.path); err == nil {
		if err := os.Rename(l.path, l.path+".1"); err != nil {
			return err
		}
	}
	return l.openFileLocked()
}

func (l *DebugLogger) openFile() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.openFileLocked()
}

func (l *DebugLogger) openFileLocked() error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	l.file = f
	l.size = stat.Size()
	return nil
}

// debugDroppedCounter 记录因队列满被丢弃的记录数。可通过外部埋点/日志暴露。
var debugDroppedCounter int64

// DebugDropped 返回启动至今被丢弃的记录数（读一次不重置）。
func DebugDropped() int64 { return atomic.LoadInt64(&debugDroppedCounter) }

// newRequestID 生成一个人类可读、够独立的 request id：ocr-<unixNano-base36>-<counter>。
var requestIDCounter uint64

func newRequestID() string {
	n := atomic.AddUint64(&requestIDCounter, 1)
	return fmt.Sprintf("ocr-%d-%d", time.Now().UnixNano(), n)
}
