package query

import (
	"testing"
)

func TestIsDateOnlyCalendarValue(t *testing.T) {
	tests := []struct {
		value any
		want  bool
	}{
		{"2026-06-20", true},
		{"2026-06-20T00:00:00Z", false},
		{"2026-06", false},
		{20260620, false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isDateOnlyCalendarValue(tt.value); got != tt.want {
			t.Errorf("isDateOnlyCalendarValue(%#v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
