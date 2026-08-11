package rule

import (
	"encoding/json"
)

// evaluateMatchSpec evaluates all dimensions of a MatchSpec against a ResponseContext.
// All non-nil dimensions must match (AND semantics).
func evaluateMatchSpec(spec *MatchSpec, ctx *ResponseContext) (bool, error) {
	// HTTP status code
	if len(spec.HTTPStatusCodes) > 0 {
		if !MatchHTTPStatus(spec.HTTPStatusCodes, ctx.StatusCode) {
			return false, nil
		}
	}

	// Headers
	if len(spec.Headers) > 0 {
		if !MatchHeaders(spec.Headers, ctx.Headers) {
			return false, nil
		}
	}

	// JSONPath
	if spec.JSONPath != nil {
		// Parse JSON body if not already parsed
		if ctx.BodyJSON == nil && len(ctx.Body) > 0 {
			var parsed interface{}
			if err := json.Unmarshal(ctx.Body, &parsed); err == nil {
				ctx.BodyJSON = parsed
			}
		}
		ok, err := MatchJSONPath(spec.JSONPath, ctx.BodyJSON)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	// Text match
	if spec.Text != nil {
		if !MatchText(spec.Text, ctx.Body) {
			return false, nil
		}
	}

	// Logic combination (overrides individual dimensions if present)
	if spec.Logic != nil {
		ok, err := MatchLogic(spec.Logic, ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}
