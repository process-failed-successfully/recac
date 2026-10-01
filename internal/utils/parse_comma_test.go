package utils

import (
	"reflect"
	"testing"
)

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", nil},
		{"single element", "a", []string{"a"}},
		{"multiple elements", "a,b,c", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", []string{"a", "b", "c"}},
		{"empty elements", "a,,c", []string{"a", "c"}},
		{"only empty elements", ",,", nil},
		{"only spaces", "  ,  ,  ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCommaSeparated(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				if len(result) == 0 && len(tt.expected) == 0 {
					return
				}
				t.Errorf("ParseCommaSeparated() = %v, want %v", result, tt.expected)
			}
		})
	}
}
