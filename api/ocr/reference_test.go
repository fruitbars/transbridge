package ocr

import "testing"

func TestExtractQuotedSegmentsIEEEReference(t *testing.T) {
	src := `[1] J. Smith and A. Doe, "A novel approach to translation," IEEE Trans. Comput., vol. 42, no. 3, pp. 123-145, 2024, doi: 10.1109/TC.2024.1234567.`
	segs := extractQuotedSegments(src)
	if len(segs) != 1 {
		t.Fatalf("segments = %d, want 1: %+v", len(segs), segs)
	}
	if segs[0].Content != "A novel approach to translation," {
		t.Errorf("content = %q, want the title inside quotes", segs[0].Content)
	}
	if src[segs[0].Start:segs[0].End] != `"A novel approach to translation,"` {
		t.Errorf("span = %q, not aligned with quotes", src[segs[0].Start:segs[0].End])
	}
}

func TestExtractQuotedSegmentsChineseReference(t *testing.T) {
	src := "[1] 张三, 李四. “一种新颖的翻译方法”[J]. 计算机学报, 2024, 42(3): 123-145."
	segs := extractQuotedSegments(src)
	if len(segs) != 1 {
		t.Fatalf("segments = %d, want 1", len(segs))
	}
	if segs[0].Content != "一种新颖的翻译方法" {
		t.Errorf("content = %q", segs[0].Content)
	}
}

func TestExtractQuotedSegmentsMultipleQuotes(t *testing.T) {
	src := `Smith, "Title A," and "Title B" appear together, doi: 10.x/y.`
	segs := extractQuotedSegments(src)
	if len(segs) != 2 {
		t.Fatalf("segments = %d, want 2", len(segs))
	}
	if segs[0].Content != "Title A," || segs[1].Content != "Title B" {
		t.Errorf("segments = %+v", segs)
	}
}

func TestExtractQuotedSegmentsUnclosedIsIgnored(t *testing.T) {
	src := `Smith, "unterminated title, IEEE Trans., 2024.`
	segs := extractQuotedSegments(src)
	if len(segs) != 0 {
		t.Errorf("unterminated quote should yield no segments, got %+v", segs)
	}
}

func TestExtractQuotedSegmentsIgnoresApostrophes(t *testing.T) {
	src := `Smith's paper on "Title" does not include don't as a title.`
	segs := extractQuotedSegments(src)
	if len(segs) != 1 {
		t.Fatalf("expected 1 quoted title, got %d: %+v", len(segs), segs)
	}
	if segs[0].Content != "Title" {
		t.Errorf("content = %q", segs[0].Content)
	}
}

func TestApplyQuotedReplacementsRebuildsString(t *testing.T) {
	src := `Smith, "Title A," and "Title B" together.`
	segs := extractQuotedSegments(src)
	out := applyQuotedReplacements(src, segs, []string{"标题甲，", "标题乙"})
	want := `Smith, "标题甲，" and "标题乙" together.`
	if out != want {
		t.Errorf("out = %q\nwant %q", out, want)
	}
}

func TestApplyQuotedReplacementsKeepsOriginalOnEmpty(t *testing.T) {
	src := `Smith, "Title A," and "Title B" together.`
	segs := extractQuotedSegments(src)
	out := applyQuotedReplacements(src, segs, []string{"", "TB"})
	want := `Smith, "Title A," and "TB" together.`
	if out != want {
		t.Errorf("out = %q\nwant %q", out, want)
	}
}
