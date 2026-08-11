package rule

import "regexp"

// MatchSpec mirrors config.MatchSpec for internal use with precompiled data.
type MatchSpec struct {
	HTTPStatusCodes []int
	Headers         []HeaderMatch
	JSONPath        *JSONPathMatch
	Text            *TextMatch
	Logic           *LogicMatch
}

// HeaderMatch matches a response header.
type HeaderMatch struct {
	Name  string
	Value string
}

// JSONPathMatch matches a JSON body field.
type JSONPathMatch struct {
	Path     string
	Operator string
	Value    interface{}
}

// TextMatch matches response body text.
type TextMatch struct {
	Contains string
	Regex    string
	regexComp *regexp.Regexp // precompiled
}

// LogicMatch combines match conditions.
type LogicMatch struct {
	And []MatchSpec
	Or  []MatchSpec
	Not *MatchSpec
}

// CompiledRule is a rule with precompiled match data for fast evaluation.
type CompiledRule struct {
	Name        string
	Description string
	Match       MatchSpec
	Action      ActionSpec
}

// ActionSpec defines the retry action.
type ActionSpec struct {
	MaxAttempts    int
	SkipRetry      bool
	Backoff       BackoffSpec
	IdempotentOnly bool
}

// BackoffSpec configures backoff delay.
type BackoffSpec struct {
	Strategy     string
	InitialDelay int
	Multiplier   float64
	MaxDelay     int
	Jitter       bool
}

// ResponseContext holds the data available for rule matching.
type ResponseContext struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	BodyJSON   interface{} // parsed JSON object, nil if not JSON
}

// CompileRegex precompiles regex patterns in TextMatch.
func (m *TextMatch) CompileRegex() error {
	if m.Regex != "" {
		re, err := regexp.Compile(m.Regex)
		if err != nil {
			return err
		}
		m.regexComp = re
	}
	return nil
}

// MatchResult holds the result of a rule evaluation.
type MatchResult struct {
	Matched bool
	Rule    *CompiledRule
}
