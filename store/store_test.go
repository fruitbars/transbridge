package store

import (
	"context"
	"path/filepath"
	"testing"

	"transbridge/config"
)

func TestBootstrapFromConfigPersistsAdminData(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "transbridge.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{
		Providers: []config.ProviderConfig{
			{
				Provider:  "openai",
				APIURL:    "http://localhost:11434/v1/chat/completions",
				APIKey:    "test",
				Timeout:   30,
				IsDefault: true,
				Models: []config.ModelConfig{
					{Name: "llama3.1", Weight: 2, MaxTokens: 1000, Temperature: 0.3},
				},
			},
		},
		Prompt:   config.PromptConfig{Template: "Translate {{input}}"},
		TransAPI: config.TransAPI{Tokens: []string{"tr-test"}},
	}

	if err := st.BootstrapFromConfig(context.Background(), cfg); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	providers, err := st.LoadProviders(context.Background())
	if err != nil {
		t.Fatalf("load providers: %v", err)
	}
	if len(providers) != 1 || len(providers[0].Models) != 1 {
		t.Fatalf("providers = %#v, want one provider with one model", providers)
	}

	allowed, err := st.TokenAllowed(context.Background(), "tr-test", "translate")
	if err != nil {
		t.Fatalf("token allowed: %v", err)
	}
	if !allowed {
		t.Fatal("expected bootstrapped token to be allowed")
	}

	prompt, err := st.ActivePrompt(context.Background(), "")
	if err != nil {
		t.Fatalf("active prompt: %v", err)
	}
	if prompt != "Translate {{input}}" {
		t.Fatalf("prompt = %q", prompt)
	}
}

func TestUpsertProviderModelKeepsExistingAPIKeyWhenBlank(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "transbridge.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	provider := config.ProviderConfig{
		Provider: "openai",
		APIURL:   "http://localhost/v1/chat/completions",
		APIKey:   "secret-key",
		Timeout:  30,
	}
	model := config.ModelConfig{Name: "m1", Weight: 1, MaxTokens: 100, Temperature: 0.3}
	if err := st.UpsertProviderModel(ctx, provider, model, true); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}
	provider.APIKey = ""
	model.Weight = 5
	if err := st.UpsertProviderModel(ctx, provider, model, true); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	providers, err := st.LoadProviders(ctx)
	if err != nil {
		t.Fatalf("load providers: %v", err)
	}
	if providers[0].APIKey != "secret-key" {
		t.Fatalf("api key = %q, want preserved secret-key", providers[0].APIKey)
	}

	views, err := st.ListModelViews(ctx)
	if err != nil {
		t.Fatalf("list model views: %v", err)
	}
	if views[0].APIKey == "secret-key" || views[0].APIKey == "" {
		t.Fatalf("masked api key = %q, want non-empty masked value", views[0].APIKey)
	}
}

func TestListTokenViewsMasksToken(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "transbridge.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := st.CreateToken(context.Background(), Token{Name: "client", Token: "tr-very-secret-token", Scope: "translate", Enabled: true}); err != nil {
		t.Fatalf("create token: %v", err)
	}
	views, err := st.ListTokenViews(context.Background())
	if err != nil {
		t.Fatalf("list token views: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("views len = %d, want 1", len(views))
	}
	if views[0].Token == "tr-very-secret-token" || views[0].Token == "" {
		t.Fatalf("masked token = %q, want non-empty masked value", views[0].Token)
	}
}
