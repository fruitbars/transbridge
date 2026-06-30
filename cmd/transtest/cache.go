package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type CacheTestResult struct {
	FirstLatency  float64 `json:"first_latency_ms"`
	SecondLatency float64 `json:"second_latency_ms"`
	CacheHit      bool    `json:"cache_hit"`
	Speedup       float64 `json:"speedup_ratio"`
}

func runCache(args []string) {
	fs := flag.NewFlagSet("cache", flag.ExitOnError)
	base := fs.String("base", "http://localhost:8080", "API base URL")
	token := fs.String("token", "", "Auth token")
	timeout := fs.Int("timeout", 60, "Request timeout in seconds")
	verbose := fs.Bool("verbose", false, "Verbose output")
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Parse(args)

	client := NewClient(*base, *token, time.Duration(*timeout)*time.Second, *verbose)
	ctx := context.Background()

	testText := "The quick brown fox jumps over the lazy dog"
	req := SimpleTranslateRequest{
		Text:       testText,
		SourceLang: "en",
		TargetLang: "zh",
	}

	if *verbose {
		fmt.Printf("Testing cache with: %s\n", testText)
	}

	// First request (should miss cache)
	start := time.Now()
	resp1, body1, err := client.PostJSON(ctx, "/translate", req)
	firstLatency := time.Since(start).Seconds() * 1000
	if err != nil {
		fmt.Fprintf(os.Stderr, "First request failed: %v\n", err)
		os.Exit(1)
	}
	if resp1.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "First request status %d: %s\n", resp1.StatusCode, string(body1))
		os.Exit(1)
	}

	var result1 SimpleTranslateResponse
	if err := json.Unmarshal(body1, &result1); err != nil {
		fmt.Fprintf(os.Stderr, "Unmarshal first response: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("First request: %.0fms → %s\n", firstLatency, result1.Data)
	}

	// Wait a moment to ensure async logging completes
	time.Sleep(100 * time.Millisecond)

	// Second request (should hit cache)
	start = time.Now()
	resp2, body2, err := client.PostJSON(ctx, "/translate", req)
	secondLatency := time.Since(start).Seconds() * 1000
	if err != nil {
		fmt.Fprintf(os.Stderr, "Second request failed: %v\n", err)
		os.Exit(1)
	}
	if resp2.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Second request status %d: %s\n", resp2.StatusCode, string(body2))
		os.Exit(1)
	}

	var result2 SimpleTranslateResponse
	if err := json.Unmarshal(body2, &result2); err != nil {
		fmt.Fprintf(os.Stderr, "Unmarshal second response: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Second request: %.0fms → %s\n", secondLatency, result2.Data)
	}

	// Cache hit heuristic: second request should be significantly faster
	// Typically: first ~500-2000ms, cached <50ms
	cacheHit := secondLatency < 100 && secondLatency < firstLatency*0.2
	speedup := firstLatency / secondLatency

	result := CacheTestResult{
		FirstLatency:  firstLatency,
		SecondLatency: secondLatency,
		CacheHit:      cacheHit,
		Speedup:       speedup,
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Println("\nCache Test Results:")
		fmt.Printf("  First request:  %.0fms\n", firstLatency)
		fmt.Printf("  Second request: %.0fms\n", secondLatency)
		fmt.Printf("  Speedup:        %.1fx\n", speedup)
		if cacheHit {
			fmt.Println("  ✓ Cache likely hit (second request <100ms and <20% of first)")
		} else {
			fmt.Println("  ✗ Cache likely missed (second request too slow)")
		}
	}

	if !cacheHit {
		os.Exit(1)
	}
}
