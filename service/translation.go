package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"transbridge/cache"
	"transbridge/internal/utils"
	"transbridge/logger"
	"transbridge/translator"
)

// TranslationService 封装翻译服务的所有操作
type TranslationService struct {
	modelManager *translator.ModelManager
	cache        cache.Cache
	logger       *logger.TranslationLogger // 新增日志记录器
}

// TranslateRequest 翻译请求参数
type TranslateRequest struct {
	Text       string
	SourceLang string
	TargetLang string
	Provider   string // 可选，指定服务提供商
	Model      string // 可选，指定模型
}

// NewTranslationService 创建翻译服务实例
func NewTranslationService(modelManager *translator.ModelManager, cache cache.Cache, translLogger *logger.TranslationLogger) *TranslationService {
	return &TranslationService{
		modelManager: modelManager,
		cache:        cache,
		logger:       translLogger,
	}
}

// Translate 处理翻译请求，自动处理缓存逻辑
func (s *TranslationService) Translate(ctx context.Context, provider, model, promptTemplate, text, sourceLang, targetLang string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	if targetLang == "" {
		return "", fmt.Errorf("target language is required")
	}

	var cacheKey string

	startTime := time.Now()

	// 2. 尝试从缓存获取（并兼容旧前缀 transbrige:）
	if s.cache != nil {
		cacheKey = utils.GenerateCacheKey(text, sourceLang, targetLang)
		if cachedData, err := s.cache.Get(ctx, cacheKey); err == nil && cachedData != "" {
			var entry cache.CacheEntry
			if err := json.Unmarshal([]byte(cachedData), &entry); err == nil {
				log.Printf("Cache hit for: %s, originally translated by %s, models : %s",
					cacheKey, entry.APIURL, entry.Model)
				s.logTranslation(text, entry.Translation, sourceLang, targetLang, entry.APIURL, entry.Provider, entry.Model, cacheKey, true, time.Since(startTime).Milliseconds())
				return entry.Translation, nil
			}
		} else {
			// 向后兼容旧键前缀
			fallbackKey := strings.Replace(cacheKey, "transbridge:", "transbrige:", 1)
			if fallbackKey != cacheKey {
				if cachedData2, err2 := s.cache.Get(ctx, fallbackKey); err2 == nil && cachedData2 != "" {
					var entry2 cache.CacheEntry
					if err := json.Unmarshal([]byte(cachedData2), &entry2); err == nil {
						log.Printf("Cache hit (legacy key) for: %s, originally translated by %s/%s",
							fallbackKey, entry2.APIURL, entry2.Model)
						s.logTranslation(text, entry2.Translation, sourceLang, targetLang, entry2.APIURL, entry2.Provider, entry2.Model, fallbackKey, true, time.Since(startTime).Milliseconds())
						return entry2.Translation, nil
					}
				}
			}
		}
	}

	var usedTranslator translator.Translator
	var err error
	// 1. 首先尝试获取指定的翻译器
	if provider != "" && model != "" {
		usedTranslator, err = s.modelManager.GetModel(provider, model)
		if err != nil {
			log.Printf("Specified model %s/%s not found: %v, falling back to default", provider, model, err)
			usedTranslator = s.modelManager.GetDefaultModel()
		}
	} else {
		usedTranslator = s.modelManager.GetRandomModel()
	}

	// 3. 执行翻译
	translation, err := usedTranslator.Translate(ctx, promptTemplate, text, sourceLang, targetLang)
	if err != nil {
		// 记录失败的翻译
		return "", fmt.Errorf("translation failed with %s/%s: %w",
			usedTranslator.GetAPIURL(), usedTranslator.GetModel(), err)
	}

	// 4. 缓存成功的翻译结果（包含模型信息）
	if s.cache != nil {
		cacheEntry := cache.CacheEntry{
			Translation: translation,
			Provider:    usedTranslator.GetProvider(),
			APIURL:      usedTranslator.GetAPIURL(),
			Model:       usedTranslator.GetModel(),
		}

		// 序列化缓存条目
		cacheData, err := json.Marshal(cacheEntry)
		if err == nil {
			cacheKey = utils.GenerateCacheKey(text, sourceLang, targetLang)
			// 让底层缓存实现使用其默认 TTL（传 0）或永久（由实现决定）
			if err := s.cache.Set(ctx, cacheKey, string(cacheData), 0); err != nil {
				log.Printf("Failed to cache translation: %v", err)
			}
		}
	}

	// 记录翻译
	s.logTranslation(text, translation, sourceLang, targetLang, usedTranslator.GetAPIURL(), usedTranslator.GetProvider(), usedTranslator.GetModel(), cacheKey, false, time.Since(startTime).Milliseconds())

	return translation, nil
}

// GetAvailableModels 获取所有可用的翻译模型
func (s *TranslationService) GetAvailableModels() []translator.ModelIdentifier {
	return s.modelManager.ListModels()
}

// GetProviderModels 获取指定提供商的所有可用模型
func (s *TranslationService) GetProviderModels(provider string) []string {
	return s.modelManager.GetModelsByProvider(provider)
}

// BatchTranslate 批量翻译
func (s *TranslationService) BatchTranslate(ctx context.Context, promptTemplate string, requests []TranslateRequest) []struct {
	Text  string
	Error error
} {
	results := make([]struct {
		Text  string
		Error error
	}, len(requests))

	for i, req := range requests {
		translation, err := s.Translate(ctx, req.Provider, req.Model, promptTemplate, req.Text, req.SourceLang, req.TargetLang)
		results[i] = struct {
			Text  string
			Error error
		}{
			Text:  translation,
			Error: err,
		}
	}

	return results
}

// logTranslation 记录翻译日志
func (s *TranslationService) logTranslation(sourceText, targetText, sourceLang, targetLang, apiURL, provider, model string, cacheKey string, cacheHit bool, processTimeMs int64) {
	if s.logger == nil {
		return
	}

	record := logger.TranslationRecord{
		SourceText:  sourceText,
		TargetText:  targetText,
		SourceLang:  sourceLang,
		TargetLang:  targetLang,
		APIURL:      apiURL,
		Provider:    provider,
		Model:       model,
		CacheKey:    cacheKey,
		CacheHit:    cacheHit,
		ProcessTime: float64(processTimeMs),
	}

	if err := s.logger.LogTranslation(record); err != nil {
		log.Printf("Failed to log translation: %v", err)
	}
}

// ValidateLanguage 检查语言代码是否有效
func (s *TranslationService) ValidateLanguage(lang string) bool {
	return utils.IsValidLanguageCode(lang)
}

// Close 关闭服务
func (s *TranslationService) Close() error {
	return nil
}
