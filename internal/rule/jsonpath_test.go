package rule

import (
	"encoding/json"
	"testing"
)

func TestMatchJSONPath(t *testing.T) {
	// Parse a sample JSON body
	body := `{"code": 700, "msg": "quota exceeded", "data": {"nested": true}}`
	var bodyJSON interface{}
	if err := json.Unmarshal([]byte(body), &bodyJSON); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	tests := []struct {
		name     string
		match    *JSONPathMatch
		want     bool
		wantErr  bool
	}{
		{
			name:  "nil match always matches",
			match: nil,
			want:  true,
		},
		{
			name: "exact int match",
			match: &JSONPathMatch{Path: "$.code", Operator: "==", Value: 700},
			want: true,
		},
		{
			name: "int no match",
			match: &JSONPathMatch{Path: "$.code", Operator: "==", Value: 200},
			want: false,
		},
		{
			name: "string match",
			match: &JSONPathMatch{Path: "$.msg", Operator: "contains", Value: "quota"},
			want: true,
		},
		{
			name: "not equal",
			match: &JSONPathMatch{Path: "$.code", Operator: "!=", Value: 200},
			want: true,
		},
		{
			name: "greater than",
			match: &JSONPathMatch{Path: "$.code", Operator: ">", Value: 600},
			want: true,
		},
		{
			name: "path not found",
			match: &JSONPathMatch{Path: "$.nonexistent", Operator: "==", Value: 1},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchJSONPath(tt.match, bodyJSON)
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchJSONPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MatchJSONPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchJSONPathNilBody(t *testing.T) {
	match := &JSONPathMatch{Path: "$.code", Operator: "==", Value: 700}
	got, err := MatchJSONPath(match, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if got != false {
		t.Errorf("MatchJSONPath with nil body should return false, got %v", got)
	}
}
