package translate_handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"transbridge/service"
)

type Handler struct {
	translationService *service.TranslationService
	authTokens         map[string]bool // 存储有效的 API 密钥
	promptTemplate     string          // 👈 新增
	maxConcurrent      int             // 批量接口最大并发
	authValidator      func(*http.Request, string) bool
	promptProvider     func(*http.Request) string
}

type HandlerConfig struct {
	AuthTokens     []string // 配置中的 API 密钥列表
	PromptTemplate string
	MaxConcurrent  int // 批量接口最大并发（可选）
	AuthValidator  func(*http.Request, string) bool
	PromptProvider func(*http.Request) string
}

func NewHandler(translationService *service.TranslationService, config HandlerConfig) *Handler {
	// 将 API 密钥列表转换为 map 以便快速查找
	authTokens := make(map[string]bool)
	for _, token := range config.AuthTokens {
		authTokens[token] = true
	}

	return &Handler{
		translationService: translationService,
		authTokens:         authTokens,
		promptTemplate:     config.PromptTemplate, // 👈 设置进去
		maxConcurrent:      config.MaxConcurrent,
		authValidator:      config.AuthValidator,
		promptProvider:     config.PromptProvider,
	}
}

func (h *Handler) HandleTranslation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 API 密钥
	authHeader := r.Header.Get("Authorization")
	apiKey := strings.TrimPrefix(authHeader, "Bearer ")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("token") // 支持 URL 参数方式传递 API 密钥
	}

	if !h.isTokenAuthorized(r, apiKey, "translate") {
		h.sendError(w, "Invalid API key", "unauthorized", http.StatusUnauthorized)
		return
	}

	var req TranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", "invalid_request", http.StatusBadRequest)
		return
	}

	// 参数校验
	if err := h.validateRequest(&req); err != nil {
		h.sendError(w, err.Error(), "invalid_request", http.StatusBadRequest)
		return
	}

	// 使用翻译服务处理请求
	translation, err := h.translationService.Translate(r.Context(), "", "", h.currentPromptTemplate(r), req.Text, req.SourceLang, req.TargetLang)
	if err != nil {
		h.sendError(w, "Translation failed", "translation_failed", http.StatusInternalServerError)
		return
	}

	// 发送响应
	h.sendResponse(w, translation, req.SourceLang, req.TargetLang)
}

func (h *Handler) currentPromptTemplate(r *http.Request) string {
	if h.promptProvider != nil {
		return h.promptProvider(r)
	}
	return h.promptTemplate
}

func (h *Handler) isTokenAuthorized(r *http.Request, token string, scope string) bool {
	if h.authValidator != nil {
		return h.authValidator(r, scope)
	}
	return h.authTokens[token]
}

// validateRequest 验证请求参数
func (h *Handler) validateRequest(req *TranslateRequest) error {
	if req.Text == "" {
		return errors.New("text is required")
	}
	if req.TargetLang == "" {
		return errors.New("target_lang is required")
	}
	return nil
}

// sendResponse 发送成功响应
func (h *Handler) sendResponse(w http.ResponseWriter, translation, sourceLang, targetLang string) {
	resp := TranslateResponse{
		Code:       200,
		Data:       translation,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// sendError 发送错误响应
func (h *Handler) sendError(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := TranslateResponse{
		Data: message,
		Code: status,
	}

	json.NewEncoder(w).Encode(resp)
}
