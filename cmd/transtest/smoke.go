package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type SmokeResult struct {
	Endpoint string  `json:"endpoint"`
	Status   int     `json:"status"`
	Success  bool    `json:"success"`
	Latency  float64 `json:"latency_ms"`
	Error    string  `json:"error,omitempty"`
	Response string  `json:"response,omitempty"`
}

func runSmoke(args []string) {
	fs := flag.NewFlagSet("smoke", flag.ExitOnError)
	base := fs.String("base", "http://localhost:8080", "API base URL")
	token := fs.String("token", "", "Auth token")
	timeout := fs.Int("timeout", 60, "Request timeout in seconds")
	verbose := fs.Bool("verbose", false, "Verbose output")
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Parse(args)

	client := NewClient(*base, *token, time.Duration(*timeout)*time.Second, *verbose)
	ctx := context.Background()

	tests := []struct {
		name     string
		endpoint string
		run      func() (string, error)
	}{
		{
			name:     "/translate",
			endpoint: "/translate",
			run: func() (string, error) {
				req := SimpleTranslateRequest{
					Text:       "Hello, world!",
					SourceLang: "en",
					TargetLang: "zh",
				}
				resp, body, err := client.PostJSON(ctx, "/translate", req)
				if err != nil {
					return "", err
				}
				if resp.StatusCode != 200 {
					return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
				}
				var result SimpleTranslateResponse
				if err := json.Unmarshal(body, &result); err != nil {
					return "", fmt.Errorf("unmarshal: %w", err)
				}
				if result.Data == "" {
					return "", fmt.Errorf("empty translation")
				}
				return result.Data, nil
			},
		},
		{
			name:     "/deepl/v2/translate",
			endpoint: "/deepl/v2/translate",
			run: func() (string, error) {
				req := DeepLTranslateRequest{
					Text:       []string{"Good morning"},
					SourceLang: "en",
					TargetLang: "zh",
				}
				resp, body, err := client.PostJSON(ctx, "/deepl/v2/translate", req)
				if err != nil {
					return "", err
				}
				if resp.StatusCode != 200 {
					return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
				}
				var result DeepLTranslateResponse
				if err := json.Unmarshal(body, &result); err != nil {
					return "", fmt.Errorf("unmarshal: %w", err)
				}
				if len(result.Translations) == 0 || result.Translations[0].Text == "" {
					return "", fmt.Errorf("empty translation")
				}
				return result.Translations[0].Text, nil
			},
		},
		{
			name:     "/immersivel",
			endpoint: "/immersivel",
			run: func() (string, error) {
				req := BatchTranslateRequest{
					SourceLang: "en",
					TargetLang: "zh",
					TextList:   []string{"Thank you"},
				}
				resp, body, err := client.PostJSON(ctx, "/immersivel", req)
				if err != nil {
					return "", err
				}
				if resp.StatusCode != 200 {
					return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
				}
				var result BatchTranslateResponse
				if err := json.Unmarshal(body, &result); err != nil {
					return "", fmt.Errorf("unmarshal: %w", err)
				}
				if len(result.Translations) == 0 || result.Translations[0].Text == "" {
					return "", fmt.Errorf("empty translation")
				}
				return result.Translations[0].Text, nil
			},
		},
		{
			name:     "/v1/chat/completions",
			endpoint: "/v1/chat/completions",
			run: func() (string, error) {
				req := OpenAIChatRequest{
					Model: "openai/random",
					Messages: []OpenAIChatMessage{
						{Role: "user", Content: "Translate to Chinese: Hi there"},
					},
				}
				resp, body, err := client.PostJSON(ctx, "/v1/chat/completions", req)
				if err != nil {
					return "", err
				}
				if resp.StatusCode != 200 {
					return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
				}
				var result OpenAIChatResponse
				if err := json.Unmarshal(body, &result); err != nil {
					return "", fmt.Errorf("unmarshal: %w", err)
				}
				if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
					return "", fmt.Errorf("empty response")
				}
				return result.Choices[0].Message.Content, nil
			},
		},
	}

	results := make([]SmokeResult, len(tests))
	passed := 0

	for i, test := range tests {
		start := time.Now()
		output, err := test.run()
		latency := time.Since(start).Seconds() * 1000

		result := SmokeResult{
			Endpoint: test.name,
			Latency:  latency,
		}

		if err != nil {
			result.Success = false
			result.Error = err.Error()
			if *verbose {
				fmt.Fprintf(os.Stderr, "✗ %s: %v\n", test.name, err)
			}
		} else {
			result.Success = true
			result.Response = output
			passed++
			if *verbose {
				fmt.Printf("✓ %s: %s (%.0fms)\n", test.name, output, latency)
			}
		}

		results[i] = result
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
	} else {
		fmt.Printf("\nSmoke Test Results: %d/%d passed\n", passed, len(tests))
		for _, r := range results {
			status := "✓"
			if !r.Success {
				status = "✗"
			}
			fmt.Printf("  %s %s (%.0fms)", status, r.Endpoint, r.Latency)
			if !r.Success {
				fmt.Printf(" - %s", r.Error)
			}
			fmt.Println()
		}
	}

	if passed < len(tests) {
		os.Exit(1)
	}
}

