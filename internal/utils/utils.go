package utils

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	iso639 "github.com/emvi/iso-639-1"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// GenerateCacheKey 生成缓存键
func GenerateCacheKey(text, sourceLang, targetLang string) string {
	// 组合键的各个部分
	key := strings.Join([]string{
		sourceLang,
		targetLang,
		text,
	}, ":")

	// 计算MD5哈希
	hasher := md5.New()
	hasher.Write([]byte(key))
	md5string := hex.EncodeToString(hasher.Sum(nil))

	return "transbridge:" + md5string
}

// GenerateModelCacheKey 生成模型级缓存键，避免不同模型/上游之间互相污染。
func GenerateModelCacheKey(provider, model, text, sourceLang, targetLang string) string {
	key := strings.Join([]string{
		provider,
		model,
		sourceLang,
		targetLang,
		text,
	}, ":")

	hasher := md5.New()
	hasher.Write([]byte(key))
	md5string := hex.EncodeToString(hasher.Sum(nil))

	return "transbridge:v2:" + md5string
}

// IsValidLanguageCode 检查语言代码是否有效
func IsValidLanguageCode(code string) bool {

	return iso639.ValidCode(strings.ToLower(code))
}

// TruncateText 截断文本到指定长度
func TruncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// SanitizeInput 清理输入文本
func SanitizeInput(text string) string {
	// 移除不可见字符
	text = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)

	// 规范化空白字符
	text = strings.TrimSpace(text)
	return text
}

// ExtractLanguageCode 从完整的语言代码中提取基础语言代码
// 例如: "zh-CN" -> "ZH"
func ExtractLanguageCode(code string) string {
	parts := strings.Split(code, "-")
	return strings.ToUpper(parts[0])
}

// ApplyPromptTemplate replaces placeholders in the prompt template with actual values.
//
// Supported placeholders:
//   - {{input}}        → the input text
//   - {{source_lang}}  → the source language
//   - {{target_lang}}  → the target language
//
// If the template does not contain {{input}}, it is considered invalid,
// and a default fallback template will be used instead.
func ApplyPromptTemplate(template, input, sourceLang, targetLang string) (string, error) {
	// Validate the template: must contain {{input}} to be meaningful
	if !strings.Contains(template, "{{input}}") {
		return "", errors.New("Invalid prompt template: must contain {{input}}")
	}

	replacer := strings.NewReplacer(
		"{{input}}", input,
		"{{source_lang}}", sourceLang,
		"{{target_lang}}", targetLang,
	)

	return replacer.Replace(template), nil
}

func GetLanguageName(langCode string) (string, error) {
	// 解析语言代码
	tag, err := language.Parse(langCode)
	if err != nil {
		return langCode, fmt.Errorf("invalid language code: %w", err)
	}

	// 使用中文本地化对象
	display := display.Tags(language.English)

	// 获取 tag 的中文名称
	return display.Name(tag), nil
}
