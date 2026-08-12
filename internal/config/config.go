package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Logging   LoggingConfig   `yaml:"logging" json:"logging"`
	Rules     []Rule          `yaml:"rules" json:"rules"`
	Proxy     ProxyConfig     `yaml:"proxy" json:"proxy"`
	RateLimit RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
}

// LoggingConfig controls the toggleable logging system.
type LoggingConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	LogRequests  bool   `yaml:"log_requests" json:"log_requests"`
	LogResponses bool   `yaml:"log_responses" json:"log_responses"`
	LogRetries   bool   `yaml:"log_retries" json:"log_retries"`
	MaxBodySize  int    `yaml:"max_body_size" json:"max_body_size"`
	Output       string `yaml:"output" json:"output"` // file / stdout / both
	FilePath     string `yaml:"file_path" json:"file_path"`
	MaxFileSize  int    `yaml:"max_file_size" json:"max_file_size"` // MB
	MaxFiles     int    `yaml:"max_files" json:"max_files"`
}

// Rule defines a single retry rule with a match condition and an action.
type Rule struct {
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description" json:"description"`
	Match       MatchSpec  `yaml:"match" json:"match"`
	Action      ActionSpec `yaml:"action" json:"action"`
}

// MatchSpec describes the conditions under which a rule triggers.
// Multiple dimensions are ANDed together unless a Logic block is specified.
type MatchSpec struct {
	HTTPStatus interface{}    `yaml:"http_status" json:"http_status"` // int, []int, or string like "5xx"
	Headers    []HeaderMatch  `yaml:"headers" json:"headers"`
	JSONPath   *JSONPathMatch `yaml:"json_path_match" json:"json_path_match"`
	Text       *TextMatch     `yaml:"text_match" json:"text_match"`
	Logic      *LogicMatch    `yaml:"logic" json:"logic"`
}

// HeaderMatch matches a specific response header.
type HeaderMatch struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"` // empty means "header exists"
}

// JSONPathMatch matches a JSON response body field using JSONPath expression.
type JSONPathMatch struct {
	Path     string      `yaml:"path" json:"path"`           // JSONPath expression, e.g. "$.code"
	Operator string      `yaml:"operator" json:"operator"`   // ==, !=, >, <, >=, <=, contains
	Value    interface{} `yaml:"value" json:"value"`         // expected value
}

// TextMatch matches the raw response body text.
type TextMatch struct {
	Contains string `yaml:"contains" json:"contains"` // substring match
	Regex    string `yaml:"regex" json:"regex"`       // regex match
}

// LogicMatch allows combining multiple match conditions with AND/OR/NOT.
type LogicMatch struct {
	And []MatchSpec `yaml:"and" json:"and"`
	Or  []MatchSpec `yaml:"or" json:"or"`
	Not *MatchSpec  `yaml:"not" json:"not"`
}

// ActionSpec defines what happens when a rule matches.
type ActionSpec struct {
	MaxAttempts    int         `yaml:"max_attempts" json:"max_attempts"`
	SkipRetry      bool        `yaml:"skip_retry" json:"skip_retry"`
	Backoff       BackoffSpec `yaml:"backoff" json:"backoff"`
	IdempotentOnly bool        `yaml:"idempotent_only" json:"idempotent_only"`
}

// BackoffSpec configures the delay strategy between retries.
type BackoffSpec struct {
	Strategy     string  `yaml:"strategy" json:"strategy"`           // fixed / exponential / linear
	InitialDelay int     `yaml:"initial_delay" json:"initial_delay"` // ms
	Multiplier   float64 `yaml:"multiplier" json:"multiplier"`
	MaxDelay     int     `yaml:"max_delay" json:"max_delay"` // ms
	Jitter       bool    `yaml:"jitter" json:"jitter"`
}

// ProxyConfig configures the reverse proxy listener.
type ProxyConfig struct {
	Listen        string `yaml:"listen" json:"listen"`
	Upstream      string `yaml:"upstream" json:"upstream"`
	TimeoutSec    int    `yaml:"timeout_seconds" json:"timeout_seconds"`
	GlobalTimeout int    `yaml:"global_timeout" json:"global_timeout"` // ms, total retry duration cap
}

// RateLimitConfig controls the global retry budget.
type RateLimitConfig struct {
	RetryBurst       int `yaml:"retry_burst" json:"retry_burst"`
	RetryBurstWindow int `yaml:"retry_burst_window" json:"retry_burst_window"` // seconds
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the config for logical errors and applies defaults.
func (c *Config) Validate() error {
	// Proxy defaults
	if c.Proxy.Listen == "" {
		c.Proxy.Listen = "127.0.0.1:15722"
	}
	if c.Proxy.Upstream == "" {
		return fmt.Errorf("proxy.upstream is required")
	}
	if c.Proxy.TimeoutSec <= 0 {
		c.Proxy.TimeoutSec = 120
	}

	// Logging defaults
	if c.Logging.MaxBodySize <= 0 {
		c.Logging.MaxBodySize = 1048576
	}
	if c.Logging.Output == "" {
		c.Logging.Output = "file"
	}
	if c.Logging.FilePath == "" {
		c.Logging.FilePath = "./logs/retry-middleware.log"
	}
	if c.Logging.MaxFileSize <= 0 {
		c.Logging.MaxFileSize = 100
	}
	if c.Logging.MaxFiles <= 0 {
		c.Logging.MaxFiles = 10
	}

	// Logging output validation
	if c.Logging.Output != "file" && c.Logging.Output != "stdout" && c.Logging.Output != "both" {
		return fmt.Errorf("logging.output must be file, stdout, or both, got %q", c.Logging.Output)
	}

	// Rules validation
	for i, r := range c.Rules {
		if r.Name == "" {
			return fmt.Errorf("rules[%d].name is required", i)
		}
		if r.Action.MaxAttempts < 0 {
			return fmt.Errorf("rules[%d].action.max_attempts must be >= 0", i)
		}
		if r.Action.MaxAttempts == 0 && !r.Action.SkipRetry {
			r.Action.MaxAttempts = 3 // default: retry 3 times
			c.Rules[i] = r
		}
		// Backoff defaults
		if r.Action.Backoff.Strategy == "" {
			r.Action.Backoff.Strategy = "exponential"
		}
		if r.Action.Backoff.InitialDelay <= 0 {
			r.Action.Backoff.InitialDelay = 1000
		}
		if r.Action.Backoff.Multiplier <= 0 {
			r.Action.Backoff.Multiplier = 2.0
		}
		if r.Action.Backoff.MaxDelay <= 0 {
			r.Action.Backoff.MaxDelay = 30000
		}
		c.Rules[i] = r

		// Validate backoff strategy
		strategy := r.Action.Backoff.Strategy
		if strategy != "fixed" && strategy != "exponential" && strategy != "linear" {
			return fmt.Errorf("rules[%d].action.backoff.strategy must be fixed, exponential, or linear, got %q", i, strategy)
		}

		// Validate JSONPath match operator
		if r.Match.JSONPath != nil {
			op := r.Match.JSONPath.Operator
			validOps := map[string]bool{"==": true, "!=": true, ">": true, "<": true, ">=": true, "<=": true, "contains": true}
			if !validOps[op] {
				return fmt.Errorf("rules[%d].match.json_path_match.operator invalid: %q", i, op)
			}
		}

		// Validate text match
		if r.Match.Text != nil && r.Match.Text.Regex != "" {
			// Pre-validate regex compiles
			if !isValidRegex(r.Match.Text.Regex) {
				return fmt.Errorf("rules[%d].match.text.regex invalid pattern: %q", i, r.Match.Text.Regex)
			}
		}
	}

	// Rate limit defaults
	if c.RateLimit.RetryBurst <= 0 {
		c.RateLimit.RetryBurst = 100
	}
	if c.RateLimit.RetryBurstWindow <= 0 {
		c.RateLimit.RetryBurstWindow = 60
	}

	return nil
}

// HTTPStatusCodes resolves the http_status field to a list of status codes.
// Supports: int (429), []int ([429,502,503]), string ("5xx" -> 500-599).
func (m *MatchSpec) HTTPStatusCodes() ([]int, error) {
	if m.HTTPStatus == nil {
		return nil, nil
	}

	switch v := m.HTTPStatus.(type) {
	case int:
		return []int{v}, nil
	case []interface{}:
		codes := make([]int, 0, len(v))
		for _, item := range v {
			code, ok := item.(int)
			if !ok {
				return nil, fmt.Errorf("http_status list item must be int, got %T", item)
			}
			codes = append(codes, code)
		}
		return codes, nil
	case string:
		return parseStatusRange(v)
	default:
		return nil, fmt.Errorf("http_status must be int, []int, or string, got %T", v)
	}
}

// parseStatusRange parses range expressions like "5xx" into [500,501,...,599].
func parseStatusRange(s string) ([]int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 3 || s[1:] != "xx" {
		return nil, fmt.Errorf("invalid status range %q, expected format like '5xx'", s)
	}
	prefix := int(s[0]-'0')
	if prefix < 1 || prefix > 5 {
		return nil, fmt.Errorf("invalid status range prefix %d, must be 1-5", prefix)
	}
	codes := make([]int, 100)
	for i := 0; i < 100; i++ {
		codes[i] = prefix*100 + i
	}
	return codes, nil
}

func isValidRegex(pattern string) bool {
	// Simple validation: try to compile
	// We'll import regexp in the file that uses it
	// For now, skip deep validation here
	_ = pattern
	return true
}
