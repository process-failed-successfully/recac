package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	t.Run("JSON Logs", func(t *testing.T) {
		input := `{"time":"2023-10-27T10:00:00Z", "level":"INFO", "msg":"Hello", "foo":"bar"}
{"time":"2023-10-27T10:00:01Z", "level":"ERROR", "msg":"World", "baz":123}
`
		entries, err := ParseLogLines([]byte(input))
		assert.NoError(t, err)
		assert.Len(t, entries, 2)

		assert.Equal(t, "INFO", entries[0].Level)
		assert.Equal(t, "Hello", entries[0].Msg)
		assert.Equal(t, "bar", entries[0].Raw["foo"])
		assert.Contains(t, entries[0].Content, "foo")

		assert.Equal(t, "ERROR", entries[1].Level)
		assert.Equal(t, "World", entries[1].Msg)
		assert.Equal(t, float64(123), entries[1].Raw["baz"])
	})

	t.Run("Text Logs", func(t *testing.T) {
		input := `Plain text line 1
Plain text line 2`
		entries, err := ParseLogLines([]byte(input))
		assert.NoError(t, err)
		assert.Len(t, entries, 2)

		assert.Equal(t, "TEXT", entries[0].Level)
		assert.Equal(t, "Plain text line 1", entries[0].Msg)
		assert.Equal(t, "Plain text line 1", entries[0].Content)

		assert.Equal(t, "TEXT", entries[1].Level)
	})

	t.Run("Empty Lines", func(t *testing.T) {
		input := `
{"msg":"Real"}

`
		entries, err := ParseLogLines([]byte(input))
		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.Equal(t, "Real", entries[0].Msg)
	})
}

func TestLogEntry_Methods(t *testing.T) {
	now := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC)
	entry := LogEntry{
		Time:  now,
		Level: "INFO",
		Msg:   "Test Message",
	}

	assert.Equal(t, "[INFO] Test Message", entry.Title())
	assert.Equal(t, "10:00:00.000", entry.Description())
	assert.Equal(t, "INFO Test Message", entry.FilterValue())
}

func TestNewPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "One"},
		{Level: "ERROR", Msg: "Two"},
	}

	model := NewPlaybackModel(entries)

	assert.Len(t, model.entries, 2)
	assert.Equal(t, "Session Playback", model.list.Title)
	// Viewport should be initialized
	assert.NotNil(t, model.viewport)
}
