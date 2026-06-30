package translator

import "testing"

func TestNormalizeChatCompletionsURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"https://api.openai.com/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1/chat/completions/", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1/", "https://api.openai.com/v1/chat/completions"},
		{"https://maas-api.cn-huabei-1.xf-yun.com/v2", "https://maas-api.cn-huabei-1.xf-yun.com/v2/chat/completions"},
		{"https://host/v1?key=1", "https://host/v1/chat/completions?key=1"},
		{"https://host/v1/chat/completions?key=1", "https://host/v1/chat/completions?key=1"},
	}
	for _, c := range cases {
		if got := normalizeChatCompletionsURL(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
