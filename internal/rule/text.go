package rule

import "strings"

// MatchText checks the raw response body against text match conditions.
func MatchText(match *TextMatch, body []byte) bool {
	if match == nil {
		return true // no constraint → always matches
	}

	bodyStr := string(body)

	if match.Contains != "" {
		if !strings.Contains(bodyStr, match.Contains) {
			return false
		}
	}

	if match.Regex != "" && match.regexComp != nil {
		if !match.regexComp.MatchString(bodyStr) {
			return false
		}
	}

	return true
}
