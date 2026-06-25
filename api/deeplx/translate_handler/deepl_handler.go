package translate_handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

type DeepLTranslateRequest struct {
	Text                       []string `json:"text"`
	TargetLang                 string   `json:"target_lang"`
	SourceLang                 string   `json:"source_lang,omitempty"`
	Context                    string   `json:"context,omitempty"`
	ShowBilledCharacters       bool     `json:"show_billed_characters,omitempty"`
	SplitSentences             string   `json:"split_sentences,omitempty"`
	PreserveFormatting         bool     `json:"preserve_formatting,omitempty"`
	Formality                  string   `json:"formality,omitempty"`
	ModelType                  string   `json:"model_type,omitempty"`
	GlossaryID                 string   `json:"glossary_id,omitempty"`
	GlossaryIDs                []string `json:"glossary_ids,omitempty"`
	StyleID                    string   `json:"style_id,omitempty"`
	TranslationMemoryID        string   `json:"translation_memory_id,omitempty"`
	TranslationMemoryThreshold int      `json:"translation_memory_threshold,omitempty"`
	CustomInstructions         []string `json:"custom_instructions,omitempty"`
	TagHandling                string   `json:"tag_handling,omitempty"`
	TagHandlingVersion         string   `json:"tag_handling_version,omitempty"`
	OutlineDetection           *bool    `json:"outline_detection,omitempty"`
	EnableBetaLanguages        bool     `json:"enable_beta_languages,omitempty"`
	NonSplittingTags           []string `json:"non_splitting_tags,omitempty"`
	SplittingTags              []string `json:"splitting_tags,omitempty"`
	IgnoreTags                 []string `json:"ignore_tags,omitempty"`
}

type DeepLTranslation struct {
	DetectedSourceLanguage string `json:"detected_source_language,omitempty"`
	Text                   string `json:"text"`
	BilledCharacters       *int   `json:"billed_characters,omitempty"`
}

type DeepLTranslateResponse struct {
	Translations []DeepLTranslation `json:"translations"`
}

type deepLErrorResponse struct {
	Message string `json:"message"`
}

func (h *Handler) HandleDeepLTranslation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendDeepLError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.isAuthorized(r) {
		h.sendDeepLError(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	var req DeepLTranslateRequest
	if err := h.decodeDeepLRequest(w, r, &req); err != nil {
		h.sendDeepLError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateDeepLRequest(req); err != nil {
		h.sendDeepLError(w, err.Error(), http.StatusBadRequest)
		return
	}

	promptTemplate := h.deepLPromptTemplate(req)
	results := make([]DeepLTranslation, len(req.Text))
	maxConcurrent := h.maxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for i, text := range req.Text {
		wg.Add(1)
		go func(idx int, sourceText string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-r.Context().Done():
				setFirstErr(&errMu, &firstErr, r.Context().Err())
				return
			}
			defer func() { <-sem }()

			translated, err := h.translationService.Translate(r.Context(), "", "", promptTemplate, sourceText, req.SourceLang, req.TargetLang)
			if err != nil {
				setFirstErr(&errMu, &firstErr, err)
				return
			}

			item := DeepLTranslation{
				DetectedSourceLanguage: strings.ToUpper(req.SourceLang),
				Text:                   translated,
			}
			if req.ShowBilledCharacters {
				billed := utf8.RuneCountInString(sourceText)
				item.BilledCharacters = &billed
			}
			results[idx] = item
		}(i, text)
	}

	wg.Wait()
	if firstErr != nil {
		h.sendDeepLError(w, "Translation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DeepLTranslateResponse{Translations: results})
}

func (h *Handler) decodeDeepLRequest(w http.ResponseWriter, r *http.Request, req *DeepLTranslateRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") || strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseForm(); err != nil {
			return err
		}
		req.Text = r.Form["text"]
		req.TargetLang = r.Form.Get("target_lang")
		req.SourceLang = r.Form.Get("source_lang")
		req.Context = r.Form.Get("context")
		req.ShowBilledCharacters = parseBool(r.Form.Get("show_billed_characters"))
		req.SplitSentences = r.Form.Get("split_sentences")
		req.PreserveFormatting = parseBool(r.Form.Get("preserve_formatting"))
		req.Formality = r.Form.Get("formality")
		req.ModelType = r.Form.Get("model_type")
		req.GlossaryID = r.Form.Get("glossary_id")
		req.GlossaryIDs = r.Form["glossary_ids"]
		req.StyleID = r.Form.Get("style_id")
		req.TranslationMemoryID = r.Form.Get("translation_memory_id")
		if threshold := r.Form.Get("translation_memory_threshold"); threshold != "" {
			req.TranslationMemoryThreshold, _ = strconv.Atoi(threshold)
		}
		req.CustomInstructions = r.Form["custom_instructions"]
		req.TagHandling = r.Form.Get("tag_handling")
		req.TagHandlingVersion = r.Form.Get("tag_handling_version")
		if outline := r.Form.Get("outline_detection"); outline != "" {
			parsed := parseBool(outline)
			req.OutlineDetection = &parsed
		}
		req.EnableBetaLanguages = parseBool(r.Form.Get("enable_beta_languages"))
		req.NonSplittingTags = r.Form["non_splitting_tags"]
		req.SplittingTags = r.Form["splitting_tags"]
		req.IgnoreTags = r.Form["ignore_tags"]
		return nil
	}

	return json.NewDecoder(r.Body).Decode(req)
}

func validateDeepLRequest(req DeepLTranslateRequest) error {
	if len(req.Text) == 0 {
		return errors.New("text is required")
	}
	for _, text := range req.Text {
		if text == "" {
			return errors.New("text must not contain empty values")
		}
	}
	if req.TargetLang == "" {
		return errors.New("target_lang is required")
	}
	if len(req.GlossaryIDs) > 5 {
		return errors.New("glossary_ids must contain at most 5 values")
	}
	if len(req.CustomInstructions) > 10 {
		return errors.New("custom_instructions must contain at most 10 values")
	}
	for _, instruction := range req.CustomInstructions {
		if utf8.RuneCountInString(instruction) > 300 {
			return errors.New("custom_instructions values must be at most 300 characters")
		}
	}
	return nil
}

func (h *Handler) deepLPromptTemplate(req DeepLTranslateRequest) string {
	template := h.promptTemplate
	var extra []string
	if req.Context != "" {
		extra = append(extra, "Additional context. Use this to guide the translation, but do not translate it directly: "+req.Context)
	}
	if len(req.CustomInstructions) > 0 {
		extra = append(extra, "Custom translation instructions: "+strings.Join(req.CustomInstructions, " "))
	}
	if req.Formality != "" && req.Formality != "default" {
		extra = append(extra, "Formality preference: "+req.Formality+".")
	}
	if len(extra) == 0 {
		return template
	}
	return template + "\n\n" + strings.Join(extra, "\n")
}

func (h *Handler) isAuthorized(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	apiKey := ""
	if scheme, token, ok := strings.Cut(authHeader, " "); ok {
		switch strings.ToLower(scheme) {
		case "bearer", "deepl-auth-key":
			apiKey = token
		}
	}
	if apiKey == "" {
		apiKey = r.URL.Query().Get("token")
	}
	return h.authTokens[apiKey]
}

func (h *Handler) sendDeepLError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(deepLErrorResponse{Message: message})
}

func setFirstErr(mu *sync.Mutex, target *error, err error) {
	if err == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if *target == nil {
		*target = err
	}
}

func parseBool(value string) bool {
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
