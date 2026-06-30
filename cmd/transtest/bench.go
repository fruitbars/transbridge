package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type BenchResult struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessCount    int64   `json:"success_count"`
	FailureCount    int64   `json:"failure_count"`
	Duration        float64 `json:"duration_seconds"`
	RPS             float64 `json:"requests_per_second"`
	AvgLatency      float64 `json:"avg_latency_ms"`
	MinLatency      float64 `json:"min_latency_ms"`
	MaxLatency      float64 `json:"max_latency_ms"`
	P50Latency      float64 `json:"p50_latency_ms"`
	P90Latency      float64 `json:"p90_latency_ms"`
	P95Latency      float64 `json:"p95_latency_ms"`
	P99Latency      float64 `json:"p99_latency_ms"`
	ErrorRate       float64 `json:"error_rate_percent"`
}

func runBench(args []string) {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	base := fs.String("base", "http://localhost:8080", "API base URL")
	token := fs.String("token", "", "Auth token")
	timeout := fs.Int("timeout", 60, "Request timeout in seconds")
	concurrency := fs.Int("c", 10, "Number of concurrent workers")
	duration := fs.String("d", "", "Duration (e.g. 30s, 5m)")
	requests := fs.Int64("n", 0, "Total requests to send")
	rps := fs.Float64("rps", 0, "Target requests per second (0 = unlimited)")
	verbose := fs.Bool("verbose", false, "Verbose output")
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Parse(args)

	if *duration == "" && *requests == 0 {
		fmt.Fprintln(os.Stderr, "Error: must specify either -d (duration) or -n (requests)")
		os.Exit(1)
	}
	if *duration != "" && *requests > 0 {
		fmt.Fprintln(os.Stderr, "Error: -d and -n are mutually exclusive")
		os.Exit(1)
	}

	var durationParsed time.Duration
	var err error
	if *duration != "" {
		durationParsed, err = time.ParseDuration(*duration)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid duration: %v\n", err)
			os.Exit(1)
		}
	}

	client := NewClient(*base, *token, time.Duration(*timeout)*time.Second, false)
	ctx := context.Background()

	fmt.Printf("Starting benchmark:\n")
	fmt.Printf("  Base URL: %s\n", *base)
	fmt.Printf("  Workers: %d\n", *concurrency)
	if *duration != "" {
		fmt.Printf("  Duration: %s\n", *duration)
	} else {
		fmt.Printf("  Total requests: %d\n", *requests)
	}
	if *rps > 0 {
		fmt.Printf("  Target RPS: %.0f\n", *rps)
	}
	fmt.Println()

	var (
		successCount int64
		failureCount int64
		latencies    []float64
		latencyMu    sync.Mutex
		stopChan     = make(chan struct{})
		wg           sync.WaitGroup
	)

	// Rate limiter
	var rateLimiter *RateLimiter
	if *rps > 0 {
		rateLimiter = NewRateLimiter(*rps)
	}

	// Worker function
	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-stopChan:
				return
			default:
			}

			if rateLimiter != nil {
				rateLimiter.Wait()
			}

			req := SimpleTranslateRequest{
				Text:       "Benchmark test message",
				SourceLang: "en",
				TargetLang: "zh",
			}

			start := time.Now()
			resp, _, err := client.PostJSON(ctx, "/translate", req)
			latency := time.Since(start).Seconds() * 1000

			if err != nil || resp.StatusCode != 200 {
				atomic.AddInt64(&failureCount, 1)
				if *verbose {
					fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
				}
			} else {
				atomic.AddInt64(&successCount, 1)
			}

			latencyMu.Lock()
			latencies = append(latencies, latency)
			latencyMu.Unlock()
		}
	}

	// Start workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	// Timer/counter logic
	benchStart := time.Now()
	if *duration != "" {
		// Duration mode
		time.AfterFunc(durationParsed, func() {
			close(stopChan)
		})
	} else {
		// Request count mode
		go func() {
			for {
				total := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&failureCount)
				if total >= *requests {
					close(stopChan)
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}

	// Wait for completion
	wg.Wait()
	benchDuration := time.Since(benchStart).Seconds()

	// Calculate statistics
	totalReqs := successCount + failureCount
	rpsActual := float64(totalReqs) / benchDuration
	errorRate := 0.0
	if totalReqs > 0 {
		errorRate = float64(failureCount) / float64(totalReqs) * 100
	}

	sort.Float64s(latencies)
	avgLatency := 0.0
	minLatency := 0.0
	maxLatency := 0.0
	p50, p90, p95, p99 := 0.0, 0.0, 0.0, 0.0

	if len(latencies) > 0 {
		sum := 0.0
		for _, lat := range latencies {
			sum += lat
		}
		avgLatency = sum / float64(len(latencies))
		minLatency = latencies[0]
		maxLatency = latencies[len(latencies)-1]
		p50 = percentile(latencies, 0.50)
		p90 = percentile(latencies, 0.90)
		p95 = percentile(latencies, 0.95)
		p99 = percentile(latencies, 0.99)
	}

	result := BenchResult{
		TotalRequests:   totalReqs,
		SuccessCount:    successCount,
		FailureCount:    failureCount,
		Duration:        benchDuration,
		RPS:             rpsActual,
		AvgLatency:      avgLatency,
		MinLatency:      minLatency,
		MaxLatency:      maxLatency,
		P50Latency:      p50,
		P90Latency:      p90,
		P95Latency:      p95,
		P99Latency:      p99,
		ErrorRate:       errorRate,
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Println("\nBenchmark Results:")
		fmt.Printf("  Duration:        %.2fs\n", benchDuration)
		fmt.Printf("  Total requests:  %d\n", totalReqs)
		fmt.Printf("  Success:         %d\n", successCount)
		fmt.Printf("  Failures:        %d (%.2f%%)\n", failureCount, errorRate)
		fmt.Printf("  RPS:             %.2f\n", rpsActual)
		fmt.Println("\nLatency:")
		fmt.Printf("  Min:     %.2fms\n", minLatency)
		fmt.Printf("  Avg:     %.2fms\n", avgLatency)
		fmt.Printf("  P50:     %.2fms\n", p50)
		fmt.Printf("  P90:     %.2fms\n", p90)
		fmt.Printf("  P95:     %.2fms\n", p95)
		fmt.Printf("  P99:     %.2fms\n", p99)
		fmt.Printf("  Max:     %.2fms\n", maxLatency)
	}

	if failureCount > 0 {
		os.Exit(1)
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted)) * p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	rate     float64
	interval time.Duration
	ticker   *time.Ticker
	tokens   chan struct{}
}

func NewRateLimiter(rps float64) *RateLimiter {
	interval := time.Duration(float64(time.Second) / rps)
	rl := &RateLimiter{
		rate:     rps,
		interval: interval,
		ticker:   time.NewTicker(interval),
		tokens:   make(chan struct{}, int(rps)),
	}
	go func() {
		for range rl.ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
			}
		}
	}()
	return rl
}

func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

