// translator/model_manager.go
package translator

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"transbridge/config"
)

// 模型标识结构
type ModelIdentifier struct {
	Provider string // 服务提供商
	Model    string // 模型名称
	APIURL   string // 模型服务地址
}

func (m ModelIdentifier) String() string {
	return fmt.Sprintf("%s/%s", m.Provider, m.Model)
}

// ModelManager 管理多个服务提供商和其模型
type ModelManager struct {
	translators  map[ModelIdentifier]Translator
	modelWeights map[ModelIdentifier]int
	modelOrder   []ModelIdentifier
	defaultModel ModelIdentifier
	mu           sync.RWMutex
	rng          *rand.Rand
}

func NewModelManager(providers []config.ProviderConfig) (*ModelManager, error) {
	if len(providers) == 0 {
		return nil, errors.New("no providers configured")
	}

	mm := &ModelManager{
		translators:  make(map[ModelIdentifier]Translator),
		modelWeights: make(map[ModelIdentifier]int),
	}

	// 使用独立的随机源，避免未播种导致的可预测选择
	mm.rng = rand.New(rand.NewSource(time.Now().UnixNano()))

	var defaultFound bool
	for _, provider := range providers {
		// 获取提供商的默认超时时间
		defaultTimeout := provider.Timeout

		for _, modelCfg := range provider.Models {
			// 确定模型的超时时间
			timeout := defaultTimeout
			if modelCfg.Timeout != nil {
				timeout = *modelCfg.Timeout
			}

			identifier := ModelIdentifier{
				Provider: provider.Provider,
				Model:    modelCfg.Name,
				APIURL:   provider.APIURL,
			}

			var translator Translator

			switch provider.Provider {
			case "openai":
				translator = NewOpenAITranslator(
					provider.Provider,
					provider.APIURL,
					provider.APIKey,
					modelCfg.Name,
					timeout,
					modelCfg.MaxTokens,
					modelCfg.Temperature,
				)
			default:
				return nil, fmt.Errorf("unsupported provider: %s", provider.Provider)
			}
			translator = newRateLimitedTranslator(translator, mergeRateLimit(provider.RateLimit, modelCfg.RateLimit))

			mm.translators[identifier] = translator
			mm.modelWeights[identifier] = modelCfg.Weight
			mm.modelOrder = append(mm.modelOrder, identifier)

			// 如果是默认提供商的第一个模型，设为默认模型
			if provider.IsDefault && !defaultFound {
				mm.defaultModel = identifier
				defaultFound = true
			}
		}
	}

	if len(mm.modelOrder) == 0 {
		return nil, errors.New("no models configured")
	}

	if !defaultFound {
		// 如果没有设置默认模型，使用配置中的第一个可用模型
		mm.defaultModel = mm.modelOrder[0]
	}

	return mm, nil
}

// Reload replaces all translators atomically from the latest provider config.
func (mm *ModelManager) Reload(providers []config.ProviderConfig) error {
	next, err := NewModelManager(providers)
	if err != nil {
		return err
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	oldTranslators := mm.translators
	mm.translators = next.translators
	mm.modelWeights = next.modelWeights
	mm.modelOrder = next.modelOrder
	mm.defaultModel = next.defaultModel
	mm.rng = next.rng

	for _, tr := range oldTranslators {
		_ = tr.Close()
	}
	return nil
}

func mergeRateLimit(providerLimit, modelLimit config.RateLimitConfig) config.RateLimitConfig {
	merged := providerLimit
	if modelLimit.MaxConcurrent > 0 {
		merged.MaxConcurrent = modelLimit.MaxConcurrent
	}
	if modelLimit.QPS > 0 {
		merged.QPS = modelLimit.QPS
	}
	if modelLimit.QPM > 0 {
		merged.QPM = modelLimit.QPM
	}
	return merged
}

// GetModel 获取指定提供商和模型的翻译器
func (mm *ModelManager) GetModel(provider, model string) (Translator, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// 由于存储键包含 APIURL，这里以 provider+model 进行匹配查找
	for _, id := range mm.modelOrder {
		if id.Provider == provider && id.Model == model {
			return mm.translators[id], nil
		}
	}
	return nil, fmt.Errorf("model %s not found for provider %s", model, provider)
}

// GetDefaultModel 获取默认模型
func (mm *ModelManager) GetDefaultModel() Translator {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.translators[mm.defaultModel]
}

func (mm *ModelManager) GetRandomModel() Translator {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	var totalWeight int
	for _, identifier := range mm.modelOrder {
		if weight := mm.modelWeights[identifier]; weight > 0 {
			totalWeight += weight
		}
	}

	if totalWeight <= 0 {
		return mm.translators[mm.defaultModel]
	}

	r := mm.rng.Intn(totalWeight)
	for _, identifier := range mm.modelOrder {
		weight := mm.modelWeights[identifier]
		if weight <= 0 {
			continue
		}
		r -= weight
		if r <= 0 {
			return mm.translators[identifier]
		}
	}

	return mm.translators[mm.defaultModel]
}

// ListModels 列出所有可用的模型
func (mm *ModelManager) ListModels() []ModelIdentifier {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	models := make([]ModelIdentifier, 0, len(mm.translators))
	for _, identifier := range mm.modelOrder {
		models = append(models, identifier)
	}
	return models
}

// GetModelsByProvider 获取指定提供商的所有模型
func (mm *ModelManager) GetModelsByProvider(provider string) []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var models []string
	for _, identifier := range mm.modelOrder {
		if identifier.Provider == provider {
			models = append(models, identifier.Model)
		}
	}
	return models
}
