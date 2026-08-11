package retry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/your-org/proxy-api/internal/rule"
)

// Executor handles the retry loop for a matched rule.
type Executor struct {
	budget *Budget
	client *http.Client
}

// NewExecutor creates a retry executor.
func NewExecutor(budget *Budget, timeout time.Duration) *Executor {
	return &Executor{
		budget: budget,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Result holds the final outcome of a retry loop.
type Result struct {
	Response    *http.Response
	Attempts    int
	TotalDelay  time.Duration
	Exhausted   bool   // true if all attempts failed
	MatchedRule string // name of the rule that triggered retry
}

// Execute runs the retry loop for a matched rule.
// The original request and the initial response are provided.
// The rule engine is used to re-evaluate responses after each retry.
func (e *Executor) Execute(
	ctx context.Context,
	origReq *http.Request,
	body []byte, // original request body (buffered for replay)
	matchedRule *rule.CompiledRule,
	engine *rule.Engine,
	globalTimeout time.Duration,
) (*Result, error) {
	result := &Result{
		MatchedRule: matchedRule.Name,
	}

	backoff := NewBackoff(
		matchedRule.Action.Backoff.Strategy,
		matchedRule.Action.Backoff.InitialDelay,
		matchedRule.Action.Backoff.Multiplier,
		matchedRule.Action.Backoff.MaxDelay,
		matchedRule.Action.Backoff.Jitter,
	)

	maxAttempts := matchedRule.Action.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	// Retry loop: attempt 1 = first retry (initial response already failed)
	for attempt := 1; attempt < maxAttempts; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			result.Exhausted = true
			return result, ctx.Err()
		}

		// Check global timeout
		if globalTimeout > 0 && result.TotalDelay >= globalTimeout {
			result.Exhausted = true
			return result, fmt.Errorf("global retry timeout exceeded (%v)", globalTimeout)
		}

		// Check retry budget
		if e.budget != nil && !e.budget.Allow() {
			result.Exhausted = true
			return result, fmt.Errorf("retry budget exceeded")
		}

		// Calculate backoff delay
		delay := backoff.Delay(attempt)
		result.TotalDelay += delay

		log.Printf("[retry] rule=%s attempt=%d/%d delay=%v",
			matchedRule.Name, attempt+1, maxAttempts, delay)

		// Wait before retry
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			result.Exhausted = true
			return result, ctx.Err()
		}

		// Rebuild the request for retry
		retryReq, err := rebuildRequest(origReq, body)
		if err != nil {
			return result, fmt.Errorf("rebuild request: %w", err)
		}

		// Send retry request
		resp, err := e.client.Do(retryReq)
		if err != nil {
			log.Printf("[retry] rule=%s attempt=%d error=%v", matchedRule.Name, attempt+1, err)
			continue
		}

		result.Attempts = attempt + 1
		result.Response = resp

		// Read and buffer response body for rule matching
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[retry] rule=%s attempt=%d read_body_error=%v", matchedRule.Name, attempt+1, err)
			continue
		}

		// Build response context for rule matching
		respCtx := &rule.ResponseContext{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
		}

		// Check if the retry response still matches a retry rule
		matched := engine.Match(respCtx)
		if matched == nil || matched.Action.SkipRetry {
			// No retry rule matched → success! Return this response
			// Restore body for the caller to read
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			return result, nil
		}

		// Still matching a retry rule → continue retrying
		log.Printf("[retry] rule=%s attempt=%d still_matching=%s",
			matchedRule.Name, attempt+1, matched.Name)
	}

	// All attempts exhausted
	result.Attempts = maxAttempts
	result.Exhausted = true
	return result, fmt.Errorf("all %d retry attempts exhausted for rule %q", maxAttempts, matchedRule.Name)
}

// rebuildRequest creates a new HTTP request from the original for retry.
// It constructs a fresh request to avoid issues with RequestURI (which
// cannot be set when using http.Client.Do).
func rebuildRequest(orig *http.Request, body []byte) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	// Build URL from the original request
	reqURL := orig.URL
	if reqURL == nil {
		return nil, fmt.Errorf("original request has no URL")
	}

	req, err := http.NewRequestWithContext(orig.Context(), orig.Method, reqURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	// Copy headers
	for k, vals := range orig.Header {
		req.Header[k] = vals
	}

	if body != nil {
		req.ContentLength = int64(len(body))
	}

	return req, nil
}
