package ocr

import (
	"strings"
	"testing"
)

func TestPrepareTableExtractsAllCellsInOrder(t *testing.T) {
	src := `<table><thead><tr><th>Name</th><th>Age</th></tr></thead><tbody><tr><td>Alice</td><td>30</td></tr><tr><td>Bob</td><td>25</td></tr></tbody></table>`
	texts, _, err := PrepareTable(src)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	want := []string{"Name", "Age", "Alice", "30", "Bob", "25"}
	if len(texts) != len(want) {
		t.Fatalf("got %d cells, want %d: %v", len(texts), len(want), texts)
	}
	for i, w := range want {
		if texts[i] != w {
			t.Errorf("cell[%d] = %q, want %q", i, texts[i], w)
		}
	}
}

func TestPrepareTablePreservesColspanRowspan(t *testing.T) {
	src := `<table><tr><td colspan="2">Merged</td></tr><tr><td rowspan="2">Left</td><td>A</td></tr><tr><td>B</td></tr></table>`
	texts, finalize, err := PrepareTable(src)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	translated := []string{"合并", "左", "甲", "乙"}
	if len(texts) != len(translated) {
		t.Fatalf("got %d cells (%v), want 4", len(texts), texts)
	}
	out, err := finalize(translated)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !strings.Contains(out, `colspan="2"`) {
		t.Errorf("colspan lost: %s", out)
	}
	if !strings.Contains(out, `rowspan="2"`) {
		t.Errorf("rowspan lost: %s", out)
	}
	if !strings.Contains(out, "合并") || !strings.Contains(out, "乙") {
		t.Errorf("translations not applied: %s", out)
	}
}

func TestPrepareTableEmptyStringKeepsOriginal(t *testing.T) {
	src := `<table><tr><td>Keep me</td><td>Translate me</td></tr></table>`
	texts, finalize, err := PrepareTable(src)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if texts[0] != "Keep me" || texts[1] != "Translate me" {
		t.Fatalf("unexpected texts: %v", texts)
	}
	out, err := finalize([]string{"", "翻译我"})
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !strings.Contains(out, "Keep me") {
		t.Errorf("original text lost when translation was empty: %s", out)
	}
	if !strings.Contains(out, "翻译我") {
		t.Errorf("translation not written: %s", out)
	}
}

func TestPrepareTableHandlesBrAsNewline(t *testing.T) {
	src := `<table><tr><td>Line 1<br>Line 2</td></tr></table>`
	texts, finalize, err := PrepareTable(src)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if texts[0] != "Line 1\nLine 2" {
		t.Errorf("br not normalized to \\n, got %q", texts[0])
	}
	out, err := finalize([]string{"第 1 行\n第 2 行"})
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !strings.Contains(out, "第 1 行") || !strings.Contains(out, "第 2 行") {
		t.Errorf("multiline translation missing: %s", out)
	}
	if !strings.Contains(out, "<br") {
		t.Errorf("<br> not restored on write-back: %s", out)
	}
}

func TestPrepareTableTrimsInnerWhitespace(t *testing.T) {
	src := "<table><tr><td>  hello   world  </td></tr></table>"
	texts, _, err := PrepareTable(src)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if texts[0] != "hello world" {
		t.Errorf("whitespace not collapsed, got %q", texts[0])
	}
}

func TestPrepareTableEmptyCellReturnsEmpty(t *testing.T) {
	src := `<table><tr><td></td><td>&nbsp;</td><td>content</td></tr></table>`
	texts, _, err := PrepareTable(src)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if len(texts) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(texts))
	}
	if texts[0] != "" {
		t.Errorf("empty td should yield empty text, got %q", texts[0])
	}
	if texts[2] != "content" {
		t.Errorf("cell[2] = %q, want content", texts[2])
	}
}

func TestPrepareTableFinalizeRejectsLengthMismatch(t *testing.T) {
	src := `<table><tr><td>a</td><td>b</td></tr></table>`
	_, finalize, err := PrepareTable(src)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := finalize([]string{"only one"}); err == nil {
		t.Fatal("expected finalize to error on length mismatch")
	}
}

func TestCountCellsMatchesPrepareTable(t *testing.T) {
	src := `<table><tr><th>h1</th><th>h2</th></tr><tr><td>a</td><td>b</td></tr></table>`
	n, err := CountCells(src)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 4 {
		t.Errorf("count = %d, want 4", n)
	}
}
