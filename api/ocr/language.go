package ocr

import (
	"strings"
	"unicode"
)

// detectDominantScript 返回 text 里占主导地位的 unicode 脚本类。
// 结果之一：cjk / latin / cyrillic / arabic / hangul / kana / ""（无字母字符）。
// 使用简单占比阈值：主导类必须占字母字符总数的 60% 以上。
func detectDominantScript(text string) string {
	var cjk, latin, cyr, ara, hangul, kana int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			cjk++
		case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
			kana++
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
		case unicode.Is(unicode.Arabic, r):
			ara++
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			latin++
		}
	}
	total := cjk + latin + cyr + ara + hangul + kana
	if total == 0 {
		return ""
	}
	counts := []struct {
		name string
		n    int
	}{{"cjk", cjk}, {"latin", latin}, {"cyrillic", cyr}, {"arabic", ara}, {"hangul", hangul}, {"kana", kana}}
	// 找到最多的一个
	var top string
	var topN int
	for _, c := range counts {
		if c.n > topN {
			top = c.name
			topN = c.n
		}
	}
	// 只有当主导脚本占比 ≥ 60% 才认定
	if topN*10 < total*6 {
		return ""
	}
	return top
}

// targetLangScript 把语言代码映射到主要脚本。未知语言返回 latin（多数罗马化语言）。
func targetLangScript(target string) string {
	lang := strings.ToLower(strings.SplitN(strings.TrimSpace(target), "-", 2)[0])
	switch lang {
	case "zh", "cn", "zh-hans", "zh-hant":
		return "cjk"
	case "ja":
		// 日语混合汉字+假名，两者都算 zh_target 兼容之外
		return "kana"
	case "ko":
		return "hangul"
	case "ru", "uk", "bg", "sr", "be", "mk":
		return "cyrillic"
	case "ar", "fa", "ur":
		return "arabic"
	default:
		return "latin"
	}
}

// contentAlreadyInTargetLang 判断 content 是否已经是 target 语言。
// 简单启发式：content 主导脚本与 target 语言脚本一致。
// 特例：日语 target 时中文汉字也当作命中（日语汉字与中文汉字同 unicode block）。
func contentAlreadyInTargetLang(content, target string) bool {
	if strings.TrimSpace(content) == "" || target == "" {
		return false
	}
	got := detectDominantScript(content)
	if got == "" {
		return false
	}
	want := targetLangScript(target)
	if got == want {
		return true
	}
	// 日语 target 时，主要文本是汉字（cjk）也算已翻译（日语汉字用 Han block）
	if want == "kana" && got == "cjk" {
		return true
	}
	// 中文 target 时，若文本主要是日语汉字，视为已经在 CJK 域内（保守 skip）
	if want == "cjk" && got == "kana" {
		return true
	}
	return false
}
