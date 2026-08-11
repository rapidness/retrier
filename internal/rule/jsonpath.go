package rule

import (
	"fmt"
	"strconv"

	"github.com/ohler55/ojg/jp"
)

// MatchJSONPath evaluates a JSONPath expression against the parsed JSON body
// and compares the result with the expected value using the specified operator.
func MatchJSONPath(match *JSONPathMatch, bodyJSON interface{}) (bool, error) {
	if match == nil {
		return true, nil // no constraint → always matches
	}
	if bodyJSON == nil {
		return false, nil // can't match JSONPath without JSON body
	}

	// Compile and evaluate JSONPath expression
	x, err := jp.Parse([]byte(match.Path))
	if err != nil {
		return false, fmt.Errorf("parse jsonpath %q: %w", match.Path, err)
	}

	results := x.Get(bodyJSON)
	if len(results) == 0 {
		return false, nil // path not found → no match
	}

	// Use the first result
	actual := results[0]
	return compareValues(actual, match.Operator, match.Value), nil
}

// compareValues compares actual vs expected using the given operator.
func compareValues(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "==":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "!=":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	case "contains":
		return containsStr(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", expected))
	case ">", "<", ">=", "<=":
		return compareNumeric(actual, operator, expected)
	default:
		return false
	}
}

func compareNumeric(actual interface{}, operator string, expected interface{}) bool {
	a, err := toFloat64(actual)
	if err != nil {
		return false
	}
	e, err := toFloat64(expected)
	if err != nil {
		return false
	}

	switch operator {
	case ">":
		return a > e
	case "<":
		return a < e
	case ">=":
		return a >= e
	case "<=":
		return a <= e
	default:
		return false
	}
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
