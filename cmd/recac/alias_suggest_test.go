package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHistory_Bash(t *testing.T) {
	content := `
ls -la
recac status
recac todo list
recac todo solve --file main.go
git status
recac help
`
	tmpfile, err := os.CreateTemp("", "bash_history")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(content)
	require.NoError(t, err)
	tmpfile.Close()

	cmds, err := parseHistory("bash", tmpfile.Name())
	require.NoError(t, err)

	expected := []string{
		"status",
		"todo list",
		"todo solve --file main.go",
		"help",
	}
	assert.Equal(t, expected, cmds)
}

func TestParseHistory_Zsh(t *testing.T) {
	content := `
: 1670000000:0;ls -la
: 1670000001:0;recac status
: 1670000002:0;recac todo solve --file main.go
`
	tmpfile, err := os.CreateTemp("", "zsh_history")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(content)
	require.NoError(t, err)
	tmpfile.Close()

	cmds, err := parseHistory("zsh", tmpfile.Name())
	require.NoError(t, err)

	expected := []string{
		"status",
		"todo solve --file main.go",
	}
	assert.Equal(t, expected, cmds)
}

func TestAnalyzeHistory(t *testing.T) {
	commands := []string{
		"short",
		"short",
		"short",
		"long command but infrequent",
		"long frequent command",
		"long frequent command",
		"long frequent command",
		"another long frequent",
		"another long frequent",
		"another long frequent",
	}

	// minFreq = 3
	stats := analyzeHistory(commands, 3)

	require.Len(t, stats, 2)

	// Should be sorted by count
	assert.Equal(t, "another long frequent", stats[0].Command)
	assert.Equal(t, 3, stats[0].Count)
	assert.Equal(t, "long frequent command", stats[1].Command)
	assert.Equal(t, 3, stats[1].Count)

	// "short" has length 5 < 10, so ignored even if frequent
}

type MockAgentAlias struct {
	Response string
	Err      error
}

func (m *MockAgentAlias) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func TestGetAISuggestions(t *testing.T) {
	stats := []commandStat{
		{Command: "todo solve --file foo.go", Count: 5},
	}

	mockResp := `[
		{"command": "todo solve --file foo.go", "alias": "fix-foo", "reason": "Test"}
	]`

	agent := &MockAgentAlias{Response: mockResp}

	suggestions, err := getAISuggestions(context.Background(), agent, stats)
	require.NoError(t, err)
	require.Len(t, suggestions, 1)
	assert.Equal(t, "fix-foo", suggestions[0].Alias)
	assert.Equal(t, 5, suggestions[0].Frequency) // Should merge count
}

func TestGetAISuggestions_InvalidJSON(t *testing.T) {
	stats := []commandStat{{Command: "foo", Count: 1}}
	agent := &MockAgentAlias{Response: "invalid json"}

	_, err := getAISuggestions(context.Background(), agent, stats)
	assert.Error(t, err)
}
