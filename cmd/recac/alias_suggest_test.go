package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/viper"
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

func TestApplySelectedSuggestions(t *testing.T) {
	suggestions := []AliasSuggestion{
		{Command: "echo hello", Alias: "h", Frequency: 1},
		{Command: "echo world", Alias: "w", Frequency: 2},
	}

	// 1. Simulate input: select 1st one (index 1)
	input := "1\n"
	in := bytes.NewBufferString(input)
	out := new(bytes.Buffer)

	// Mock Viper config
	// We need to use a config file or just set it in memory
	// Viper operates on global state
	viper.Reset()
	// Create a temp config file to allow SafeWriteConfig to work (it needs a file path)
	tmpConfig, err := os.CreateTemp("", "config.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpConfig.Name())
	tmpConfig.Close()

	viper.SetConfigFile(tmpConfig.Name())
	// Pre-populate aliases to ensure merge or overwrite
	viper.Set("aliases", map[string]string{"existing": "cmd"})

	applySelectedSuggestions(in, out, suggestions)

	output := out.String()
	assert.Contains(t, output, "Applied: h='echo hello'")

	aliases := viper.GetStringMapString("aliases")
	assert.Equal(t, "echo hello", aliases["h"])
	assert.Equal(t, "cmd", aliases["existing"])
}

func TestApplySelectedSuggestions_All(t *testing.T) {
	suggestions := []AliasSuggestion{
		{Command: "a", Alias: "aa"},
		{Command: "b", Alias: "bb"},
	}

	input := "all\n"
	in := bytes.NewBufferString(input)
	out := new(bytes.Buffer)

	viper.Reset()
	tmpConfig, _ := os.CreateTemp("", "config.yaml")
	defer os.Remove(tmpConfig.Name())
	tmpConfig.Close()
	viper.SetConfigFile(tmpConfig.Name())

	applySelectedSuggestions(in, out, suggestions)

	aliases := viper.GetStringMapString("aliases")
	assert.Equal(t, "a", aliases["aa"])
	assert.Equal(t, "b", aliases["bb"])
}
func TestAnalyzeHistory_LimitAndSort(t *testing.T) {
	var commands []string

	// Top frequencies:
	// A: 10
	// B: 9
	// ...
	for i := 1; i <= 15; i++ {
		cmdStr := "long command prefix " + string(rune('A'+i)) // B to P
		for j := 0; j < i; j++ {                               // B: 1, C: 2, ... P: 15
			commands = append(commands, cmdStr)
		}
	}

	// Add two commands with exactly 15 frequency to test alphabetical sort
	for j := 0; j < 15; j++ {
		commands = append(commands, "long same freq Z")
		commands = append(commands, "long same freq A")
	}

	stats := analyzeHistory(commands, 2)

	require.Len(t, stats, 10) // Limit is 10

	// The highest freq is 15. The commands with 15 are:
	// "long command prefix P"
	// "long same freq A"
	// "long same freq Z"

	// Alphabetical order for freq 15:
	// "long command prefix P"
	// "long same freq A"
	// "long same freq Z"

	assert.Equal(t, "long command prefix P", stats[0].Command)
	assert.Equal(t, 15, stats[0].Count)

	assert.Equal(t, "long same freq A", stats[1].Command)
	assert.Equal(t, 15, stats[1].Count)

	assert.Equal(t, "long same freq Z", stats[2].Command)
	assert.Equal(t, 15, stats[2].Count)
}

func TestRunAliasSuggest_HistoryNotFound(t *testing.T) {
	oldHist := aliasSuggestHistoryFile
	defer func() { aliasSuggestHistoryFile = oldHist }()
	aliasSuggestHistoryFile = "/path/to/non/existent/history"

	cmd := aliasSuggestCmd
	err := runAliasSuggest(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "history file not found")
}

func TestRunAliasSuggest_NoFrequentCmds(t *testing.T) {
	// Create mock history with infrequent cmds
	content := "recac todo solve --file main.go\n"
	tmpfile, err := os.CreateTemp("", "bash_history")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString(content)
	tmpfile.Close()

	oldHist := aliasSuggestHistoryFile
	oldMinFreq := aliasSuggestMinFreq
	defer func() {
		aliasSuggestHistoryFile = oldHist
		aliasSuggestMinFreq = oldMinFreq
	}()

	aliasSuggestHistoryFile = tmpfile.Name()
	aliasSuggestMinFreq = 2 // but only 1 occurence

	cmd := aliasSuggestCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runAliasSuggest(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No frequent long 'recac' commands found.")
}

func TestRunAliasSuggest_AgentFailure(t *testing.T) {
	// Create mock history
	content := "recac todo solve --file main.go\nrecac todo solve --file main.go\n"
	tmpfile, err := os.CreateTemp("", "bash_history")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString(content)
	tmpfile.Close()

	// Mock Agent
	mockAg := &MockAgentAlias{Err: os.ErrPermission} // simulate agent error

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = origFactory }()

	oldHist := aliasSuggestHistoryFile
	oldMinFreq := aliasSuggestMinFreq
	defer func() {
		aliasSuggestHistoryFile = oldHist
		aliasSuggestMinFreq = oldMinFreq
	}()
	aliasSuggestHistoryFile = tmpfile.Name()
	aliasSuggestMinFreq = 2

	cmd := aliasSuggestCmd
	err = runAliasSuggest(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
}

func TestRunAliasSuggest_AgentFactoryFailure(t *testing.T) {
	// Create mock history
	content := "recac todo solve --file main.go\nrecac todo solve --file main.go\n"
	tmpfile, err := os.CreateTemp("", "bash_history")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString(content)
	tmpfile.Close()

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, os.ErrPermission
	}
	defer func() { agentClientFactory = origFactory }()

	oldHist := aliasSuggestHistoryFile
	oldMinFreq := aliasSuggestMinFreq
	defer func() {
		aliasSuggestHistoryFile = oldHist
		aliasSuggestMinFreq = oldMinFreq
	}()
	aliasSuggestHistoryFile = tmpfile.Name()
	aliasSuggestMinFreq = 2

	cmd := aliasSuggestCmd
	err = runAliasSuggest(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create agent")
}

func TestApplySelectedSuggestions_EmptyInput(t *testing.T) {
	suggestions := []AliasSuggestion{
		{Command: "echo hello", Alias: "h"},
	}

	input := "\n"
	in := bytes.NewBufferString(input)
	out := new(bytes.Buffer)

	applySelectedSuggestions(in, out, suggestions)
	assert.NotContains(t, out.String(), "Applied:")
}

func TestApplySelectedSuggestions_InvalidInput(t *testing.T) {
	suggestions := []AliasSuggestion{
		{Command: "echo hello", Alias: "h"},
	}

	input := "invalid,99\n"
	in := bytes.NewBufferString(input)
	out := new(bytes.Buffer)

	applySelectedSuggestions(in, out, suggestions)
	assert.Contains(t, out.String(), "No aliases selected.")
}

func TestRunAliasSuggest_AutoShellAndHome(t *testing.T) {
	oldShell := aliasSuggestShell
	oldHist := aliasSuggestHistoryFile
	oldEnvShell := os.Getenv("SHELL")
	oldHome := os.Getenv("HOME")

	defer func() {
		aliasSuggestShell = oldShell
		aliasSuggestHistoryFile = oldHist
		os.Setenv("SHELL", oldEnvShell)
		os.Setenv("HOME", oldHome)
	}()

	aliasSuggestShell = "auto"
	aliasSuggestHistoryFile = ""

	tempHome := t.TempDir()
	os.Setenv("HOME", tempHome)

	// Test bash detect
	os.Setenv("SHELL", "/bin/bash")
	bashHist := filepath.Join(tempHome, ".bash_history")
	os.WriteFile(bashHist, []byte("recac mock\nrecac mock\n"), 0644)

	aliasSuggestMinFreq = 2

	// Provide mock agent to not fail later steps
	mockAg := &MockAgentAlias{Response: "[]"}
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = origFactory }()

	cmd := aliasSuggestCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	err := runAliasSuggest(cmd, []string{})
	require.NoError(t, err)

	// Test zsh detect
	aliasSuggestShell = "auto"
	aliasSuggestHistoryFile = ""
	os.Setenv("SHELL", "/bin/zsh")
	zshHist := filepath.Join(tempHome, ".zsh_history")
	os.WriteFile(zshHist, []byte("recac mockz\nrecac mockz\n"), 0644)

	err = runAliasSuggest(cmd, []string{})
	require.NoError(t, err)
}
