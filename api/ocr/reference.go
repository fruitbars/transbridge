package ocr

import "strings"

// quotedSegment 是原文中一个引号包裹的可翻译片段。
// Start/End 是 outer 位置（含引号本身），Content 是引号内的裸文本。
type quotedSegment struct {
	Start   int    // 引号起始位置（字节索引，含开引号）
	End     int    // 引号结束位置（字节索引，指向闭引号后一位）
	Open    string // 开引号（可能是多字节，如 “ 或 «）
	Close   string // 闭引号
	Content string // 引号内裸文本
}

// 支持的引号对：ASCII 双引号、Unicode 弯双引号、法文 guillemet。
// 故意不加 ASCII 单引号（会误吃缩写 don't）；也不加中文单引号（同样存在歧义）。
var quoteRunes = []struct {
	open, close string
}{
	{`"`, `"`},
	{"“", "”"}, // “ ”
	{"«", "»"}, // « »
	{"「", "」"}, // 「 」
	{"『", "』"}, // 『 』
}

// extractQuotedSegments 顺序扫描 text，找到所有引号包裹段。
// 只匹配同类引号（"..." 或 "..." 或 «...»），不做嵌套；遇到未闭合的引号忽略。
// 空内容（"") 不算段。
func extractQuotedSegments(text string) []quotedSegment {
	var out []quotedSegment
	i := 0
	for i < len(text) {
		matched := false
		for _, q := range quoteRunes {
			if !strings.HasPrefix(text[i:], q.open) {
				continue
			}
			// 找同类闭引号
			rest := text[i+len(q.open):]
			end := strings.Index(rest, q.close)
			if end < 0 {
				continue
			}
			content := rest[:end]
			if strings.TrimSpace(content) == "" {
				i += len(q.open)
				matched = true
				break
			}
			seg := quotedSegment{
				Start:   i,
				End:     i + len(q.open) + end + len(q.close),
				Open:    q.open,
				Close:   q.close,
				Content: content,
			}
			out = append(out, seg)
			i = seg.End
			matched = true
			break
		}
		if !matched {
			i++
		}
	}
	return out
}

// applyQuotedReplacements 把 segments 按新的 Content 拼回原字符串。segments 顺序必须与
// extractQuotedSegments 返回一致（按 Start 递增），replacements 长度必须等长。
// replacement 为空或等于原 Content 则保留原文。
func applyQuotedReplacements(text string, segments []quotedSegment, replacements []string) string {
	if len(segments) == 0 {
		return text
	}
	var b strings.Builder
	cursor := 0
	for i, seg := range segments {
		b.WriteString(text[cursor:seg.Start])
		content := seg.Content
		if i < len(replacements) && replacements[i] != "" && replacements[i] != seg.Content {
			content = replacements[i]
		}
		b.WriteString(seg.Open)
		b.WriteString(content)
		b.WriteString(seg.Close)
		cursor = seg.End
	}
	b.WriteString(text[cursor:])
	return b.String()
}
