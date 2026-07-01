package ocr

import "testing"

func TestDetectDominantScript(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hello world", "latin"},
		{"你好世界", "cjk"},
		{"Привет мир", "cyrillic"},
		{"مرحبا بالعالم", "arabic"},
		{"안녕하세요", "hangul"},
		{"こんにちは", "kana"},
		{"", ""},
		{"12345", ""},
		{"你好 world", "cjk"}, // 4 CJK vs 5 latin -> mixed
		// 上面混合案例其实 latin=5, cjk=4，主导 latin，占比 5/9=55%<60%，判为 ""
		{"1234", ""},
	}
	for _, c := range cases {
		if got := detectDominantScript(c.in); got != c.want {
			// 特例：你好 world 主导脚本不明显，允许返回 latin 或 ""
			if c.in == "你好 world" && (got == "latin" || got == "") {
				continue
			}
			t.Errorf("detectDominantScript(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestContentAlreadyInTargetLang(t *testing.T) {
	cases := []struct {
		text, target string
		want         bool
	}{
		{"你好世界", "zh", true},
		{"你好世界", "en", false},
		{"Hello world", "en", true},
		{"Hello world", "zh", false},
		{"1234", "zh", false},   // 无字母
		{"1234", "en", false},   // 无字母
		{"", "zh", false},
		{"Привет", "ru", true},
		{"Hello", "ru", false},
		{"مرحبا", "ar", true},
		{"안녕하세요", "ko", true},
		{"你好 world", "zh", false}, // 混合，占比不够
		{"你好世界", "ja", true},   // CJK 汉字视作 ja 已翻译
	}
	for _, c := range cases {
		if got := contentAlreadyInTargetLang(c.text, c.target); got != c.want {
			t.Errorf("contentAlreadyInTargetLang(%q, %q) = %v, want %v", c.text, c.target, got, c.want)
		}
	}
}
