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
	"transbridge/store"
	"transbridge/translator"
)

// TranslationService 封装翻译服务的所有操作
type TranslationService struct {
	modelManager *translator.ModelManager
	cache        cache.Cache
	logger       *logger.TranslationLogger // 新增日志记录器
	store        *store.Store
}

// TranslateRequest 翻译请求参数
type TranslateRequest struct {
	Text       string
	SourceLang string
	TargetLang string
	Provider   string // 可选，指定服务提供商
	Model      string // 可选，指定模型
}

func (s *TranslationService) SetStore(st *store.Store) {
	s.store = st
}

// NewTranslationService 创建翻译服务实例
func NewTranslationService(modelManager *translator.ModelManager, cache cache.Cache, translLogger *logger.TranslationLogger) *TranslationService {
	return &TranslationService{
		modelManager: modelManager,
		cache:        cache,
		logger:       translLogger,
	}
}

// Translate 处理翻译请求，自动处理缓存逻辑。使用 general 策略。
func (s *TranslationService) Translate(ctx context.Context, provider, model, promptTemplate, text, sourceLang, targetLang string) (string, error) {
	return s.translateWithMode(ctx, provider, model, promptTemplate, text, sourceLang, targetLang, policyModeGeneral)
}

// TranslateConservative 与 Translate 相同，但走保守策略：不在词典里的短英文单词（≤12 字符）会被 skip，
// 保留原文。适合表格单元格场景，避免把姓名、缩写、单位符号送模型翻译。
func (s *TranslationService) TranslateConservative(ctx context.Context, provider, model, promptTemplate, text, sourceLang, targetLang string) (string, error) {
	return s.translateWithMode(ctx, provider, model, promptTemplate, text, sourceLang, targetLang, policyModeConservative)
}

func (s *TranslationService) translateWithMode(ctx context.Context, provider, model, promptTemplate, text, sourceLang, targetLang string, mode translationPolicyMode) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	if targetLang == "" {
		return "", fmt.Errorf("target language is required")
	}

	var cacheKey string

	startTime := time.Now()

	policy := applyTranslationPolicyWithMode(text, targetLang, mode)
	if policy.Decision == decisionSkip || policy.Decision == decisionDict {
		s.logTranslation(text, policy.Output, sourceLang, targetLang, "", string(policy.Decision), policy.Reason, "", false, time.Since(startTime).Milliseconds())
		s.logRequest(ctx, text, policy.Output, sourceLang, targetLang, "", string(policy.Decision), "", false, true, "", time.Since(startTime).Milliseconds())
		return policy.Output, nil
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

	// 2. 尝试从缓存获取（优先模型级 key，并兼容旧 key 和旧前缀 transbrige:）
	if s.cache != nil {
		cacheKey = utils.GenerateModelCacheKey(usedTranslator.GetProvider(), usedTranslator.GetModel(), text, sourceLang, targetLang)
		if cachedData, err := s.cache.Get(ctx, cacheKey); err == nil && cachedData != "" {
			var entry cache.CacheEntry
			if err := json.Unmarshal([]byte(cachedData), &entry); err == nil && shouldCacheTranslation(text, entry.Translation) {
				log.Printf("Cache hit for: %s, originally translated by %s, models : %s",
					cacheKey, entry.APIURL, entry.Model)
				s.logTranslation(text, entry.Translation, sourceLang, targetLang, entry.APIURL, entry.Provider, entry.Model, cacheKey, true, time.Since(startTime).Milliseconds())
				s.logRequest(ctx, text, entry.Translation, sourceLang, targetLang, entry.APIURL, entry.Provider, entry.Model, true, true, "", time.Since(startTime).Milliseconds())
				return entry.Translation, nil
			}
		}

		legacyKey := utils.GenerateCacheKey(text, sourceLang, targetLang)
		if cachedData, err := s.cache.Get(ctx, legacyKey); err == nil && cachedData != "" {
			var entry cache.CacheEntry
			if err := json.Unmarshal([]byte(cachedData), &entry); err == nil && shouldCacheTranslation(text, entry.Translation) {
				log.Printf("Legacy cache hit for: %s, originally translated by %s, models : %s",
					legacyKey, entry.APIURL, entry.Model)
				s.logTranslation(text, entry.Translation, sourceLang, targetLang, entry.APIURL, entry.Provider, entry.Model, legacyKey, true, time.Since(startTime).Milliseconds())
				s.logRequest(ctx, text, entry.Translation, sourceLang, targetLang, entry.APIURL, entry.Provider, entry.Model, true, true, "", time.Since(startTime).Milliseconds())
				return entry.Translation, nil
			}
		} else {
			// 向后兼容旧键前缀
			fallbackKey := strings.Replace(legacyKey, "transbridge:", "transbrige:", 1)
			if fallbackKey != legacyKey {
				if cachedData2, err2 := s.cache.Get(ctx, fallbackKey); err2 == nil && cachedData2 != "" {
					var entry2 cache.CacheEntry
					if err := json.Unmarshal([]byte(cachedData2), &entry2); err == nil && shouldCacheTranslation(text, entry2.Translation) {
						log.Printf("Cache hit (legacy key) for: %s, originally translated by %s/%s",
							fallbackKey, entry2.APIURL, entry2.Model)
						s.logTranslation(text, entry2.Translation, sourceLang, targetLang, entry2.APIURL, entry2.Provider, entry2.Model, fallbackKey, true, time.Since(startTime).Milliseconds())
						s.logRequest(ctx, text, entry2.Translation, sourceLang, targetLang, entry2.APIURL, entry2.Provider, entry2.Model, true, true, "", time.Since(startTime).Milliseconds())
						return entry2.Translation, nil
					}
				}
			}
		}
	}

	// 3. 执行翻译
	translation, err := usedTranslator.Translate(ctx, promptTemplate, text, sourceLang, targetLang)
	if err != nil {
		// 记录失败的翻译
		s.logRequest(ctx, text, "", sourceLang, targetLang, usedTranslator.GetAPIURL(), usedTranslator.GetProvider(), usedTranslator.GetModel(), false, false, err.Error(), time.Since(startTime).Milliseconds())
		return "", fmt.Errorf("translation failed (url=%s model=%s): %w",
			usedTranslator.GetAPIURL(), usedTranslator.GetModel(), err)
	}

	translationCacheable := shouldCacheTranslation(text, translation)

	// 4. 缓存成功且通过质量门禁的翻译结果（包含模型信息）
	if s.cache != nil && translationCacheable {
		cacheEntry := cache.CacheEntry{
			Translation: translation,
			Provider:    usedTranslator.GetProvider(),
			APIURL:      usedTranslator.GetAPIURL(),
			Model:       usedTranslator.GetModel(),
		}

		// 序列化缓存条目
		cacheData, err := json.Marshal(cacheEntry)
		if err == nil {
			cacheKey = utils.GenerateModelCacheKey(usedTranslator.GetProvider(), usedTranslator.GetModel(), text, sourceLang, targetLang)
			// 让底层缓存实现使用其默认 TTL（传 0）或永久（由实现决定）
			if err := s.cache.Set(ctx, cacheKey, string(cacheData), 0); err != nil {
				log.Printf("Failed to cache translation: %v", err)
			}
		}
	} else if !translationCacheable {
		log.Printf("Skipped caching unusable translation for %s/%s", usedTranslator.GetProvider(), usedTranslator.GetModel())
	}

	// 记录翻译
	s.logTranslation(text, translation, sourceLang, targetLang, usedTranslator.GetAPIURL(), usedTranslator.GetProvider(), usedTranslator.GetModel(), cacheKey, false, time.Since(startTime).Milliseconds())
	s.logRequest(ctx, text, translation, sourceLang, targetLang, usedTranslator.GetAPIURL(), usedTranslator.GetProvider(), usedTranslator.GetModel(), false, true, "", time.Since(startTime).Milliseconds())

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

func (s *TranslationService) logRequest(ctx context.Context, sourceText, targetText, sourceLang, targetLang, apiURL, provider, model string, cacheHit, success bool, errText string, processTimeMs int64) {
	if s.store == nil {
		return
	}
	if err := s.store.LogRequest(ctx, store.RequestLog{
		Timestamp:     time.Now(),
		Endpoint:      "translate",
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
		Provider:      provider,
		Model:         model,
		APIURL:        apiURL,
		CacheHit:      cacheHit,
		Success:       success,
		Error:         errText,
		ProcessTimeMS: float64(processTimeMs),
		SourceChars:   len([]rune(sourceText)),
		TargetChars:   len([]rune(targetText)),
	}); err != nil {
		log.Printf("Failed to log request stats: %v", err)
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
