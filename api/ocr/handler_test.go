package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type fakeTranslator struct {
	fn                func(text string) (string, error)
	calls             int32
	conservativeCalls int32
}

func (f *fakeTranslator) Translate(ctx context.Context, provider, model, prompt, text, src, tgt string) (string, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		return f.fn(text)
	}
	return "ZH(" + text + ")", nil
}

// TranslateConservative 在测试里当作 policy=general 处理，但记录调用次数以区分。
// 若测试想验证具体是 general 还是 conservative 路径，可查看 conservativeCalls。
func (f *fakeTranslator) TranslateConservative(ctx context.Context, provider, model, prompt, text, src, tgt string) (string, error) {
	atomic.AddInt32(&f.conservativeCalls, 1)
	return f.Translate(ctx, provider, model, prompt, text, src, tgt)
}

func newTestHandler(t *testing.T, fn func(string) (string, error)) (*Handler, *fakeTranslator) {
	t.Helper()
	fk := &fakeTranslator{fn: fn}
	return newHandler(fk, HandlerConfig{DefaultPrompt: "Translate {{input}}"}), fk
}

func post(t *testing.T, h *Handler, body OCRRequest) OCRResponse {
	t.Helper()
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/ocr/translate", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp OCRResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (%s)", err, w.Body.String())
	}
	return resp
}

func mustFindByID(t *testing.T, resp OCRResponse, id string) OCRTranslation {
	t.Helper()
	for _, r := range resp.Translations {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no translation with id %q", id)
	return OCRTranslation{}
}

func TestSkipsHeaderFooterFigureEquation(t *testing.T) {
	h, fk := newTestHandler(t, nil)
	resp := post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements: []OCRElement{
			{ID: "h", Type: ElementHeader, Content: "Header text"},
			{ID: "f", Type: ElementFooter, Content: "Footer"},
			{ID: "fig", Type: ElementFigure, Content: ""},
			{ID: "eq", Type: ElementEquation, Content: "E=mc^2"},
		},
	})
	for _, id := range []string{"h", "f", "fig", "eq"} {
		tr := mustFindByID(t, resp, id)
		if tr.Translated {
			t.Errorf("%s should be skipped", id)
		}
		if !strings.HasSuffix(tr.Reason, "_skipped") {
			t.Errorf("%s reason = %q, want *_skipped", id, tr.Reason)
		}
	}
	if fk.calls != 0 {
		t.Errorf("upstream calls = %d, want 0", fk.calls)
	}
}

func TestSkipsContentAlreadyInTargetLang(t *testing.T) {
	h, fk := newTestHandler(t, nil)
	resp := post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements: []OCRElement{
			{ID: "cn", Type: ElementText, Content: "这一段本来就是中文，不用翻。"},
			{ID: "en", Type: ElementText, Content: "This one is English, should be translated."},
		},
	})
	cn := mustFindByID(t, resp, "cn")
	if cn.Translated {
		t.Errorf("chinese content should not be translated, got %+v", cn)
	}
	if cn.Reason != "already_in_target_lang" {
		t.Errorf("reason = %q, want already_in_target_lang", cn.Reason)
	}
	en := mustFindByID(t, resp, "en")
	if !en.Translated {
		t.Errorf("english content should be translated, got %+v", en)
	}
	if fk.calls != 1 {
		t.Errorf("upstream calls = %d, want 1", fk.calls)
	}
}

func TestReferenceTranslatesOnlyQuotedTitle(t *testing.T) {
	h, fk := newTestHandler(t, nil)
	resp := post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements: []OCRElement{
			{ID: "ieee", Type: ElementReference,
				Content: `[1] J. Smith, "A novel approach to translation," IEEE Trans. Comput., vol. 42, no. 3, pp. 123-145, 2024, doi: 10.1109/TC.2024.1234567.`},
			{ID: "notitle", Type: ElementReference,
				Content: `Smith J. IEEE Trans., 2024, doi: 10.x/y.`},
		},
	})

	ieee := mustFindByID(t, resp, "ieee")
	if !ieee.Translated {
		t.Fatalf("ieee should be translated, got %+v", ieee)
	}
	if !strings.Contains(ieee.Content, "ZH(A novel approach to translation,)") {
		t.Errorf("quoted title not translated: %q", ieee.Content)
	}
	if !strings.Contains(ieee.Content, "IEEE Trans. Comput.") || !strings.Contains(ieee.Content, "10.1109/TC.2024.1234567") {
		t.Errorf("journal name or DOI corrupted: %q", ieee.Content)
	}

	notitle := mustFindByID(t, resp, "notitle")
	if notitle.Translated {
		t.Errorf("reference without quoted title should not be translated: %+v", notitle)
	}
	if notitle.Reason != "reference_no_translatable_title" {
		t.Errorf("reason = %q, want reference_no_translatable_title", notitle.Reason)
	}

	if fk.calls != 1 {
		t.Errorf("upstream calls = %d, want 1 (only the ieee title)", fk.calls)
	}
}

func TestCaptionPreservesPrefixAndTranslatesRest(t *testing.T) {
	h, fk := newTestHandler(t, nil)
	// 英文 caption → 中文
	resp := post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements: []OCRElement{
			{ID: "en", Type: ElementFigureCaption, Content: "Figure 2. Sample distribution"},
			{ID: "prefix-only", Type: ElementFigureCaption, Content: "Figure 4."},
		},
	})

	en := mustFindByID(t, resp, "en")
	if !strings.HasPrefix(en.Content, "Figure 2.") {
		t.Errorf("prefix lost: %q", en.Content)
	}
	if !strings.Contains(en.Content, "ZH(Sample distribution)") {
		t.Errorf("rest not translated: %q", en.Content)
	}

	// 中文 caption → 英文（source language 需要真的与 target 不同才能触发翻译）
	resp2 := post(t, h, OCRRequest{
		TargetLang: "en",
		Elements: []OCRElement{
			{ID: "zh", Type: ElementTableCaption, Content: "表 3 实验结果"},
		},
	})
	zh := mustFindByID(t, resp2, "zh")
	if !strings.HasPrefix(zh.Content, "表 3") {
		t.Errorf("cn prefix lost: %q", zh.Content)
	}
	if !strings.Contains(zh.Content, "ZH(实验结果)") {
		t.Errorf("cn caption rest not translated: %q", zh.Content)
	}

	po := mustFindByID(t, resp, "prefix-only")
	if po.Translated {
		t.Error("prefix-only caption should not be translated")
	}
	if po.Reason != "caption_prefix_only" {
		t.Errorf("reason = %q, want caption_prefix_only", po.Reason)
	}
	if fk.calls != 2 {
		t.Errorf("upstream calls = %d, want 2", fk.calls)
	}
}

func TestTableHTMLUsesConservativePolicy(t *testing.T) {
	h, fk := newTestHandler(t, nil)
	post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements: []OCRElement{
			{ID: "t", Type: ElementTable, ContentFormat: FormatHTML,
				Content: `<table><tr><td>Alice</td><td>Bob</td></tr></table>`},
		},
	})
	if fk.conservativeCalls == 0 {
		t.Errorf("table cells should go through TranslateConservative, got %d conservative calls", fk.conservativeCalls)
	}
	if fk.calls-fk.conservativeCalls != 0 {
		t.Errorf("table cells leaked to general Translate: general=%d conservative=%d", fk.calls-fk.conservativeCalls, fk.conservativeCalls)
	}
}

func TestTableHTMLTranslatesEachCellAndKeepsStructure(t *testing.T) {
	h, _ := newTestHandler(t, nil)
	resp := post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements: []OCRElement{
			{ID: "t", Type: ElementTable, ContentFormat: FormatHTML,
				Content: `<table><thead><tr><th>Name</th><th>Age</th></tr></thead><tbody><tr><td colspan="2">Merged</td></tr><tr><td>Alice</td><td>30</td></tr></tbody></table>`},
		},
	})
	tr := mustFindByID(t, resp, "t")
	if !tr.Translated {
		t.Fatalf("expected translated=true, got %+v", tr)
	}
	if !strings.Contains(tr.Content, `colspan="2"`) {
		t.Errorf("colspan lost: %s", tr.Content)
	}
	if !strings.Contains(tr.Content, "ZH(Merged)") {
		t.Errorf("Merged cell not translated: %s", tr.Content)
	}
	if !strings.Contains(tr.Content, "ZH(Alice)") {
		t.Errorf("Alice cell not translated: %s", tr.Content)
	}
	// numeric cell "30" should not appear as ZH(30) — fakeTranslator wraps everything,
	// but the handler passes through raw ints. Here fakeTranslator is called for every
	// non-empty cell, so 30 will also be wrapped. That's fine for this test; the
	// production path relies on TranslationService's policy to skip numerics.
}

func TestTextMarkdownAndHTMLBranching(t *testing.T) {
	seen := make([]string, 0, 4)
	h, _ := newTestHandler(t, func(text string) (string, error) {
		seen = append(seen, text)
		return "ZH(" + text + ")", nil
	})
	resp := post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements: []OCRElement{
			{ID: "text", Type: ElementText, Content: "Plain sentence."},
			{ID: "md", Type: ElementText, ContentFormat: FormatMarkdown, Content: "# Head\n\nHello **world**"},
			{ID: "bad", Type: ElementText, ContentFormat: FormatHTML, Content: "<p>should be rejected</p>"},
		},
	})

	txt := mustFindByID(t, resp, "text")
	if !txt.Translated || !strings.HasPrefix(txt.Content, "ZH(") {
		t.Errorf("text not translated: %+v", txt)
	}
	md := mustFindByID(t, resp, "md")
	if !md.Translated {
		t.Errorf("markdown not translated: %+v", md)
	}
	// The fake translator sees the input augmented with markdownInputPrefix.
	foundMD := false
	for _, s := range seen {
		if strings.HasPrefix(s, markdownInputPrefix) {
			foundMD = true
		}
	}
	if !foundMD {
		t.Errorf("markdown input prefix not applied to upstream input, saw: %v", seen)
	}

	bad := mustFindByID(t, resp, "bad")
	if bad.Translated {
		t.Errorf("non-table html should not be translated: %+v", bad)
	}
	if bad.Reason != "html_only_supported_for_table" {
		t.Errorf("reason = %q, want html_only_supported_for_table", bad.Reason)
	}
}

func TestUnknownTypeFallsBackToText(t *testing.T) {
	h, _ := newTestHandler(t, nil)
	resp := post(t, h, OCRRequest{
		TargetLang: "zh",
		Elements:   []OCRElement{{ID: "u", Type: ElementType("unknown_from_pp"), Content: "Hello."}},
	})
	tr := mustFindByID(t, resp, "u")
	if !tr.Translated {
		t.Errorf("unknown type should fall back to text and translate: %+v", tr)
	}
}

func TestValidationErrors(t *testing.T) {
	h, _ := newTestHandler(t, nil)

	// missing target_lang
	req := httptest.NewRequest(http.MethodPost, "/ocr/translate", strings.NewReader(`{"elements":[{"id":"x","type":"text","content":"hi"}]}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing target_lang: status = %d, want 400", w.Code)
	}

	// empty elements
	req = httptest.NewRequest(http.MethodPost, "/ocr/translate", strings.NewReader(`{"target_lang":"zh","elements":[]}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty elements: status = %d, want 400", w.Code)
	}

	// wrong method
	req = httptest.NewRequest(http.MethodGet, "/ocr/translate", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: status = %d, want 405", w.Code)
	}
}
