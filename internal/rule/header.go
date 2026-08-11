package rule

import "strings"

// MatchHeaders checks if all specified header conditions are met.
func MatchHeaders(headers []HeaderMatch, respHeaders map[string][]string) bool {
	if len(headers) == 0 {
		return true // no constraint → always matches
	}

	for _, h := range headers {
		values, ok := respHeaders[h.Name]
		if !ok {
			// Also try canonical header name
			values, ok = respHeaders[canonicalHeaderKey(h.Name)]
		}
		if !ok {
			return false // header not found
		}

		if h.Value == "" {
			// Just check existence — already confirmed above
			continue
		}

		// Check if any value matches
		found := false
		for _, v := range values {
			if strings.EqualFold(v, h.Value) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// canonicalHeaderKey converts a header name to canonical form (Title-Case).
func canonicalHeaderKey(key string) string {
	// Simple canonicalization: capitalize first letter and after hyphens
	result := make([]byte, len(key))
	upper := true
	for i, c := range key {
		if upper {
			result[i] = byte(toUpper(c))
			upper = false
		} else {
			result[i] = byte(toLower(c))
		}
		if c == '-' {
			upper = true
		}
	}
	return string(result)
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}
