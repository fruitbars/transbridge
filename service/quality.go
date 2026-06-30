package service

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	explanatoryCacheRejectPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\byou (have not|haven't|did not|didn't) (provided|provide|include|give)\b`),
		regexp.MustCompile(`(?i)\bplease (paste|provide|send|share|include) (the )?(actual )?(text|content|document)\b`),
		regexp.MustCompile(`(?i)\bI (can't|cannot|can not|am unable to) translate\b`),
		regexp.MustCompile(`(?i)\bthere is no (text|content) to translate\b`),
		regexp.MustCompile(`(?i)\bnot (performing|doing) (a )?translation\b`),
		regexp.MustCompile(`(?i)\blooks like (a )?(proper noun|title|heading)\b`),
		regexp.MustCompile(`(?i)\bauto-?detected language\b`),
		regexp.MustCompile(`(?i)\bfrom auto-?detected language to\b`),
		regexp.MustCompile(`(?i)\bthe translation is\b`),
		regexp.MustCompile(`(?i)\bI'd be happy to help\b`),
		regexp.MustCompile(`(?i)\btherefore\b`),
		regexp.MustCompile(`(?i)\bmeaning of\b`),
		regexp.MustCompile(`您提供的文本`),
		regexp.MustCompile(`您要求翻译`),
		regexp.MustCompile(`意思是`),
		regexp.MustCompile(`可以翻译为`),
		regexp.MustCompile(`翻译成简体中文为`),
	}
)

func shouldCacheTranslation(sourceText, translatedText string) bool {
	source := strings.TrimSpace(sourceText)
	translated := strings.TrimSpace(translatedText)
	if source == "" || translated == "" {
		return false
	}

	normalized := normalizeQualityText(translated)
	for _, pattern := range explanatoryCacheRejectPatterns {
		if pattern.MatchString(normalized) {
			return false
		}
	}

	if looksLikeFragmentedAssistantMessage(translated, normalized) {
		return false
	}

	return true
}

func normalizeQualityText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "-\n", "")
	return strings.Join(strings.Fields(value), " ")
}

func looksLikeFragmentedAssistantMessage(raw, normalized string) bool {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	nonEmpty := 0
	shortLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		if utf8.RuneCountInString(line) <= 12 {
			shortLines++
		}
	}

	if nonEmpty >= 8 && shortLines*100/nonEmpty >= 70 {
		lower := strings.ToLower(normalized)
		if strings.Contains(lower, "translate") || strings.Contains(lower, "provided") || strings.Contains(lower, "please") {
			return true
		}
	}
	return false
}
