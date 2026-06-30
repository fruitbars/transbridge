package openai

import (
	"testing"

	"transbridge/config"
	"transbridge/translator"
)

func TestUnwrapOpenAITranslatorThroughRateLimiter(t *testing.T) {
	manager, err := translator.NewModelManager([]config.ProviderConfig{
		{
			Provider: "openai",
			APIURL:   "http://localhost:11434/v1/chat/completions",
			APIKey:   "test",
			Timeout:  30,
			RateLimit: config.RateLimitConfig{
				QPS: 1,
			},
			Models: []config.ModelConfig{
				{Name: "dummy", Weight: 1, MaxTokens: 100, Temperature: 0.3},
			},
		},
	})
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}

	model, err := manager.GetModel("openai", "dummy")
	if err != nil {
		t.Fatalf("get model: %v", err)
	}

	openaiTranslator, ok := unwrapOpenAITranslator(model)
	if !ok {
		t.Fatal("expected wrapped OpenAI translator to unwrap")
	}
	if openaiTranslator.GetModel() != "dummy" {
		t.Fatalf("model = %q, want dummy", openaiTranslator.GetModel())
	}
}
