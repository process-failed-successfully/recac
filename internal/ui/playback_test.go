package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogLines(t *testing.T) {
	// 1. Create temporary log file content
	jsonl := `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Starting session","session":"test-1"}
{"time":"2023-10-27T10:00:01Z","level":"DEBUG","msg":"Thinking","prompt":"User asked..."}
{"time":"2023-10-27T10:00:02Z","level":"ERROR","msg":"Failed","error":"connection refused"}
Non-JSON line here
`

	// 2. Parse
	entries, err := ParseLogLines(strings.NewReader(jsonl))
	require.NoError(t, err)

	// 3. Assertions
	require.Len(t, entries, 4)

	// Entry 0
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "Starting session", entries[0].Msg)
	assert.Equal(t, "test-1", entries[0].Raw["session"])
	expectedTime, _ := time.Parse(time.RFC3339, "2023-10-27T10:00:00Z")
	assert.Equal(t, expectedTime, entries[0].Time)

	// Entry 1
	assert.Equal(t, "DEBUG", entries[1].Level)
	assert.Equal(t, "Thinking", entries[1].Msg)
	assert.Equal(t, "User asked...", entries[1].Raw["prompt"])

	// Entry 2
	assert.Equal(t, "ERROR", entries[2].Level)
	assert.Equal(t, "Failed", entries[2].Msg)

	// Entry 3 (Non-JSON)
	assert.Equal(t, "TEXT", entries[3].Level)
	assert.Equal(t, "Non-JSON line here", entries[3].Msg)
}
