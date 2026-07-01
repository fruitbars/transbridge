package ocr

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// PrepareTable 解析 HTML 表格片段，抽取每个 <td>/<th> 的可翻译文本，返回文本列表和一个 finalize 闭包。
//
// 调用方拿到 texts 后按顺序生成 translated（长度必须一致），传给 finalize 得到回填后的 HTML。
// translated[i] == "" 或与 texts[i] 相等表示保留原文，其它值会替换单元格内容。
//
// 保留：table 的整体结构（thead/tbody/tfoot/tr）、colspan/rowspan 等属性、非 td 的兄弟节点。
// 不保留：单元格里的内联标签（<b>/<i>/<a> 等）——第一版丢样式，只保留文本 + <br> 换行。
func PrepareTable(htmlSource string) ([]string, func([]string) (string, error), error) {
	root, err := parseHTMLFragment(htmlSource)
	if err != nil {
		return nil, nil, err
	}

	var cells []*html.Node
	var texts []string
	var perCellRecords [][]inlineTagRecord
	walk(root, func(n *html.Node) {
		if n.Type == html.ElementNode && (n.DataAtom == atom.Td || n.DataAtom == atom.Th) {
			cells = append(cells, n)
			text, records := tokenizeCell(n)
			texts = append(texts, text)
			perCellRecords = append(perCellRecords, records)
		}
	})

	finalize := func(translated []string) (string, error) {
		if len(translated) != len(cells) {
			return "", fmt.Errorf("translated length %d != cell count %d", len(translated), len(cells))
		}
		for i, cell := range cells {
			t := translated[i]
			if t == "" || t == texts[i] {
				continue
			}
			detokenizeInto(cell, t, perCellRecords[i])
		}
		return renderHTMLFragment(root)
	}

	return texts, finalize, nil
}

// CountCells 只统计表格里有多少个 td/th，供上层估算并发和批量。
func CountCells(htmlSource string) (int, error) {
	root, err := parseHTMLFragment(htmlSource)
	if err != nil {
		return 0, err
	}
	count := 0
	walk(root, func(n *html.Node) {
		if n.Type == html.ElementNode && (n.DataAtom == atom.Td || n.DataAtom == atom.Th) {
			count++
		}
	})
	return count, nil
}

// parseHTMLFragment 用 body 作为 context node 解析 table 片段。
// html.ParseFragment 会根据 context 决定 tag 的合法父子关系；body 能让顶级 <table> 保持结构。
func parseHTMLFragment(src string) (*html.Node, error) {
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(src), ctx)
	if err != nil {
		return nil, err
	}
	root := &html.Node{Type: html.DocumentNode}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	return root, nil
}

func renderHTMLFragment(root *html.Node) (string, error) {
	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

// extractCellText 把 td 内所有文本节点拼起来，<br> 变换行，多空白折叠为单空格，
// 每行两端 trim。空 cell 返回 ""。
func extractCellText(cell *html.Node) string {
	var b strings.Builder
	walk(cell, func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type == html.ElementNode && n.DataAtom == atom.Br:
			b.WriteString("\n")
		}
	})
	return normalizeCellText(b.String())
}

func normalizeCellText(s string) string {
	lines := strings.Split(s, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		if collapsed := strings.Join(strings.Fields(line), " "); collapsed != "" {
			trimmed = append(trimmed, collapsed)
		}
	}
	return strings.Join(trimmed, "\n")
}

// replaceCellContent 清空 td 的现有子节点，用 text 重建：换行 → <br>，其余作为文本节点。
func replaceCellContent(cell *html.Node, text string) {
	for c := cell.FirstChild; c != nil; {
		next := c.NextSibling
		cell.RemoveChild(c)
		c = next
	}
	parts := strings.Split(text, "\n")
	for i, part := range parts {
		if i > 0 {
			cell.AppendChild(&html.Node{Type: html.ElementNode, Data: "br", DataAtom: atom.Br})
		}
		if part != "" {
			cell.AppendChild(&html.Node{Type: html.TextNode, Data: part})
		}
	}
}
