package main

import (
	"bytes"
	"context"
	"os"
	"testing"
	"recac/internal/agent"

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

func (m *MockAgentAlias) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestGetAISuggestions(t *testing.T) {
	stats := []commandStat{
		{Command: "todo solve --file foo.go", Count: 5},
	}

	mockResp := `[
		{"command": "todo solve --file foo.go", "alias": "fix-foo", "reason": "Test"}
	]`

	ag := &MockAgentAlias{Response: mockResp}

	suggestions, err := getAISuggestions(context.Background(), ag, stats)
	require.NoError(t, err)
	require.Len(t, suggestions, 1)
	assert.Equal(t, "fix-foo", suggestions[0].Alias)
	assert.Equal(t, 5, suggestions[0].Frequency) // Should merge count
}

func TestGetAISuggestions_InvalidJSON(t *testing.T) {
	stats := []commandStat{{Command: "foo", Count: 1}}
	ag := &MockAgentAlias{Response: "invalid json"}

	_, err := getAISuggestions(context.Background(), ag, stats)
	assert.Error(t, err)
}

func TestRunAliasSuggest(t *testing.T) {
	// Create mock history
	content := `
recac todo solve --file main.go
recac todo solve --file main.go
recac todo solve --file main.go
`
	tmpfile, err := os.CreateTemp("", "bash_history")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	_, err = tmpfile.WriteString(content)
	require.NoError(t, err)
	tmpfile.Close()

	// Mock Agent
	mockResp := `[
		{"command": "todo solve --file main.go", "alias": "fix-main", "reason": "Frequent"}
	]`
	mockAg := &MockAgentAlias{Response: mockResp}

	// Override Factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute command
	// Set flags via variables (since they are package level vars in alias_suggest.go)
	// aliasSuggestHistoryFile, etc. are vars in alias_suggest.go
	// But flags are bound to them.
	// runAliasSuggest reads from flags?
	// runAliasSuggest reads from variables: `histFile := aliasSuggestHistoryFile`

	oldHist := aliasSuggestHistoryFile
	oldShell := aliasSuggestShell
	oldMinFreq := aliasSuggestMinFreq
	oldJSON := aliasSuggestJSON

	defer func() {
		aliasSuggestHistoryFile = oldHist
		aliasSuggestShell = oldShell
		aliasSuggestMinFreq = oldMinFreq
		aliasSuggestJSON = oldJSON
	}()

	aliasSuggestHistoryFile = tmpfile.Name()
	aliasSuggestShell = "bash"
	aliasSuggestMinFreq = 2
	aliasSuggestJSON = false

	cmd := aliasSuggestCmd
	// Set output to buffer
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// We can't call cmd.Execute() easily because it parses flags.
	// But runAliasSuggest takes cmd and args.
	err = runAliasSuggest(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "fix-main")
	assert.Contains(t, output, "recac todo solve --file main.go")
}

func TestRunAliasSuggest_JSON(t *testing.T) {
	// Mock History
	content := `recac long command
recac long command
recac long command`
	tmpfile, _ := os.CreateTemp("", "hist")
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString(content)
	tmpfile.Close()

	// Mock Agent
	mockResp := `[{"command": "long command", "alias": "lc", "reason": "test"}]`
	mockAg := &MockAgentAlias{Response: mockResp}

	// Override Factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Set Vars
	aliasSuggestHistoryFile = tmpfile.Name()
	aliasSuggestShell = "bash"
	aliasSuggestMinFreq = 2
	aliasSuggestJSON = true
	defer func() { aliasSuggestJSON = false }()

	cmd := aliasSuggestCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runAliasSuggest(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"alias": "lc"`)
}
