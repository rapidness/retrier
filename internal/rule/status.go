package rule

// MatchHTTPStatus checks if the response status code matches the rule.
func MatchHTTPStatus(codes []int, statusCode int) bool {
	if len(codes) == 0 {
		return true // no constraint → always matches
	}
	for _, code := range codes {
		if code == statusCode {
			return true
		}
	}
	return false
}
