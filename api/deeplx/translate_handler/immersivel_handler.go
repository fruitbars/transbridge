package translate_handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

type BatchTranslateRequest struct {
	SourceLang string   `json:"source_lang"`
	TargetLang string   `json:"target_lang"`
	TextList   []string `json:"text_list"`
}

type BatchTranslateItem struct {
	Index              int    `json:"index"`
	DetectedSourceLang string `json:"detected_source_lang"`
	Text               string `json:"text"`
	Error              string `json:"error,omitempty"`
}

type BatchTranslateResponse struct {
	Code         int                   `json:"code"`
	Translations []*BatchTranslateItem `json:"translations"` // 注意是指针数组
}

func (h *Handler) HandleImmersiveLTranslation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 API Key
	authHeader := r.Header.Get("Authorization")
	apiKey := strings.TrimPrefix(authHeader, "Bearer ")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("token")
	}
	if !h.isTokenAuthorized(r, apiKey, "translate") {
		h.sendError(w, "Invalid API key", "unauthorized", http.StatusUnauthorized)
		return
	}

	// 解析请求体
	var req BatchTranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", "invalid_request", http.StatusBadRequest)
		return
	}

	// 参数校验
	if len(req.TextList) == 0 || req.TargetLang == "" {
		h.sendError(w, "source_lang, target_lang and text_list are required", "invalid_request", http.StatusBadRequest)
		return
	}
	if len(req.TextList) > 50 {
		h.sendError(w, "Too many texts: maximum allowed is 50", "too_many_texts", http.StatusBadRequest)
		return
	}

	type result struct {
		index int
		item  *BatchTranslateItem
	}

	maxConcurrent := h.maxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	sem := make(chan struct{}, maxConcurrent)
	resultChan := make(chan result, len(req.TextList))
	results := make([]*BatchTranslateItem, len(req.TextList))

	var wg sync.WaitGroup
	for i, text := range req.TextList {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-r.Context().Done():
				resultChan <- result{
					index: idx,
					item: &BatchTranslateItem{
						Index:              idx,
						DetectedSourceLang: req.SourceLang,
						Text:               "",
						Error:              r.Context().Err().Error(),
					},
				}
				return
			}
			defer func() { <-sem }()

			translated, err := h.translationService.Translate(r.Context(), "", "", h.currentPromptTemplate(r), t, req.SourceLang, req.TargetLang)
			if err != nil {
				resultChan <- result{
					index: idx,
					item: &BatchTranslateItem{
						Index:              idx,
						DetectedSourceLang: req.SourceLang,
						Text:               "",
						Error:              err.Error(),
					},
				}
				return
			}

			resultChan <- result{
				index: idx,
				item: &BatchTranslateItem{
					Index:              idx,
					DetectedSourceLang: req.SourceLang,
					Text:               translated,
				},
			}
		}(i, text)
	}

	wg.Wait()
	close(resultChan)

	for res := range resultChan {
		results[res.index] = res.item
	}

	resp := BatchTranslateResponse{
		Code:         200,
		Translations: results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
