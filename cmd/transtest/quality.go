package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type GoldenCase struct {
	Name       string `yaml:"name"`
	Text       string `yaml:"text"`
	SourceLang string `yaml:"source_lang"`
	TargetLang string `yaml:"target_lang"`
	Expected   string `yaml:"expected"`
	Category   string `yaml:"category"` // "translate", "preserve", etc.
}

type GoldenTestSuite struct {
	Cases []GoldenCase `yaml:"cases"`
}

type QualityResult struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Error    string `json:"error,omitempty"`
}

func runQuality(args []string) {
	fs := flag.NewFlagSet("quality", flag.ExitOnError)
	base := fs.String("base", "http://localhost:8080", "API base URL")
	token := fs.String("token", "", "Auth token")
	timeout := fs.Int("timeout", 60, "Request timeout in seconds")
	golden := fs.String("golden", "testdata/golden.yml", "Golden test cases file")
	update := fs.Bool("update", false, "Update golden file with actual results")
	showDiff := fs.Bool("diff", false, "Show diff for failed cases")
	verbose := fs.Bool("verbose", false, "Verbose output")
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Parse(args)

	// Load golden cases
	data, err := os.ReadFile(*golden)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read golden file: %v\n", err)
		os.Exit(1)
	}

	var suite GoldenTestSuite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse golden file: %v\n", err)
		os.Exit(1)
	}

	if len(suite.Cases) == 0 {
		fmt.Fprintln(os.Stderr, "No test cases found in golden file")
		os.Exit(1)
	}

	client := NewClient(*base, *token, time.Duration(*timeout)*time.Second, *verbose)
	ctx := context.Background()

	if *verbose {
		fmt.Printf("Running %d golden test cases...\n\n", len(suite.Cases))
	}

	results := make([]QualityResult, 0, len(suite.Cases))
	passed := 0

	for i, tc := range suite.Cases {
		req := SimpleTranslateRequest{
			Text:       tc.Text,
			SourceLang: tc.SourceLang,
			TargetLang: tc.TargetLang,
		}

		resp, body, err := client.PostJSON(ctx, "/translate", req)

		result := QualityResult{
			Name:     tc.Name,
			Category: tc.Category,
			Expected: tc.Expected,
		}

		if err != nil {
			result.Error = err.Error()
			result.Passed = false
		} else if resp.StatusCode != 200 {
			result.Error = fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))
			result.Passed = false
		} else {
			var transResp SimpleTranslateResponse
			if err := json.Unmarshal(body, &transResp); err != nil {
				result.Error = fmt.Sprintf("unmarshal: %v", err)
				result.Passed = false
			} else {
				result.Actual = transResp.Data

				// For "preserve" category, check exact match
				// For "translate" category, just check non-empty (allow model variance)
				if tc.Category == "preserve" {
					result.Passed = result.Actual == tc.Expected
				} else {
					// For translation, if we're updating, accept any non-empty result
					// Otherwise compare against expected
					if *update {
						result.Passed = result.Actual != ""
					} else {
						result.Passed = result.Actual == tc.Expected
					}
				}

				if *update {
					suite.Cases[i].Expected = result.Actual
				}
			}
		}

		if result.Passed {
			passed++
			if *verbose {
				fmt.Printf("✓ %s\n", tc.Name)
			}
		} else {
			if *verbose || *showDiff {
				fmt.Printf("✗ %s\n", tc.Name)
				if *showDiff && result.Error == "" {
					fmt.Printf("  Expected: %s\n", result.Expected)
					fmt.Printf("  Actual:   %s\n", result.Actual)
				}
				if result.Error != "" {
					fmt.Printf("  Error: %s\n", result.Error)
				}
			}
		}

		results = append(results, result)
	}

	// Update golden file if requested
	if *update {
		updatedData, err := yaml.Marshal(&suite)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal updated cases: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*golden, updatedData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write golden file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n✓ Updated golden file: %s\n", *golden)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
	} else {
		fmt.Printf("\nQuality Test Results: %d/%d passed\n", passed, len(results))
		if passed < len(results) {
			fmt.Println("\nFailed cases:")
			for _, r := range results {
				if !r.Passed {
					fmt.Printf("  - %s (%s)\n", r.Name, r.Category)
				}
			}
		}
	}

	if passed < len(results) {
		os.Exit(1)
	}
}
