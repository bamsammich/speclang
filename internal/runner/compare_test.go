package runner

import (
	"encoding/json"
	"testing"
)

func TestCompareAssertion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		op       string
		actual   any
		expected any
		want     bool
	}{
		{"gt true", ">", 5.0, 3.0, true},
		{"gt false", ">", 3.0, 5.0, false},
		{"gt equal", ">", 3.0, 3.0, false},
		{"gte true", ">=", 5.0, 3.0, true},
		{"gte equal", ">=", 3.0, 3.0, true},
		{"gte false", ">=", 2.0, 3.0, false},
		{"lt true", "<", 1.0, 3.0, true},
		{"lt false", "<", 5.0, 3.0, false},
		{"lte true", "<=", 3.0, 3.0, true},
		{"lte false", "<=", 5.0, 3.0, false},
		{"neq true", "!=", 1.0, 2.0, true},
		{"neq false", "!=", 1.0, 1.0, false},
		{"neq strings", "!=", "a", "b", true},
		{"neq strings equal", "!=", "a", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, _ := json.Marshal(tt.actual)
			expected, _ := json.Marshal(tt.expected)
			got, err := compareAssertion(tt.op, actual, expected)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("compareAssertion(%q, %s, %s) = %v, want %v",
					tt.op, actual, expected, got, tt.want)
			}
		})
	}
}

func TestCompareAssertion_NonNumericRelational(t *testing.T) {
	t.Parallel()

	actual, _ := json.Marshal("not a number")
	expected, _ := json.Marshal(1.0)
	_, err := compareAssertion(">=", actual, expected)
	if err == nil {
		t.Fatal("expected error for non-numeric relational comparison")
	}
}
