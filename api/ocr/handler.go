package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"transbridge/service"
)

// TranslatorFn 是 handler 依赖的最小翻译接口，方便单元测试注入 fake。
// service.TranslationService 天然满足。
type TranslatorFn interface {
	Translate(ctx context.Context, provider, model, promptTemplate, text, sourceLang, targetLang string) (string, error)
}

// Handler 处理 /ocr/translate 请求。逐元素按类型 + content_format 分发：
//
//   type=header|footer|figure|equation                → skip
//   type=reference                                    → skip
//   type=figure_caption|table_caption                 → 保留前置编号（"Figure 2." / "表 3"），只翻后半
//   type=table && content_format=html                 → 拆 <td>/<th>，逐格走 policy → 翻译 → 回填
//   type=table && content_format=markdown             → 透传 markdown 内容，prompt 追加保留 markdown 语法
//   type=table && content_format=text                 → 视作普通文本
//   其他 type                                          → 按 content_format 处理
//
// 未知类型退化到 text。参考 docs/TRANSLATION.md 的 policy / cache / quality gate 决策。
type Handler struct {
	translationService TranslatorFn
	authValidator      func(*http.Request, string) bool
	promptProvider     func(*http.Request) string
	defaultPrompt      string
	maxConcurrent      int
}

// HandlerConfig 构造参数
type HandlerConfig struct {
	AuthValidator  func(*http.Request, string) bool
	PromptProvider func(*http.Request) string
	DefaultPrompt  string
	MaxConcurrent  int
}

func NewHandler(svc *service.TranslationService, cfg HandlerConfig) *Handler {
	return newHandler(svc, cfg)
}

func newHandler(svc TranslatorFn, cfg HandlerConfig) *Handler {
	max := cfg.MaxConcurrent
	if max <= 0 {
		max = 5
	}
	return &Handler{
		translationService: svc,
		authValidator:      cfg.AuthValidator,
		promptProvider:     cfg.PromptProvider,
		defaultPrompt:      cfg.DefaultPrompt,
		maxConcurrent:      max,
	}
}

// figure/table 前置编号：Figure 2. / Fig. 3 / Table 4: / 图 2. / 表 3、
var captionPrefixPattern = regexp.MustCompile(`^\s*((?:Figure|Fig\.?|Table)\s*\d+(?:\.\d+)?[.:：]?\s*|(?:图|表)\s*\d+(?:\.\d+)?[、.:：]?\s*)`)

// markdownInputPrefix 加在 text 前，让模型知道输入是 markdown 并保留语法。
// 用 input 前缀而不是 prompt suffix，是因为大部分 prompt 模板会把 {{input}} 放在末尾，
// 追加在 prompt 后面时模型可能把提示词误当作待翻译内容。
const markdownInputPrefix = "The following content is in markdown. Preserve all markdown syntax (headings, lists, tables, emphasis, links, code fences) and translate only the natural-language text.\n\n"

// ServeHTTP 主入口
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.isAuthorized(r) {
		h.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req OCRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TargetLang == "" {
		h.sendError(w, "target_lang is required", http.StatusBadRequest)
		return
	}
	if len(req.Elements) == 0 {
		h.sendError(w, "elements is required", http.StatusBadRequest)
		return
	}
	if len(req.Elements) > 500 {
		h.sendError(w, "too many elements (max 500)", http.StatusBadRequest)
		return
	}

	promptTemplate := h.currentPromptTemplate(r)

	sem := make(chan struct{}, h.maxConcurrent)
	results := make([]OCRTranslation, len(req.Elements))
	var wg sync.WaitGroup

	for i, el := range req.Elements {
		wg.Add(1)
		go func(idx int, e OCRElement) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-r.Context().Done():
				results[idx] = OCRTranslation{ID: e.ID, Type: e.Type, ContentFormat: e.ContentFormat, Content: e.Content, Reason: r.Context().Err().Error()}
				return
			}
			defer func() { <-sem }()
			results[idx] = h.processElement(r.Context(), e, promptTemplate, req.SourceLang, req.TargetLang)
		}(i, el)
	}
	wg.Wait()

	h.sendJSON(w, OCRResponse{Code: 200, Translations: results})
}

func (h *Handler) processElement(ctx context.Context, e OCRElement, promptTemplate, srcLang, tgtLang string) OCRTranslation {
	et := e.Type
	if et == "" {
		et = ElementText
	}
	base := OCRTranslation{ID: e.ID, Type: et, ContentFormat: e.ContentFormat, Content: e.Content}

	// 直接 skip 的类型：页眉/页脚/图/公式/参考文献条目
	switch et {
	case ElementHeader, ElementFooter, ElementFigure, ElementEquation, ElementReference:
		base.Reason = fmt.Sprintf("%s_skipped", et)
		return base
	}

	// caption：保留 "Figure 2." / "表 3" 前置编号
	if et == ElementTableCaption || et == ElementFigureCaption {
		return h.translateCaption(ctx, base, e, promptTemplate, srcLang, tgtLang)
	}

	// table + html：走单元格粒度翻译
	if et == ElementTable && e.ContentFormat == FormatHTML {
		return h.translateTableHTML(ctx, base, e, promptTemplate, srcLang, tgtLang)
	}

	// 其余：按 content_format 处理
	switch e.ContentFormat {
	case FormatHTML:
		// 第一版不支持非表格 HTML
		base.Reason = "html_only_supported_for_table"
		return base
	case FormatMarkdown:
		return h.translateMarkdown(ctx, base, e.Content, promptTemplate, srcLang, tgtLang)
	default: // FormatText 或空
		return h.translateWithSuffix(ctx, base, e.Content, promptTemplate, srcLang, tgtLang)
	}
}

func (h *Handler) translateMarkdown(ctx context.Context, base OCRTranslation, text, promptTemplate, srcLang, tgtLang string) OCRTranslation {
	if strings.TrimSpace(text) == "" {
		base.Reason = "blank"
		return base
	}
	augmented := markdownInputPrefix + text
	translated, err := h.translationService.Translate(ctx, "", "", promptTemplate, augmented, srcLang, tgtLang)
	if err != nil {
		base.Error = err.Error()
		return base
	}
	// 剥掉可能被模型回显的前缀
	translated = strings.TrimPrefix(translated, markdownInputPrefix)
	if translated == text || translated == augmented {
		base.Reason = "policy_kept_original"
		return base
	}
	base.Content = translated
	base.Translated = true
	return base
}

func (h *Handler) translateCaption(ctx context.Context, base OCRTranslation, e OCRElement, promptTemplate, srcLang, tgtLang string) OCRTranslation {
	text := e.Content
	if e.ContentFormat != FormatText && e.ContentFormat != "" {
		// caption 只处理纯文本；其它格式的 caption 极少见，直接透传
		return h.translateWithSuffix(ctx, base, text, promptTemplate, srcLang, tgtLang)
	}
	prefix := captionPrefixPattern.FindString(text)
	rest := strings.TrimPrefix(text, prefix)
	if strings.TrimSpace(rest) == "" {
		base.Reason = "caption_prefix_only"
		return base
	}
	translated, err := h.translationService.Translate(ctx, "", "", promptTemplate, rest, srcLang, tgtLang)
	if err != nil {
		base.Error = err.Error()
		return base
	}
	base.Content = prefix + translated
	base.Translated = true
	return base
}

func (h *Handler) translateWithSuffix(ctx context.Context, base OCRTranslation, text, promptTemplate, srcLang, tgtLang string) OCRTranslation {
	if strings.TrimSpace(text) == "" {
		base.Reason = "blank"
		return base
	}
	translated, err := h.translationService.Translate(ctx, "", "", promptTemplate, text, srcLang, tgtLang)
	if err != nil {
		base.Error = err.Error()
		return base
	}
	if translated == text {
		base.Reason = "policy_kept_original"
		return base
	}
	base.Content = translated
	base.Translated = true
	return base
}

func (h *Handler) translateTableHTML(ctx context.Context, base OCRTranslation, e OCRElement, promptTemplate, srcLang, tgtLang string) OCRTranslation {
	texts, finalize, err := PrepareTable(e.Content)
	if err != nil {
		base.Error = "parse table: " + err.Error()
		return base
	}
	if len(texts) == 0 {
		base.Reason = "no_cells"
		return base
	}

	translated := make([]string, len(texts))
	sem := make(chan struct{}, h.maxConcurrent)
	var wg sync.WaitGroup
	anyTranslated := false
	var anyErrMu sync.Mutex
	var anyErr string

	for i, cellText := range texts {
		if strings.TrimSpace(cellText) == "" {
			translated[i] = ""
			continue
		}
		wg.Add(1)
		go func(idx int, srcText string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			out, err := h.translationService.Translate(ctx, "", "", promptTemplate, srcText, srcLang, tgtLang)
			if err != nil {
				anyErrMu.Lock()
				if anyErr == "" {
					anyErr = err.Error()
				}
				anyErrMu.Unlock()
				translated[idx] = ""
				return
			}
			if out == srcText {
				translated[idx] = ""
				return
			}
			translated[idx] = out
			anyErrMu.Lock()
			anyTranslated = true
			anyErrMu.Unlock()
		}(i, cellText)
	}
	wg.Wait()

	rendered, err := finalize(translated)
	if err != nil {
		base.Error = "finalize table: " + err.Error()
		return base
	}
	base.Content = rendered
	base.Translated = anyTranslated
	if !anyTranslated {
		if anyErr != "" {
			base.Error = anyErr
		} else {
			base.Reason = "all_cells_kept_original"
		}
	}
	return base
}

func (h *Handler) currentPromptTemplate(r *http.Request) string {
	if h.promptProvider != nil {
		if t := h.promptProvider(r); t != "" {
			return t
		}
	}
	return h.defaultPrompt
}

func (h *Handler) isAuthorized(r *http.Request) bool {
	if h.authValidator != nil {
		return h.authValidator(r, "translate")
	}
	return true
}

func (h *Handler) sendError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": status, "error": msg})
}

func (h *Handler) sendJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
