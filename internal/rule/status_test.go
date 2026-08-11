package rule

import (
	"testing"
)

func TestMatchHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		codes      []int
		statusCode int
		want       bool
	}{
		{"exact match", []int{429}, 429, true},
		{"no match", []int{429}, 200, false},
		{"list match", []int{429, 502, 503}, 502, true},
		{"list no match", []int{429, 502, 503}, 500, false},
		{"empty codes (always match)", []int{}, 200, true},
		{"nil codes (always match)", nil, 500, true},
		{"5xx range", []int{500, 501, 502, 503, 504, 599}, 503, true},
		{"5xx range miss", []int{500, 501, 502, 503, 504, 599}, 429, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchHTTPStatus(tt.codes, tt.statusCode)
			if got != tt.want {
				t.Errorf("MatchHTTPStatus(%v, %d) = %v, want %v", tt.codes, tt.statusCode, got, tt.want)
			}
		})
	}
}
