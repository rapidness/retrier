package logger

import (
	"net/http"
	"testing"
)

func TestSanitizerHeaders(t *testing.T) {
	s := NewSanitizer(10240)

	headers := map[string][]string{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer sk-abc123def456"},
		"X-Api-Key":     {"sk-test-key-123"},
	}

	sanitized := s.SanitizeHeaders(headers)

	// Non-sensitive header should pass through
	if sanitized["Content-Type"][0] != "application/json" {
		t.Error("Content-Type should not be sanitized")
	}

	// Authorization should be redacted
	auth := sanitized["Authorization"][0]
	if auth == "Bearer sk-abc123def456" {
		t.Error("Authorization should be redacted, got raw value")
	}

	// X-Api-Key should be redacted
	apiKey := sanitized["X-Api-Key"][0]
	if apiKey == "sk-test-key-123" {
		t.Error("X-Api-Key should be redacted, got raw value")
	}
}

func TestSanitizerBody(t *testing.T) {
	s := NewSanitizer(10240)

	body := []byte(`{"model":"deepseek-chat","key":"sk-abc123def456","auth":"Bearer sk-test123"}`)
	sanitized := s.SanitizeBody(body)

	if sanitized == string(body) {
		t.Error("Body should be sanitized")
	}
}

func TestSanitizerBodyTruncation(t *testing.T) {
	s := NewSanitizer(10) // very small limit

	body := []byte("this is a very long body that should be truncated")
	sanitized := s.SanitizeBody(body)

	if len(sanitized) > 10 {
		t.Errorf("Body should be truncated to 10 bytes, got %d", len(sanitized))
	}
}

func TestLoggerToggle(t *testing.T) {
	// Test with logging disabled
	l := New(false, true, true, true, 10240, "file", "./logs/test.log", 100, 10)

	// These should be no-ops (no files created)
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	l.LogRequest("test-id", req, nil)
	l.LogResponse("test-id", 1, &http.Response{StatusCode: 200}, 0, nil, 0)
	l.LogRetry("test-id", 1, "test-rule", 429, nil, 0)

	// Verify logging is off
	if l.Enabled() {
		t.Error("Logger should be disabled")
	}

	// Enable logging
	l.Update(true, true, true, true, 10240)
	if !l.Enabled() {
		t.Error("Logger should be enabled after Update")
	}

	// Clean up
	l.Close()
}

func BenchmarkLoggerToggleOff(b *testing.B) {
	l := New(false, true, true, true, 10240, "file", "./logs/bench.log", 100, 10)
	defer l.Close()

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l.LogRequest("bench-id", req, nil)
	}
}

func BenchmarkLoggerToggleOn(b *testing.B) {
	l := New(true, true, true, true, 10240, "file", "./logs/bench-on.log", 100, 10)
	defer l.Close()

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	body := []byte(`{"model":"deepseek-chat"}`)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l.LogRequest("bench-id", req, body)
	}
}
