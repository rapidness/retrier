package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/your-org/proxy-api/internal/config"
)

// TestIntegrationShortCircuitRetry tests the core AC-01 scenario:
// API returns {"code": 700} with HTTP 200, middleware should
// internally retry and return the successful result to the client.
func TestIntegrationShortCircuitRetry(t *testing.T) {
	callCount := 0

	// Mock upstream: returns code=700 on first 2 calls, success on 3rd
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			// Return business error (code 700)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 700,
				"msg":  "quota exceeded",
			})
		} else {
			// Return success
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": "result",
			})
		}
	}))
	defer upstream.Close()

	// Create config pointing to mock upstream
	cfg := &config.Config{
		Logging: config.LoggingConfig{
			Enabled:      false,
			MaxBodySize:  10240,
			Output:       "file",
			FilePath:     "./logs/test-integration.log",
			MaxFileSize:  100,
			MaxFiles:     10,
		},
		Rules: []config.Rule{
			{
				Name:        "retry-on-code-700",
				Description: "retry on code 700",
				Match: config.MatchSpec{
					HTTPStatus: 200,
					JSONPath: &config.JSONPathMatch{
						Path:     "$.code",
						Operator: "==",
						Value:    700,
					},
				},
				Action: config.ActionSpec{
					MaxAttempts: 3,
					Backoff: config.BackoffSpec{
						Strategy:     "fixed",
						InitialDelay: 100, // fast for testing
						Multiplier:   2.0,
						MaxDelay:     1000,
						Jitter:       false,
					},
				},
			},
		},
		Proxy: config.ProxyConfig{
			Upstream:      upstream.URL,
			TimeoutSec:    10,
			GlobalTimeout: 30000,
		},
		RateLimit: config.RateLimitConfig{
			RetryBurst:       100,
			RetryBurstWindow: 60,
		},
	}

	// Create proxy handler
	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	// Make request through the proxy
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-req-001")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify: client should receive the success response, NOT the code=700 error
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	code, _ := resp["code"].(float64)
	if code != 0 {
		t.Errorf("Client should see success (code=0), got code=%v", resp["code"])
	}

	// Verify upstream was called 3 times (2 failures + 1 success)
	if callCount != 3 {
		t.Errorf("Expected 3 upstream calls, got %d", callCount)
	}

	t.Logf("Integration test passed: client got success after %d attempts", callCount)
}

// TestIntegrationSkipRetry tests AC-03:
// skip_retry=true should return the error immediately.
func TestIntegrationSkipRetry(t *testing.T) {
	callCount := 0

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		Logging: config.LoggingConfig{
			Enabled:     false,
			MaxBodySize: 10240,
			Output:      "file",
			FilePath:    "./logs/test-skip.log",
			MaxFileSize: 100,
			MaxFiles:    10,
		},
		Rules: []config.Rule{
			{
				Name: "never-retry-400",
				Match: config.MatchSpec{
					HTTPStatus: 400,
				},
				Action: config.ActionSpec{
					MaxAttempts: 1,
					SkipRetry:   true,
				},
			},
		},
		Proxy: config.ProxyConfig{
			Upstream:      upstream.URL,
			TimeoutSec:    10,
			GlobalTimeout: 30000,
		},
		RateLimit: config.RateLimitConfig{
			RetryBurst:       100,
			RetryBurstWindow: 60,
		},
	}

	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should get 400 immediately
	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Should only call upstream once
	if callCount != 1 {
		t.Errorf("Expected 1 upstream call (no retry), got %d", callCount)
	}
}

// TestIntegrationHTTP429Retry tests AC-02:
// HTTP 429 should trigger exponential backoff retry.
func TestIntegrationHTTP429Retry(t *testing.T) {
	callCount := 0

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			w.Write([]byte(`{"error":"rate limited"}`))
		} else {
			w.WriteHeader(200)
			w.Write([]byte(`{"result":"ok"}`))
		}
	}))
	defer upstream.Close()

	cfg := &config.Config{
		Logging: config.LoggingConfig{
			Enabled:     false,
			MaxBodySize: 10240,
			Output:      "file",
			FilePath:    "./logs/test-429.log",
			MaxFileSize: 100,
			MaxFiles:    10,
		},
		Rules: []config.Rule{
			{
				Name: "retry-on-429",
				Match: config.MatchSpec{
					HTTPStatus: 429,
				},
				Action: config.ActionSpec{
					MaxAttempts: 5,
					Backoff: config.BackoffSpec{
						Strategy:     "exponential",
						InitialDelay: 100,
						Multiplier:   2.0,
						MaxDelay:     1000,
						Jitter:       false,
					},
				},
			},
		},
		Proxy: config.ProxyConfig{
			Upstream:      upstream.URL,
			TimeoutSec:    10,
			GlobalTimeout: 30000,
		},
		RateLimit: config.RateLimitConfig{
			RetryBurst:       100,
			RetryBurstWindow: 60,
		},
	}

	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200 after retry, got %d", w.Code)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls (2 x 429 + 1 success), got %d", callCount)
	}
}

// Ensure unused imports are referenced
var _ = fmt.Sprintf
var _ = io.ReadAll
var _ = time.Duration(0)
