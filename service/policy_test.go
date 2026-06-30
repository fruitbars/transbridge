package service

import "testing"

func TestTranslationPolicySkipsNonLinguisticText(t *testing.T) {
	cases := []string{
		"95.99%",
		"[7]",
		"P",
		"P. radiata",
		"H2O",
		"NO3-",
		"mg/L",
		"BRCA1",
		"FY2023",
		"III",
	}
	for _, tc := range cases {
		result := applyTranslationPolicy(tc, "zh-CN")
		if result.Decision != decisionSkip {
			t.Fatalf("%q decision = %s, want skip", tc, result.Decision)
		}
	}
}

func TestTranslationPolicyUsesCommonTermDictionary(t *testing.T) {
	result := applyTranslationPolicy(" total ", "zh-CN")
	if result.Decision != decisionDict {
		t.Fatalf("decision = %s, want dictionary", result.Decision)
	}
	if result.Output != " 总计 " {
		t.Fatalf("output = %q, want preserved-space dictionary translation", result.Output)
	}
}

func TestTranslationPolicyTranslatesUnknownSingleWordInGeneralMode(t *testing.T) {
	result := applyTranslationPolicy("In", "zh-CN")
	if result.Decision != decisionModel {
		t.Fatalf("decision = %s, want model", result.Decision)
	}
}

func TestTranslationPolicyKeepsUnknownSingleWordInConservativeMode(t *testing.T) {
	result := applyTranslationPolicyWithMode("In", "zh-CN", policyModeConservative)
	if result.Decision != decisionSkip {
		t.Fatalf("decision = %s, want skip", result.Decision)
	}
}

func TestShouldCacheTranslationRejectsExplanatoryOutput(t *testing.T) {
	bad := "Sec-\nheader，因为在 auto 检测下，这看起来像一个专有名词或标题，不进行翻译。\nyou\nhaven't\nprovided\nthe actual\ntext to\ntranslate."
	if shouldCacheTranslation("Sec-header", bad) {
		t.Fatal("expected explanatory fragmented output to be rejected for cache")
	}
}

func TestShouldCacheTranslationAllowsNormalOutput(t *testing.T) {
	if !shouldCacheTranslation("Mean annual rainfall", "平均年降雨量") {
		t.Fatal("expected normal translation to be cacheable")
	}
}
