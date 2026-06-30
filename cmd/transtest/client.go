package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client with convenience methods for testing TransBridge API
type Client struct {
	BaseURL string
	Token   string
	Timeout time.Duration
	client  *http.Client
	Verbose bool
}

func NewClient(baseURL, token string, timeout time.Duration, verbose bool) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		Timeout: timeout,
		Verbose: verbose,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) PostJSON(ctx context.Context, endpoint string, payload interface{}) (*http.Response, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal payload: %w", err)
	}

	url := c.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	if c.Verbose {
		fmt.Printf("[%s] POST %s\n", time.Now().Format("15:04:05.000"), endpoint)
	}

	start := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	elapsed := time.Since(start)

	if c.Verbose {
		fmt.Printf("[%s] %d %s (%.0fms)\n", time.Now().Format("15:04:05.000"), resp.StatusCode, resp.Status, elapsed.Seconds()*1000)
	}

	if err != nil {
		return resp, nil, fmt.Errorf("read response: %w", err)
	}

	return resp, respBody, nil
}
