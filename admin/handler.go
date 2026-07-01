package admin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"transbridge/config"
	"transbridge/service"
	"transbridge/store"
	"transbridge/translator"
)

const adminSessionCookie = "tb_admin"
const adminSessionTTL = 12 * time.Hour

type Handler struct {
	store        *store.Store
	cfg          *config.Config
	modelManager *translator.ModelManager
	translation  *service.TranslationService
	basePath     string
}

func NewHandler(st *store.Store, cfg *config.Config, modelManager *translator.ModelManager, translation *service.TranslationService) *Handler {
	basePath := cfg.Admin.Path
	if basePath == "" {
		basePath = "/admin"
	}
	basePath = "/" + strings.Trim(basePath, "/")
	return &Handler{store: st, cfg: cfg, modelManager: modelManager, translation: translation, basePath: basePath}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	path := strings.TrimPrefix(r.URL.Path, h.basePath)
	if path == "" || path == "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
		return
	}
	if strings.HasPrefix(path, "/api/") {
		h.serveAPI(w, r, strings.TrimPrefix(path, "/api"))
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) serveAPI(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case path == "/stats" && r.Method == http.MethodGet:
		stats, err := h.store.Stats(r.Context())
		h.write(w, stats, err)
	case path == "/models" && r.Method == http.MethodGet:
		models, err := h.store.ListModelViews(r.Context())
		h.write(w, models, err)
	case path == "/models" && r.Method == http.MethodPost:
		h.upsertModel(w, r)
	case path == "/models" && r.Method == http.MethodDelete:
		h.deleteModel(w, r)
	case path == "/models/test" && r.Method == http.MethodPost:
		h.testModel(w, r)
	case path == "/tokens" && r.Method == http.MethodGet:
		tokens, err := h.store.ListTokenViews(r.Context())
		h.write(w, tokens, err)
	case path == "/tokens" && r.Method == http.MethodPost:
		h.createToken(w, r)
	case path == "/tokens" && r.Method == http.MethodPut:
		h.updateToken(w, r)
	case path == "/tokens" && r.Method == http.MethodDelete:
		h.deleteToken(w, r)
	case path == "/prompts" && r.Method == http.MethodGet:
		prompts, err := h.store.ListPrompts(r.Context())
		h.write(w, prompts, err)
	case path == "/prompts" && r.Method == http.MethodPost:
		h.createPrompt(w, r)
	case path == "/prompts/activate" && r.Method == http.MethodPost:
		h.activatePrompt(w, r)
	case path == "/logs" && r.Method == http.MethodGet:
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		logs, err := h.store.ListRequestLogs(r.Context(), limit)
		h.write(w, logs, err)
	case path == "/translate" && r.Method == http.MethodPost:
		h.tryTranslate(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) tryTranslate(w http.ResponseWriter, r *http.Request) {
	if h.translation == nil {
		h.errorText(w, http.StatusServiceUnavailable, "translation service not available")
		return
	}
	var req struct {
		Provider   string `json:"provider"`
		Model      string `json:"model"`
		Text       string `json:"text"`
		SourceLang string `json:"source_lang"`
		TargetLang string `json:"target_lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	if req.Text == "" || req.TargetLang == "" {
		h.errorText(w, http.StatusBadRequest, "text and target_lang are required")
		return
	}

	promptTemplate, err := h.store.ActivePrompt(r.Context(), h.cfg.Prompt.Template)
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}

	start := time.Now()
	translation, err := h.translation.Translate(r.Context(), req.Provider, req.Model, promptTemplate, req.Text, req.SourceLang, req.TargetLang)
	elapsedMs := time.Since(start).Milliseconds()
	if err != nil {
		h.errorText(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.write(w, map[string]interface{}{
		"translation":   translation,
		"elapsed_ms":    elapsedMs,
		"source_lang":   req.SourceLang,
		"target_lang":   req.TargetLang,
		"used_provider": req.Provider,
		"used_model":    req.Model,
	}, nil)
}

func (h *Handler) upsertModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider          string         `json:"provider"`
		APIURL            string         `json:"api_url"`
		APIKey            string         `json:"api_key"`
		ProviderTimeout   int            `json:"provider_timeout"`
		IsDefault         bool           `json:"is_default"`
		Name              string         `json:"name"`
		Weight            int            `json:"weight"`
		TopP              int            `json:"top_p"`
		MaxTokens         int            `json:"max_tokens"`
		Temperature       float32        `json:"temperature"`
		Timeout           *int           `json:"timeout"`
		Enabled           bool           `json:"enabled"`
		RateLimit         store.RateSpec `json:"rate_limit"`
		ProviderRateLimit store.RateSpec `json:"provider_rate_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	if req.Provider == "" || req.APIURL == "" || req.Name == "" {
		h.errorText(w, http.StatusBadRequest, "provider, api_url and name are required")
		return
	}
	if req.Provider != "openai" {
		h.errorText(w, http.StatusBadRequest, "only openai-compatible provider is supported")
		return
	}
	if req.ProviderTimeout <= 0 {
		req.ProviderTimeout = 60
	}
	if !req.Enabled {
		if ok := h.canDisableModel(w, r, req.Provider, req.APIURL, req.Name); !ok {
			return
		}
	}
	if req.Weight == 0 {
		req.Weight = 1
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 2000
	}
	if req.Temperature == 0 {
		req.Temperature = 0.3
	}
	provider := config.ProviderConfig{
		Provider:  req.Provider,
		APIURL:    req.APIURL,
		APIKey:    req.APIKey,
		Timeout:   req.ProviderTimeout,
		IsDefault: req.IsDefault,
		RateLimit: config.RateLimitConfig{
			MaxConcurrent: req.ProviderRateLimit.MaxConcurrent,
			QPS:           req.ProviderRateLimit.QPS,
			QPM:           req.ProviderRateLimit.QPM,
		},
	}
	model := config.ModelConfig{
		Name:        req.Name,
		Weight:      req.Weight,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Timeout:     req.Timeout,
		RateLimit: config.RateLimitConfig{
			MaxConcurrent: req.RateLimit.MaxConcurrent,
			QPS:           req.RateLimit.QPS,
			QPM:           req.RateLimit.QPM,
		},
	}
	if err := h.store.UpsertProviderModel(r.Context(), provider, model, req.Enabled); err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}
	h.reloadModels(w, r.Context())
}

func (h *Handler) deleteModel(w http.ResponseWriter, r *http.Request) {
	if !h.canDeleteModel(w, r) {
		return
	}
	var err error
	if idText := r.URL.Query().Get("id"); idText != "" {
		id, parseErr := strconv.ParseInt(idText, 10, 64)
		if parseErr != nil {
			h.error(w, http.StatusBadRequest, parseErr)
			return
		}
		err = h.store.DeleteModel(r.Context(), id)
	} else {
		err = h.store.DeleteModelByName(r.Context(), r.URL.Query().Get("provider"), r.URL.Query().Get("api_url"), r.URL.Query().Get("model"))
	}
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}
	h.reloadModels(w, r.Context())
}

func (h *Handler) canDeleteModel(w http.ResponseWriter, r *http.Request) bool {
	models, err := h.store.ListModels(r.Context())
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return false
	}
	enabledCount := 0
	for _, model := range models {
		if model.Enabled {
			enabledCount++
		}
	}
	for _, model := range models {
		matches := false
		if idText := r.URL.Query().Get("id"); idText != "" {
			id, err := strconv.ParseInt(idText, 10, 64)
			if err != nil {
				h.error(w, http.StatusBadRequest, err)
				return false
			}
			matches = model.ID == id
		} else {
			matches = model.Provider == r.URL.Query().Get("provider") && model.APIURL == r.URL.Query().Get("api_url") && model.Name == r.URL.Query().Get("model")
		}
		if matches && model.Enabled && enabledCount <= 1 {
			h.errorText(w, http.StatusBadRequest, "cannot delete the last enabled model")
			return false
		}
	}
	return true
}

func (h *Handler) testModel(w http.ResponseWriter, r *http.Request) {
	if h.translation == nil {
		h.errorText(w, http.StatusServiceUnavailable, "translation service not available")
		return
	}
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		h.errorText(w, http.StatusBadRequest, "id required")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	models, err := h.store.ListModelViews(r.Context())
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}
	var target *store.ModelView
	for _, m := range models {
		if m.ID == id {
			target = &m
			break
		}
	}
	if target == nil {
		h.errorText(w, http.StatusNotFound, "model not found")
		return
	}
	promptTemplate, err := h.store.ActivePrompt(r.Context(), h.cfg.Prompt.Template)
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}
	start := time.Now()
	_, testErr := h.translation.Translate(r.Context(), target.Provider, target.Name, promptTemplate, "hi", "", "en")
	latency := time.Since(start).Milliseconds()
	if testErr != nil {
		h.write(w, map[string]interface{}{"success": false, "latency_ms": latency, "error": testErr.Error()}, nil)
	} else {
		h.write(w, map[string]interface{}{"success": true, "latency_ms": latency}, nil)
	}
}

func (h *Handler) canDisableModel(w http.ResponseWriter, r *http.Request, provider, apiURL, name string) bool {
	models, err := h.store.ListModels(r.Context())
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return false
	}
	enabledCount := 0
	for _, model := range models {
		if model.Enabled {
			enabledCount++
		}
	}
	for _, model := range models {
		if model.Provider == provider && model.APIURL == apiURL && model.Name == name && model.Enabled && enabledCount <= 1 {
			h.errorText(w, http.StatusBadRequest, "cannot disable the last enabled model")
			return false
		}
	}
	return true
}

func (h *Handler) reloadModels(w http.ResponseWriter, ctx context.Context) {
	providers, err := h.store.LoadProviders(ctx)
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.modelManager.Reload(providers); err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	h.write(w, map[string]string{"status": "ok"}, nil)
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Token   string `json:"token"`
		Scope   string `json:"scope"`
		Enabled *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	if req.Token == "" {
		h.errorText(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.Scope == "" {
		req.Scope = "translate"
	}
	if !validTokenScope(req.Scope) {
		h.errorText(w, http.StatusBadRequest, "invalid token scope")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	h.write(w, map[string]string{"status": "ok"}, h.store.CreateToken(r.Context(), store.Token{
		Name:    req.Name,
		Token:   req.Token,
		Scope:   req.Scope,
		Enabled: enabled,
	}))
}

func (h *Handler) updateToken(w http.ResponseWriter, r *http.Request) {
	var token store.Token
	if err := json.NewDecoder(r.Body).Decode(&token); err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	if token.ID == 0 || token.Token == "" {
		h.errorText(w, http.StatusBadRequest, "id and token are required")
		return
	}
	if !validTokenScope(token.Scope) {
		h.errorText(w, http.StatusBadRequest, "invalid token scope")
		return
	}
	h.write(w, map[string]string{"status": "ok"}, h.store.UpdateToken(r.Context(), token))
}

func (h *Handler) deleteToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	h.write(w, map[string]string{"status": "ok"}, h.store.DeleteToken(r.Context(), id))
}

func (h *Handler) createPrompt(w http.ResponseWriter, r *http.Request) {
	var prompt store.PromptVersion
	if err := json.NewDecoder(r.Body).Decode(&prompt); err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	if prompt.Template == "" {
		h.errorText(w, http.StatusBadRequest, "template is required")
		return
	}
	h.write(w, map[string]string{"status": "ok"}, h.store.CreatePrompt(r.Context(), prompt))
}

func (h *Handler) activatePrompt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		h.error(w, http.StatusBadRequest, err)
		return
	}
	h.write(w, map[string]string{"status": "ok"}, h.store.ActivatePrompt(r.Context(), id))
}

func (h *Handler) authorized(w http.ResponseWriter, r *http.Request) bool {
	if h.cfg.Admin.LocalOnly && !isLocal(r.RemoteAddr) {
		http.Error(w, "admin is local only", http.StatusForbidden)
		return false
	}
	if h.cfg.Admin.Username == "" && h.cfg.Admin.Password == "" {
		return true
	}
	if h.validSessionCookie(r) {
		h.issueSessionCookie(w, r)
		return true
	}
	user, pass, ok := r.BasicAuth()
	if !ok ||
		subtle.ConstantTimeCompare([]byte(user), []byte(h.cfg.Admin.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(pass), []byte(h.cfg.Admin.Password)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="transbridge admin"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return false
	}
	h.issueSessionCookie(w, r)
	return true
}

func (h *Handler) sessionKey() []byte {
	return []byte(h.cfg.Admin.Username + "\x00" + h.cfg.Admin.Password)
}

func (h *Handler) signSession(payload string) string {
	mac := hmac.New(sha256.New, h.sessionKey())
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (h *Handler) issueSessionCookie(w http.ResponseWriter, r *http.Request) {
	exp := time.Now().Add(adminSessionTTL).Unix()
	payload := h.cfg.Admin.Username + ":" + strconv.FormatInt(exp, 10)
	value := payload + ":" + h.signSession(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    value,
		Path:     h.basePath,
		Expires:  time.Unix(exp, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})
}

func (h *Handler) validSessionCookie(r *http.Request) bool {
	c, err := r.Cookie(adminSessionCookie)
	if err != nil {
		return false
	}
	parts := strings.SplitN(c.Value, ":", 3)
	if len(parts) != 3 {
		return false
	}
	user, expStr, sig := parts[0], parts[1], parts[2]
	if subtle.ConstantTimeCompare([]byte(user), []byte(h.cfg.Admin.Username)) != 1 {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() >= exp {
		return false
	}
	expected := h.signSession(user + ":" + expStr)
	return subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) == 1
}

func isLocal(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validTokenScope(scope string) bool {
	return scope == "translate" || scope == "openai" || scope == "all"
}

func (h *Handler) write(w http.ResponseWriter, data any, err error) {
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) error(w http.ResponseWriter, status int, err error) {
	h.errorText(w, status, err.Error())
}

func (h *Handler) errorText(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
