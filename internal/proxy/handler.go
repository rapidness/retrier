package proxy

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/your-org/proxy-api/internal/config"
	"github.com/your-org/proxy-api/internal/logger"
	"github.com/your-org/proxy-api/internal/metrics"
	"github.com/your-org/proxy-api/internal/middleware"
	"github.com/your-org/proxy-api/internal/retry"
	"github.com/your-org/proxy-api/internal/rule"

	"github.com/prometheus/client_golang/prometheus"
)

// Handler is the core proxy handler that intercepts requests,
// forwards them upstream, evaluates retry rules, and handles
// short-circuit retry.
type Handler struct {
	reverseProxy *httputil.ReverseProxy
	engine       *rule.Engine
	executor     *retry.Executor
	logger       *logger.Logger
	metrics      *metrics.Metrics
	cfg          *config.Config
}

// NewHandler creates a proxy handler from config.
func NewHandler(cfg *config.Config) (*Handler, error) {
	upstreamURL, err := url.Parse(cfg.Proxy.Upstream)
	if err != nil {
		return nil, err
	}

	// Create rule engine
	engine, err := rule.NewEngine(cfg.Rules)
	if err != nil {
		return nil, err
	}

	// Create retry budget
	budget := retry.NewBudget(cfg.RateLimit.RetryBurst, cfg.RateLimit.RetryBurstWindow)
	timeout := time.Duration(cfg.Proxy.TimeoutSec) * time.Second

	// Create logger
	lgr := logger.New(
		cfg.Logging.Enabled,
		cfg.Logging.LogRequests,
		cfg.Logging.LogResponses,
		cfg.Logging.LogRetries,
		cfg.Logging.MaxBodySize,
		cfg.Logging.Output,
		cfg.Logging.FilePath,
		cfg.Logging.MaxFileSize,
		cfg.Logging.MaxFiles,
	)

	// Create metrics
	m := metrics.New()

	h := &Handler{
		engine:   engine,
		executor: retry.NewExecutor(budget, timeout, lgr),
		logger:   lgr,
		metrics:  m,
		cfg:      cfg,
	}

	// Create reverse proxy
	h.reverseProxy = httputil.NewSingleHostReverseProxy(upstreamURL)
	h.reverseProxy.ModifyResponse = h.modifyResponse
	h.reverseProxy.ErrorHandler = h.errorHandler

	return h, nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := middleware.GetRequestID(r)

	// Buffer request body for potential retry
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Log request
	h.logger.LogRequest(reqID, r, bodyBytes)

	// Track active requests
	h.metrics.ActiveRequestsInc()
	defer h.metrics.ActiveRequestsDec()

	// Store body in request context for retry
	ctx := context.WithValue(r.Context(), ctxKeyRequestBody, bodyBytes)
	r = r.WithContext(ctx)

	// Forward to reverse proxy (which will call modifyResponse)
	h.reverseProxy.ServeHTTP(w, r)

	// Record request duration
	h.metrics.RequestDurationObserve(time.Since(start).Seconds())
}

// modifyResponse is the hook called after receiving the upstream response.
// It evaluates retry rules and handles short-circuit retry.
func (h *Handler) modifyResponse(resp *http.Response) error {
	req := resp.Request
	reqID := middleware.GetRequestID(req)
	start := time.Now()

	// Read and buffer response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Build response context for rule matching
	respCtx := &rule.ResponseContext{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       bodyBytes,
	}

	// Evaluate rules
	matchedRule := h.engine.Match(respCtx)

	// No rule matched → return response as-is
	if matchedRule == nil {
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		h.logger.LogResponse(reqID, 1, resp, len(bodyBytes), bodyBytes, time.Since(start))
		return nil
	}

	// Rule matched + skip_retry → return error directly
	if matchedRule.Action.SkipRetry {
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		h.logger.LogResponse(reqID, 1, resp, len(bodyBytes), bodyBytes, time.Since(start))
		h.metrics.RetryByRuleInc(matchedRule.Name)
		return nil
	}

	log.Printf("[proxy] rule=%s matched for request %s, starting retry loop",
		matchedRule.Name, reqID)

	// Rule matched → short-circuit retry
	h.metrics.RetryByRuleInc(matchedRule.Name)
	h.metrics.RetryTotalInc()

	// Get buffered request body
	bodyBytesReq, _ := req.Context().Value(ctxKeyRequestBody).([]byte)

	// Execute retry loop
	globalTimeout := time.Duration(h.cfg.Proxy.GlobalTimeout) * time.Millisecond
	result, err := h.executor.Execute(
		req.Context(),
		req,
		bodyBytesReq,
		matchedRule,
		h.engine,
		globalTimeout,
		reqID,
	)

	if err != nil || result.Exhausted {
		// All retries exhausted → return error to agent
		log.Printf("[proxy] retry exhausted for request %s: %v", reqID, err)
		h.metrics.RetryExhaustedInc()
		h.logger.LogRetryExhausted(reqID, result.Attempts, matchedRule.Name, result.TotalDelay)

		// Return the original error response
		resp.StatusCode = 502
		resp.Status = "502 Bad Gateway"
		resp.Body = io.NopCloser(bytes.NewReader([]byte(`{"error":"retry exhausted","rule":"` + matchedRule.Name + `"}`)))
		return nil
	}

	// Retry succeeded → replace the response with the successful one
	log.Printf("[proxy] retry succeeded for request %s after %d attempts",
		reqID, result.Attempts)
	h.metrics.RetrySuccessInc()

	// Copy the successful response
	resp.StatusCode = result.Response.StatusCode
	resp.Status = result.Response.Status
	resp.Header = result.Response.Header
	resp.Body = result.Response.Body

	// Read response body for logging, then restore
	var retryBodyBytes []byte
	if resp.Body != nil {
		retryBodyBytes, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(retryBodyBytes))
	}
	h.logger.LogResponse(reqID, result.Attempts, resp, len(retryBodyBytes), retryBodyBytes, time.Since(start))

	return nil
}

// errorHandler handles proxy forwarding errors.
func (h *Handler) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.GetRequestID(r)
	log.Printf("[proxy] forwarding error for request %s: %v", reqID, err)
	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}

// UpdateConfig updates the handler's configuration (for hot-reload).
func (h *Handler) UpdateConfig(cfg *config.Config) {
	h.cfg = cfg

	// Update rule engine
	if err := h.engine.UpdateRules(cfg.Rules); err != nil {
		log.Printf("[proxy] failed to update rules: %v", err)
	}

	// Update logger
	h.logger.Update(
		cfg.Logging.Enabled,
		cfg.Logging.LogRequests,
		cfg.Logging.LogResponses,
		cfg.Logging.LogRetries,
		cfg.Logging.MaxBodySize,
	)
}

// Close cleans up resources.
func (h *Handler) Close() {
	h.logger.Close()
}

// MetricsRegistry returns the Prometheus registry for metrics endpoint.
func (h *Handler) MetricsRegistry() *prometheus.Registry {
	return h.metrics.Registry()
}

// context key for request body
type contextKey string

const ctxKeyRequestBody contextKey = "requestBody"
