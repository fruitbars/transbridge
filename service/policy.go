package service

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type translationDecision string

const (
	decisionModel translationDecision = "model"
	decisionSkip  translationDecision = "skip"
	decisionDict  translationDecision = "dictionary"
)

type translationPolicyResult struct {
	Decision translationDecision
	Output   string
	Reason   string
}

type translationPolicyMode int

const (
	policyModeGeneral translationPolicyMode = iota
	policyModeConservative
)

var (
	citationPattern        = regexp.MustCompile(`^\s*(\[\d+([,\-–]\s*\d+)*\]|\d+\s*[\-–]\s*\d+)\s*$`)
	numericPattern         = regexp.MustCompile(`^\s*[<>≤≥±+\-−]?\s*[\d,.]+(\s*[\-–]\s*[<>≤≥±+\-−]?\s*[\d,.]+)?\s*([%‰])?\s*$`)
	urlPattern             = regexp.MustCompile(`(?i)^\s*(https?://|www\.)\S+\s*$`)
	emailPattern           = regexp.MustCompile(`(?i)^\s*[^@\s]+@[^@\s]+\.[^@\s]+\s*$`)
	speciesPattern         = regexp.MustCompile(`^\s*[A-Z]\.\s+[a-z][a-z-]+(\s+[a-z][a-z-]+)?\s*$`)
	chemicalPattern        = regexp.MustCompile(`^\s*(?:[A-Z][a-z]?\d*){2,}(?:[·.](?:[A-Z][a-z]?\d*)+)*(\^?\d*[+\-−])?\s*$`)
	chargedIonPattern      = regexp.MustCompile(`^\s*(?:[A-Z][a-z]?\d*)+(\^?\d+)?[+\-−]\s*$`)
	compoundUnitPattern    = regexp.MustCompile(`(?i)^\s*[µμa-z]+(\^?[+\-−]?\d+)?(\s*[/.·]\s*[µμa-z]+(\^?[+\-−]?\d+)?|\s+[µμa-z]+(\^?[+\-−]?\d+|[+\-−]\d*))+\s*$`)
	geneProteinPattern     = regexp.MustCompile(`^\s*([A-Z]{2,}[A-Z0-9-]*\d+|[a-z]\d+|[A-Z]{1,5}-[A-Za-z0-9α-ωΑ-Ω]+|ATCC\s+\d+)\s*$`)
	dateMoneyPattern       = regexp.MustCompile(`(?i)^\s*(\d{4}[-/]\d{1,2}[-/]\d{1,2}|Q[1-4]|FY\d{2,4}|[$€¥£]\s*[\d,.]+|[A-Z]{3}\s+[\d,.]+)\s*$`)
	romanPattern           = regexp.MustCompile(`^\s*[IVXLCDM]+\s*$`)
	acronymPattern         = regexp.MustCompile(`^\s*[A-Z]{2,5}\s*$`)
	singleASCIIWordPattern = regexp.MustCompile(`^[A-Za-z]+$`)
)

var skipUnits = map[string]bool{
	"kg": true, "g": true, "mg": true, "ug": true, "µg": true, "μg": true,
	"m": true, "cm": true, "mm": true, "km": true, "ha": true,
	"l": true, "ml": true, "mol": true, "s": true, "ms": true, "h": true,
	"yr": true, "year": true, "years": true, "db": true, "hz": true,
}

var missingValueTokens = map[string]bool{
	"na": true, "n/a": true, "nil": true, "null": true, "-": true, "--": true,
}

var zhCommonTerms = map[string]string{
	"text": "文本", "total": "总计", "subtotal": "小计", "sum": "合计",
	"count": "计数", "average": "平均值", "mean": "均值",
	"minimum": "最小值", "min": "最小值", "maximum": "最大值", "max": "最大值",
	"name": "名称", "value": "值", "unit": "单位", "date": "日期",
	"description": "描述", "high": "高", "medium": "中", "low": "低",
	"control": "对照", "treatment": "处理", "case": "病例", "normal": "正常",
	"yes": "是", "no": "否", "group": "组", "area": "面积", "year": "年份",
}

func applyTranslationPolicy(text, targetLang string) translationPolicyResult {
	return applyTranslationPolicyWithMode(text, targetLang, policyModeGeneral)
}

func applyTranslationPolicyWithMode(text, targetLang string, mode translationPolicyMode) translationPolicyResult {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return translationPolicyResult{Decision: decisionSkip, Output: text, Reason: "blank"}
	}
	if shouldSkipTranslationText(trimmed) {
		return translationPolicyResult{Decision: decisionSkip, Output: text, Reason: "non_linguistic"}
	}
	if translated, ok := translateCommonTerm(trimmed, targetLang); ok {
		return translationPolicyResult{Decision: decisionDict, Output: preserveOuterSpace(text, trimmed, translated), Reason: "common_term"}
	}
	if mode == policyModeConservative && singleASCIIWordPattern.MatchString(trimmed) && utf8.RuneCountInString(trimmed) <= 12 {
		return translationPolicyResult{Decision: decisionSkip, Output: text, Reason: "single_word_without_dictionary"}
	}
	return translationPolicyResult{Decision: decisionModel}
}

func shouldSkipTranslationText(text string) bool {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if citationPattern.MatchString(trimmed) ||
		numericPattern.MatchString(trimmed) ||
		urlPattern.MatchString(trimmed) ||
		emailPattern.MatchString(trimmed) ||
		speciesPattern.MatchString(trimmed) ||
		chemicalPattern.MatchString(trimmed) ||
		chargedIonPattern.MatchString(trimmed) ||
		compoundUnitPattern.MatchString(trimmed) ||
		geneProteinPattern.MatchString(trimmed) ||
		dateMoneyPattern.MatchString(trimmed) ||
		romanPattern.MatchString(trimmed) {
		return true
	}
	if skipUnits[lower] || missingValueTokens[lower] {
		return true
	}
	if utf8.RuneCountInString(trimmed) == 1 {
		return true
	}
	if acronymPattern.MatchString(trimmed) {
		return true
	}
	if isOnlySymbols(trimmed) {
		return true
	}
	return false
}

func translateCommonTerm(text, targetLang string) (string, bool) {
	if !isChineseTarget(targetLang) {
		return "", false
	}
	value, ok := zhCommonTerms[strings.ToLower(strings.TrimSpace(text))]
	return value, ok
}

func isChineseTarget(targetLang string) bool {
	lang := strings.ToLower(targetLang)
	return lang == "zh" || lang == "cn" || strings.HasPrefix(lang, "zh-")
}

func isOnlySymbols(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func preserveOuterSpace(original, trimmed, replacement string) string {
	start := strings.Index(original, trimmed)
	if start < 0 {
		return replacement
	}
	return original[:start] + replacement + original[start+len(trimmed):]
}
