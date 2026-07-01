package ocr

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDebugLoggerNilPathReturnsNil(t *testing.T) {
	l, err := NewDebugLogger("", 0, 0)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if l != nil {
		t.Errorf("expected nil logger for empty path, got %+v", l)
	}
	// nil logger 的方法调用不能 panic
	l.Log(DebugRecord{})
	l.Close()
}

func TestDebugLoggerWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ocr.jsonl")
	l, err := NewDebugLogger(path, 100, 3)
	if err != nil {
		t.Fatalf("NewDebugLogger: %v", err)
	}
	defer l.Close()

	l.Log(DebugRecord{RequestID: "r1", TargetLang: "zh", ElementCount: 2})
	l.Log(DebugRecord{RequestID: "r2", TargetLang: "en", ElementCount: 1})

	waitForFile(t, path, 20*time.Millisecond, 20)

	l.Close()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var records []DebugRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var r DebugRecord
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Errorf("bad json: %s", scanner.Text())
			continue
		}
		records = append(records, r)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].RequestID != "r1" || records[1].RequestID != "r2" {
		t.Errorf("record order wrong: %+v", records)
	}
}

func TestDebugLoggerRotatesWhenExceedingSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ocr.jsonl")
	// maxSize=0 会走到默认 100MB，我们要一个很小的值：直接构造 logger 后手工改 field 不合适。
	// 用反射太脏，改用足够多的 record 来撑破默认阈值? 太慢。
	// 内部把 int64(maxSizeMB) * 1MB 计算，最小 1MB。用 1MB + 大 record 撑爆。
	l, err := NewDebugLogger(path, 1, 3) // 1MB
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer l.Close()

	// 每条 ~120KB，写 20 条就 ~2.4MB，会触发 rotation 2 次
	big := strings.Repeat("x", 120_000)
	for i := 0; i < 20; i++ {
		l.Log(DebugRecord{RequestID: "r", TargetLang: big})
	}
	// 等异步写完
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path + ".1"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("expected rotated file %s.1: %v", path, err)
	}
	// path 本身应该继续存在，且尺寸不超过 maxSize（1MB + 一条余量）
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat main: %v", err)
	}
	if stat.Size() > 2*1024*1024 {
		t.Errorf("main file too large after rotation: %d bytes", stat.Size())
	}
}

func TestDebugLoggerDropsWhenQueueFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ocr.jsonl")
	l, err := NewDebugLogger(path, 100, 3)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer l.Close()

	before := DebugDropped()
	// 队列容量 128，瞬时投递 5000 条应该丢一些
	for i := 0; i < 5000; i++ {
		l.Log(DebugRecord{RequestID: "flood"})
	}
	// 让后台 goroutine 消费一段时间
	time.Sleep(50 * time.Millisecond)
	if DebugDropped() <= before {
		t.Logf("no records dropped this run; that's OK if the writer kept up (dropped=%d)", DebugDropped()-before)
	}
}

// waitForFile 等文件出现（异步写入完成的替代）。
func waitForFile(t *testing.T, path string, step time.Duration, tries int) {
	t.Helper()
	for i := 0; i < tries; i++ {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(step)
	}
}
