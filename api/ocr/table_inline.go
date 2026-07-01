package ocr

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// 支持的内联标签集合。翻译时把它们变成 placeholder，翻完再还原。
// 块级标签（p, div, ul, li, table 等）不列入——它们要么在 cell 里罕见，要么被扁平化更安全。
var inlineTags = map[atom.Atom]bool{
	atom.B: true, atom.Strong: true,
	atom.I: true, atom.Em: true,
	atom.U:      true,
	atom.Sub:    true, atom.Sup: true,
	atom.Code:   true,
	atom.Span:   true,
	atom.Mark:   true,
	atom.Small:  true,
	atom.A:      true,
}

// inlineTagRecord 记录一个 tokenized 标签的 tag name 和 attrs，供 detokenize 时重建 element node。
type inlineTagRecord struct {
	Atom  atom.Atom
	Data  string
	Attrs []html.Attribute
}

const (
	placeholderOpen  = "⟪"       // ⟪
	placeholderClose = "⟫"       // ⟫
	closeMarkerRune  = '/'
)

func openMarker(idx int) string  { return placeholderOpen + strconv.Itoa(idx) + placeholderClose }
func closeMarker(idx int) string { return placeholderOpen + "/" + strconv.Itoa(idx) + placeholderClose }

// tokenizeCell 遍历 cell，将内联标签替换为 ⟪N⟫...⟪/N⟫ 形式的 placeholder，
// 记录每个 idx 对应的标签信息。返回 tokens 长度即使用的 placeholder 数。
// br 输出为 \n，非内联块级标签被"扁平化"（穿透）——只提取其内部文本节点。
func tokenizeCell(cell *html.Node) (string, []inlineTagRecord) {
	var b strings.Builder
	var records []inlineTagRecord
	var recurse func(*html.Node)
	recurse = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == html.TextNode:
				b.WriteString(c.Data)
			case c.Type == html.ElementNode && c.DataAtom == atom.Br:
				b.WriteString("\n")
			case c.Type == html.ElementNode && inlineTags[c.DataAtom]:
				records = append(records, inlineTagRecord{
					Atom:  c.DataAtom,
					Data:  c.Data,
					Attrs: append([]html.Attribute(nil), c.Attr...),
				})
				idx := len(records) // 1-based
				b.WriteString(openMarker(idx))
				recurse(c)
				b.WriteString(closeMarker(idx))
			case c.Type == html.ElementNode:
				// 块级或未识别标签：穿透进入子节点
				recurse(c)
			}
		}
	}
	recurse(cell)
	return normalizeCellTextPreservingPlaceholders(b.String()), records
}

// normalizeCellTextPreservingPlaceholders 折叠空白但不动 placeholder。
// 与 normalizeCellText 类似，先按 \n 拆行、每行折叠连续空白、丢弃全空白行；
// placeholder 内部不含空格，本身就不会被折叠破坏。
func normalizeCellTextPreservingPlaceholders(s string) string {
	return normalizeCellText(s)
}

// detokenizeInto 清空 cell 的子节点，按 translated 里的 placeholder 序列重建 DOM。
// 未闭合或找不到对应 record 的 placeholder 会被忽略（只输出其中的文本内容）。
func detokenizeInto(cell *html.Node, translated string, records []inlineTagRecord) {
	for c := cell.FirstChild; c != nil; {
		next := c.NextSibling
		cell.RemoveChild(c)
		c = next
	}

	tokens := lexPlaceholders(translated, len(records))

	// 用栈重建：每个入栈项是"当前正在往里 append 的 parent 节点"。
	stack := []*html.Node{cell}
	top := func() *html.Node { return stack[len(stack)-1] }
	appendText := func(s string) {
		if s == "" {
			return
		}
		// 保留 \n 作为 <br>
		parts := strings.Split(s, "\n")
		for i, p := range parts {
			if i > 0 {
				top().AppendChild(&html.Node{Type: html.ElementNode, Data: "br", DataAtom: atom.Br})
			}
			if p != "" {
				top().AppendChild(&html.Node{Type: html.TextNode, Data: p})
			}
		}
	}

	// 用一个 map 追踪每个 record idx 已经打开但还没关闭的对应节点在 stack 中的位置。
	openIdx := map[int]int{} // idx -> stack index

	for _, tk := range tokens {
		switch tk.kind {
		case tokText:
			appendText(tk.text)
		case tokOpen:
			rec := records[tk.idx-1]
			el := &html.Node{Type: html.ElementNode, DataAtom: rec.Atom, Data: rec.Data, Attr: append([]html.Attribute(nil), rec.Attrs...)}
			top().AppendChild(el)
			stack = append(stack, el)
			openIdx[tk.idx] = len(stack) - 1
		case tokClose:
			pos, ok := openIdx[tk.idx]
			if !ok {
				// 对应的 open 缺失，忽略这个 close
				continue
			}
			// 关闭 pos 及之后所有未闭合的元素（就近关闭）
			// 严格 XML 会要求正确嵌套；如果模型改变了嵌套顺序，我们容错关闭到当前 idx
			for i := len(stack) - 1; i >= pos; i-- {
				delete(openIdx, indexOfRecordInStack(stack, i, openIdx))
			}
			stack = stack[:pos]
		}
	}
	// 结束时如仍有未闭合，栈会自动"关闭到根"——所有元素已经作为子节点挂在 cell 上，直接丢弃。
}

// indexOfRecordInStack 从 openIdx 反查栈位置对应的 idx（用于 close 时批量清理）。
// 找不到返回 0（不会误删有效项）。
func indexOfRecordInStack(_ []*html.Node, stackPos int, openIdx map[int]int) int {
	for idx, pos := range openIdx {
		if pos == stackPos {
			return idx
		}
	}
	return 0
}

type placeholderTokenKind int

const (
	tokText placeholderTokenKind = iota
	tokOpen
	tokClose
)

type placeholderToken struct {
	kind placeholderTokenKind
	idx  int    // for open/close
	text string // for text
}

// lexPlaceholders 扫描 translated，输出 text / open / close 三种 token 序列。
// maxIdx 是有效 placeholder 编号上限（超出的当作普通文本）。
func lexPlaceholders(translated string, maxIdx int) []placeholderToken {
	var out []placeholderToken
	i := 0
	textStart := 0
	flushText := func(end int) {
		if end > textStart {
			out = append(out, placeholderToken{kind: tokText, text: translated[textStart:end]})
		}
		textStart = end
	}
	for i < len(translated) {
		if !strings.HasPrefix(translated[i:], placeholderOpen) {
			i++
			continue
		}
		// 匹配 ⟪ N ⟫ 或 ⟪ / N ⟫
		endClose := strings.Index(translated[i:], placeholderClose)
		if endClose < 0 {
			i++
			continue
		}
		body := translated[i+len(placeholderOpen) : i+endClose]
		closeKind := false
		if strings.HasPrefix(body, string(closeMarkerRune)) {
			closeKind = true
			body = body[1:]
		}
		idx, err := strconv.Atoi(body)
		if err != nil || idx < 1 || idx > maxIdx {
			i++
			continue
		}
		flushText(i)
		full := i + endClose + len(placeholderClose)
		if closeKind {
			out = append(out, placeholderToken{kind: tokClose, idx: idx})
		} else {
			out = append(out, placeholderToken{kind: tokOpen, idx: idx})
		}
		i = full
		textStart = i
	}
	flushText(len(translated))
	return out
}

