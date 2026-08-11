package logger

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger provides zero-overhead toggleable structured logging.
// When disabled, all log methods return immediately after an atomic.Bool check (~1ns).
type Logger struct {
	enabled      atomic.Bool
	logRequests  atomic.Bool
	logResponses atomic.Bool
	logRetries   atomic.Bool
	maxBodySize  int
	sanitizer    *Sanitizer

	mu     sync.Mutex
	writer *lumberjack.Logger
	slog   *slog.Logger
}

// New creates a Logger from config. Initially applies the enabled flags.
func New(enabled, logReq, logResp, logRetry bool, maxBodySize int, filePath string, maxFileSize, maxFiles int) *Logger {
	l := &Logger{
		maxBodySize: maxBodySize,
		sanitizer:   NewSanitizer(maxBodySize),
	}
	l.enabled.Store(enabled)
	l.logRequests.Store(logReq)
	l.logResponses.Store(logResp)
	l.logRetries.Store(logRetry)

	l.writer = &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    maxFileSize, // MB
		MaxBackups: maxFiles,
		Compress:   false,
	}
	l.slog = slog.New(slog.NewJSONHandler(l.writer, nil))

	return l
}

// Update reconfigures logging flags (for hot-reload).
func (l *Logger) Update(enabled, logReq, logResp, logRetry bool) {
	l.enabled.Store(enabled)
	l.logRequests.Store(logReq)
	l.logResponses.Store(logResp)
	l.logRetries.Store(logRetry)
}

// LogRequest logs an incoming request (if enabled).
func (l *Logger) LogRequest(reqID string, req *http.Request, body []byte) {
	if !l.enabled.Load() || !l.logRequests.Load() {
		return // ~1ns fast path
	}

	entry := map[string]interface{}{
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"request_id": reqID,
		"event":      "request",
		"method":     req.Method,
		"url":        req.URL.String(),
	}

	// Sanitize headers
	headers := make(map[string][]string)
	for k, vals := range req.Header {
		headers[k] = vals
	}
	entry["headers"] = l.sanitizer.SanitizeHeaders(headers)

	// Body (truncated & sanitized)
	if body != nil {
		entry["body"] = l.sanitizer.SanitizeBody(body)
		entry["body_size"] = len(body)
	}

	l.writeJSON(entry)
}

// LogResponse logs a response (if enabled).
func (l *Logger) LogResponse(reqID string, attempt int, resp *http.Response, bodySize int, elapsed time.Duration) {
	if !l.enabled.Load() || !l.logResponses.Load() {
		return
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	entry := map[string]interface{}{
		"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
		"request_id":       reqID,
		"event":            "response",
		"attempt":          attempt,
		"success":          success,
		"status_code":      resp.StatusCode,
		"response_body_size": bodySize,
		"total_elapsed_ms": elapsed.Milliseconds(),
	}

	l.writeJSON(entry)
}

// LogRetry logs a retry event (if enabled).
func (l *Logger) LogRetry(reqID string, attempt int, ruleName string, statusCode int, responseBody []byte, nextDelay time.Duration) {
	if !l.enabled.Load() || !l.logRetries.Load() {
		return
	}

	entry := map[string]interface{}{
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
		"request_id":    reqID,
		"event":         "retry_triggered",
		"attempt":       attempt,
		"trigger_rule":  ruleName,
		"response_code": statusCode,
		"next_delay_ms": nextDelay.Milliseconds(),
	}

	if responseBody != nil {
		entry["response_body"] = l.sanitizer.SanitizeBody(responseBody)
	}

	l.writeJSON(entry)
}

// LogRetryExhausted logs when all retry attempts are exhausted.
func (l *Logger) LogRetryExhausted(reqID string, attempts int, ruleName string, totalDelay time.Duration) {
	if !l.enabled.Load() || !l.logRetries.Load() {
		return
	}

	entry := map[string]interface{}{
		"timestamp":       time.Now().UTC().Format(time.RFC3339Nano),
		"request_id":      reqID,
		"event":           "retry_exhausted",
		"attempts":        attempts,
		"trigger_rule":    ruleName,
		"total_delay_ms":  totalDelay.Milliseconds(),
	}

	l.writeJSON(entry)
}

// Enabled returns whether logging is currently enabled.
func (l *Logger) Enabled() bool {
	return l.enabled.Load()
}

// Close flushes and closes the log writer.
func (l *Logger) Close() error {
	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

// writeJSON writes a JSON entry to the log file.
func (l *Logger) writeJSON(entry map[string]interface{}) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer.Write(append(data, '\n'))
}

// Ensure lumberjack.Logger implements io.Writer
var _ io.Writer = (*lumberjack.Logger)(nil)

// Ensure we can create os.File for stdout mode
var _ = os.Stdout
