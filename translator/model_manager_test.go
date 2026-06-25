package translator

import (
	"sync"
	"testing"
	"transbridge/config"
)

func TestModelManagerDefaultAndListOrderFollowConfig(t *testing.T) {
	mm, err := NewModelManager([]config.ProviderConfig{
		{
			Provider: "openai",
			APIURL:   "http://example.test/v1/chat/completions",
			Models: []config.ModelConfig{
				{Name: "first", Weight: 1},
				{Name: "second", Weight: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}

	if got := mm.GetDefaultModel().GetModel(); got != "first" {
		t.Fatalf("expected first configured model as default, got %q", got)
	}

	models := mm.ListModels()
	if len(models) != 2 || models[0].Model != "first" || models[1].Model != "second" {
		t.Fatalf("models not listed in config order: %+v", models)
	}
}

func TestModelManagerGetRandomModelConcurrent(t *testing.T) {
	mm, err := NewModelManager([]config.ProviderConfig{
		{
			Provider: "openai",
			APIURL:   "http://example.test/v1/chat/completions",
			Models: []config.ModelConfig{
				{Name: "first", Weight: 1},
				{Name: "second", Weight: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("new model manager: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if model := mm.GetRandomModel(); model == nil {
				t.Error("expected random model, got nil")
			}
		}()
	}
	wg.Wait()
}
