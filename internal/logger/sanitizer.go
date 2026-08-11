package logger

import (
	"regexp"
	"strings"
)

var (
	// Patterns for sensitive headers to sanitize
	authPattern    = regexp.MustCompile(`(?i)^(authorization|api-key|apikey|x-api-key)$`)
	bearerPattern  = regexp.MustCompile(`(?i)(bearer\s+)\S+`)
	skPattern      = regexp.MustCompile(`(?i)(sk-)\S+`)
)

// Sanitizer redacts sensitive data in headers and body text.
type Sanitizer struct {
	maxBodySize int
}

// NewSanitizer creates a sanitizer with the given max body size.
func NewSanitizer(maxBodySize int) *Sanitizer {
	return &Sanitizer{maxBodySize: maxBodySize}
}

// SanitizeHeaders returns a copy of headers with sensitive values redacted.
func (s *Sanitizer) SanitizeHeaders(headers map[string][]string) map[string][]string {
	result := make(map[string][]string, len(headers))
	for k, vals := range headers {
		if authPattern.MatchString(k) {
			redacted := make([]string, len(vals))
			for i, v := range vals {
				redacted[i] = s.sanitizeAuthValue(v)
			}
			result[k] = redacted
		} else {
			result[k] = vals
		}
	}
	return result
}

// SanitizeBody truncates and redacts the body string.
func (s *Sanitizer) SanitizeBody(body []byte) string {
	text := string(body)
	if len(text) > s.maxBodySize {
		text = text[:s.maxBodySize]
	}
	// Redact bearer tokens
	text = bearerPattern.ReplaceAllString(text, "${1}***")
	// Redact sk- keys
	text = skPattern.ReplaceAllString(text, "${1}***")
	return text
}

// sanitizeAuthValue redacts an auth header value.
func (s *Sanitizer) sanitizeAuthValue(val string) string {
	val = bearerPattern.ReplaceAllString(val, "${1}sk-***")
	val = skPattern.ReplaceAllString(val, "${1}***")
	// For non-bearer auth, just show the scheme
	if !strings.Contains(val, "***") && len(val) > 8 {
		return val[:4] + "****"
	}
	return val
}
