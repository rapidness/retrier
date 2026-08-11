package rule

import (
	"fmt"
	"regexp"

	"github.com/your-org/proxy-api/internal/config"
)

// Engine evaluates rules against response contexts.
type Engine struct {
	rules []CompiledRule
}

// NewEngine creates a rule engine from config rules.
func NewEngine(rules []config.Rule) (*Engine, error) {
	compiled := make([]CompiledRule, 0, len(rules))

	for _, r := range rules {
		cr, err := compileRule(r)
		if err != nil {
			return nil, fmt.Errorf("compile rule %q: %w", r.Name, err)
		}
		compiled = append(compiled, cr)
	}

	return &Engine{rules: compiled}, nil
}

// UpdateRules replaces the rule set (for hot-reload).
func (e *Engine) UpdateRules(rules []config.Rule) error {
	compiled := make([]CompiledRule, 0, len(rules))

	for _, r := range rules {
		cr, err := compileRule(r)
		if err != nil {
			return fmt.Errorf("compile rule %q: %w", r.Name, err)
		}
		compiled = append(compiled, cr)
	}

	e.rules = compiled
	return nil
}

// Match evaluates all rules against the response context and returns
// the first matching rule (first-match-wins), or nil if no rule matches.
func (e *Engine) Match(ctx *ResponseContext) *CompiledRule {
	for i := range e.rules {
		ok, err := evaluateMatchSpec(&e.rules[i].Match, ctx)
		if err != nil {
			// Log error but continue to next rule
			continue
		}
		if ok {
			return &e.rules[i]
		}
	}
	return nil
}

// compileRule converts a config.Rule to a CompiledRule with precompiled data.
func compileRule(r config.Rule) (CompiledRule, error) {
	cr := CompiledRule{
		Name:        r.Name,
		Description: r.Description,
		Action: ActionSpec{
			MaxAttempts:    r.Action.MaxAttempts,
			SkipRetry:      r.Action.SkipRetry,
			IdempotentOnly: r.Action.IdempotentOnly,
			Backoff: BackoffSpec{
				Strategy:     r.Action.Backoff.Strategy,
				InitialDelay: r.Action.Backoff.InitialDelay,
				Multiplier:   r.Action.Backoff.Multiplier,
				MaxDelay:     r.Action.Backoff.MaxDelay,
				Jitter:       r.Action.Backoff.Jitter,
			},
		},
	}

	// Resolve HTTP status codes
	codes, err := r.Match.HTTPStatusCodes()
	if err != nil {
		return cr, fmt.Errorf("resolve http_status: %w", err)
	}
	cr.Match.HTTPStatusCodes = codes

	// Compile headers
	cr.Match.Headers = make([]HeaderMatch, len(r.Match.Headers))
	for i, h := range r.Match.Headers {
		cr.Match.Headers[i] = HeaderMatch{Name: h.Name, Value: h.Value}
	}

	// Compile JSONPath match
	if r.Match.JSONPath != nil {
		cr.Match.JSONPath = &JSONPathMatch{
			Path:     r.Match.JSONPath.Path,
			Operator: r.Match.JSONPath.Operator,
			Value:    r.Match.JSONPath.Value,
		}
	}

	// Compile text match
	if r.Match.Text != nil {
		cr.Match.Text = &TextMatch{
			Contains: r.Match.Text.Contains,
			Regex:    r.Match.Text.Regex,
		}
		if r.Match.Text.Regex != "" {
			re, err := regexp.Compile(r.Match.Text.Regex)
			if err != nil {
				return cr, fmt.Errorf("compile regex %q: %w", r.Match.Text.Regex, err)
			}
			cr.Match.Text.regexComp = re
		}
	}

	// Compile logic match
	if r.Match.Logic != nil {
		cr.Match.Logic = compileLogic(r.Match.Logic)
	}

	return cr, nil
}

func compileLogic(lm *config.LogicMatch) *LogicMatch {
	if lm == nil {
		return nil
	}
	result := &LogicMatch{}

	if len(lm.And) > 0 {
		result.And = make([]MatchSpec, len(lm.And))
		for i, sub := range lm.And {
			result.And[i] = compileMatchSpec(sub)
		}
	}

	if len(lm.Or) > 0 {
		result.Or = make([]MatchSpec, len(lm.Or))
		for i, sub := range lm.Or {
			result.Or[i] = compileMatchSpec(sub)
		}
	}

	if lm.Not != nil {
		not := compileMatchSpec(*lm.Not)
		result.Not = &not
	}

	return result
}

func compileMatchSpec(ms config.MatchSpec) MatchSpec {
	result := MatchSpec{}

	codes, _ := ms.HTTPStatusCodes()
	result.HTTPStatusCodes = codes

	result.Headers = make([]HeaderMatch, len(ms.Headers))
	for i, h := range ms.Headers {
		result.Headers[i] = HeaderMatch{Name: h.Name, Value: h.Value}
	}

	if ms.JSONPath != nil {
		result.JSONPath = &JSONPathMatch{
			Path:     ms.JSONPath.Path,
			Operator: ms.JSONPath.Operator,
			Value:    ms.JSONPath.Value,
		}
	}

	if ms.Text != nil {
		result.Text = &TextMatch{
			Contains: ms.Text.Contains,
			Regex:    ms.Text.Regex,
		}
		if ms.Text.Regex != "" {
			if re, err := regexp.Compile(ms.Text.Regex); err == nil {
				result.Text.regexComp = re
			}
		}
	}

	if ms.Logic != nil {
		result.Logic = compileLogic(ms.Logic)
	}

	return result
}
