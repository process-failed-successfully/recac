package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseStaleDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"1h", time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"0.5d", 12 * time.Hour, false},
		{"1.5d", 36 * time.Hour, false},
		{"10m", 10 * time.Minute, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseStaleDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
