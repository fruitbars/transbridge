package utils

import (
	"crypto/md5"
	"encoding/hex"
	iso639 "github.com/emvi/iso-639-1"
	"strings"
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

	return "transbrige:" + md5string
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
