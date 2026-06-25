// api/openai/openai_handler.go
package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"transbridge/translator"

	"github.com/sashabaranov/go-openai"
)

type OpenAIHandler struct {
	modelManager *translator.ModelManager
	authTokens   map[string]bool
}

// ModelInfo 用于 API 响应的模型信息
type ModelInfo struct {
	ID         string        `json:"id"`
	Object     string        `json:"object"`
	Created    int           `json:"created"`
	Owned      bool          `json:"owned_by"`
	Permission []interface{} `json:"permission"`
}

// ModelsResponse OpenAI 模型列表响应格式
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

func NewOpenAIHandler(modelManager *translator.ModelManager, authTokens []string) *OpenAIHandler {
	tokenMap := make(map[string]bool)
	for _, token := range authTokens {
		tokenMap[token] = true
	}

	return &OpenAIHandler{
		modelManager: modelManager,
		authTokens:   tokenMap,
	}
}

func (h *OpenAIHandler) HandleChatCompletion(w http.ResponseWriter, r *http.Request) {
	// 验证 API 密钥
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !h.authTokens[token] {
		h.sendError(w, "Unauthorized", "unauthorized", http.StatusUnauthorized)
		return
	}

	// 解析请求
	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", "invalid_request", http.StatusBadRequest)
		return
	}

	// 获取提供商和模型信息
	providerName, modelName := h.parseModelIdentifier(req.Model)

	// 获取模型
	model, err := h.modelManager.GetModel(providerName, modelName)
	if err != nil {
		h.sendError(w, fmt.Sprintf("Model %s not found", req.Model), "invalid_model", http.StatusBadRequest)
		return
	}

	// 尝试获取 OpenAI 翻译器
	openaiTranslator, ok := model.(*translator.OpenAITranslator)
	if !ok {
		h.sendError(w, fmt.Sprintf("Model %s/%s is not an OpenAI model", providerName, modelName), "invalid_model", http.StatusBadRequest)
		return
	}

	// 创建聊天完成实例
	chatCompletion := translator.NewOpenAIChatCompletion(openaiTranslator)
	req.Model = openaiTranslator.GetModel()

	// 处理请求
	openaiResp, err := chatCompletion.CreateChatCompletion(r.Context(), req)
	if err != nil {
		h.sendError(w, err.Error(), "internal_error", http.StatusInternalServerError)
		return
	}

	// 发送响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(openaiResp)
}

func (h *OpenAIHandler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	// 验证 API 密钥以保持与其它端点一致
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !h.authTokens[token] {
		h.sendError(w, "Unauthorized", "unauthorized", http.StatusUnauthorized)
		return
	}

	// 获取所有可用模型
	models := h.modelManager.ListModels()

	// 转换为 OpenAI 格式的响应
	response := ModelsResponse{
		Object: "list",
		Data:   make([]ModelInfo, 0, len(models)),
	}

	// 构建模型信息
	for _, model := range models {
		response.Data = append(response.Data, ModelInfo{
			ID:         model.String(), // provider/model
			Object:     "model",
			Created:    1677610602, // 固定时间戳
			Owned:      true,
			Permission: []interface{}{}, // 简化的权限信息
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// parseModelIdentifier 从模型标识符解析提供商和模型名称。
// 兼容接口的模型列表返回 provider/model，调用方也应使用同样格式。
func (h *OpenAIHandler) parseModelIdentifier(modelID string) (provider, model string) {
	parts := strings.Split(modelID, "/")
	if len(parts) != 2 {
		return "", modelID
	}
	return parts[0], parts[1]
}

// sendError 发送标准化的错误响应
func (h *OpenAIHandler) sendError(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	error := struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}{}

	error.Error.Message = message
	error.Error.Type = "invalid_request_error"
	error.Error.Code = code

	json.NewEncoder(w).Encode(error)
}
