package ui

import (
	"encoding/json"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	// 1. Valid JSON Log
	validLog := map[string]interface{}{
		"time":  "2023-10-27T10:00:00Z",
		"level": "INFO",
		"msg":   "test message",
		"extra": "value",
	}
	validBytes, _ := json.Marshal(validLog)

	// 2. Invalid JSON (Raw Text)
	rawText := "This is a raw log line"

	// 3. Empty Line
	emptyLine := ""

	// Combine
	input := string(validBytes) + "\n" + rawText + "\n" + emptyLine

	entries, err := ParseLogLines([]byte(input))
	assert.NoError(t, err)
	assert.Len(t, entries, 2)

	// Verify Valid Entry
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "test message", entries[0].Msg)
	assert.Equal(t, "value", entries[0].Raw["extra"])
	expectedTime, _ := time.Parse(time.RFC3339, "2023-10-27T10:00:00Z")
	assert.Equal(t, expectedTime, entries[0].Time)

	// Verify Raw Entry
	assert.Equal(t, "TEXT", entries[1].Level)
	assert.Equal(t, "This is a raw log line", entries[1].Msg)
}

func TestParseLogLines_TimeFormats(t *testing.T) {
	// RFC3339Nano
	logNano := map[string]interface{}{
		"time":  "2023-10-27T10:00:00.123456Z",
		"level": "DEBUG",
		"msg":   "nano",
	}
	bytesNano, _ := json.Marshal(logNano)

	entries, err := ParseLogLines(bytesNano)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "DEBUG", entries[0].Level)
	// Just check if time parses reasonably close or specific nanoseconds if needed
	// Here we just ensure it didn't fail and defaulted to Now() (which we can't easily test without mocking time, but we can check if it parsed)
	// Since the code explicitly handles RFC3339Nano, it should match.
	expected, _ := time.Parse(time.RFC3339Nano, "2023-10-27T10:00:00.123456Z")
	assert.Equal(t, expected, entries[0].Time)
}

func TestParseLogLines_MissingFields(t *testing.T) {
	// JSON without time/level/msg
	logEmpty := map[string]interface{}{
		"foo": "bar",
	}
	bytesEmpty, _ := json.Marshal(logEmpty)

	entries, err := ParseLogLines(bytesEmpty)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	// Should default level to INFO
	assert.Equal(t, "INFO", entries[0].Level)
	// Msg should be empty or handle gracefully
	assert.Equal(t, "", entries[0].Msg)
	// Time should be zero if missing
	assert.True(t, entries[0].Time.IsZero())
}

func TestLogEntry_Methods(t *testing.T) {
	entry := LogEntry{
		Time:  time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		Level: "ERROR",
		Msg:   "Something bad",
	}

	assert.Equal(t, "[ERROR] Something bad", entry.Title())
	assert.Equal(t, "10:00:00.000", entry.Description())
	assert.Equal(t, "ERROR Something bad", entry.FilterValue())
}

func TestPlaybackModel_Init(t *testing.T) {
	m := NewPlaybackModel(nil)
	assert.Nil(t, m.Init())
}

func TestPlaybackModel_Update(t *testing.T) {
	// Setup
	entries := []LogEntry{
		{Msg: "Log 1", Level: "INFO", Time: time.Now()},
		{Msg: "Log 2", Level: "ERROR", Time: time.Now()},
	}
	m := NewPlaybackModel(entries)
	m.width = 100
	m.height = 20

	// 1. Resize
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedM, _ := m.Update(msg)
	m = updatedM.(PlaybackModel)
	assert.Equal(t, 80, m.width)
	assert.Equal(t, 24, m.height)

	// 2. Quit
	msgKey := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msgKey)
	assert.Equal(t, tea.Quit(), cmd())

	// 3. Select Item (Enter) -> Details View
	// List starts with first item selected by default
	msgKey = tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ = m.Update(msgKey)
	m = updatedM.(PlaybackModel)
	assert.True(t, m.viewingDetails)

	// 4. View Details -> Back (Esc)
	msgKey = tea.KeyMsg{Type: tea.KeyEsc}
	updatedM, _ = m.Update(msgKey)
	m = updatedM.(PlaybackModel)
	assert.False(t, m.viewingDetails)
}

func TestPlaybackModel_View(t *testing.T) {
	entries := []LogEntry{
		{Msg: "Log 1", Level: "INFO", Time: time.Now()},
	}
	m := NewPlaybackModel(entries)

	// Update with size to ensure list renders
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updatedM.(PlaybackModel)

	// List View
	v := m.View()
	assert.Contains(t, v, "Log 1")

	// Details View
	m.viewingDetails = true
	v = m.View()
	assert.Contains(t, v, "Entry Details")
}
