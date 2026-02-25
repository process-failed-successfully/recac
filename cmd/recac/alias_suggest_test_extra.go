package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestApplySelectedSuggestions(t *testing.T) {
	// Setup temp config file
	tmpFile, err := os.CreateTemp("", "config.*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	viper.SetConfigFile(tmpFile.Name())

	// Setup
	cmd := &cobra.Command{}
	outBuf := new(bytes.Buffer)
	inBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetIn(inBuf)

	suggestions := []AliasSuggestion{
		{Command: "cmd1", Alias: "a1", Frequency: 1},
		{Command: "cmd2", Alias: "a2", Frequency: 1},
	}

	// Mock Viper
	viper.Set("aliases", map[string]string{})

	// Case 1: Select "1"
	inBuf.WriteString("1\n")
	applySelectedSuggestions(cmd, suggestions)

	output := outBuf.String()
	assert.Contains(t, output, "Applied: a1='cmd1'")

	aliases := viper.GetStringMapString("aliases")
	assert.Equal(t, "cmd1", aliases["a1"], "Alias a1 should be cmd1")
	_, ok := aliases["a2"]
	assert.False(t, ok, "Alias a2 should not be set")

	// Case 2: Select "all"
	outBuf.Reset()
	inBuf.Reset()
	inBuf.WriteString("all\n")
	viper.Set("aliases", map[string]string{})

	applySelectedSuggestions(cmd, suggestions)

	output = outBuf.String()
	assert.Contains(t, output, "Applied: a1='cmd1'")
	assert.Contains(t, output, "Applied: a2='cmd2'")

	aliases = viper.GetStringMapString("aliases")
	assert.Equal(t, "cmd1", aliases["a1"])
	assert.Equal(t, "cmd2", aliases["a2"])

	// Case 3: Empty input
	outBuf.Reset()
	inBuf.Reset()
	inBuf.WriteString("\n")
	viper.Set("aliases", map[string]string{})

	applySelectedSuggestions(cmd, suggestions)
	assert.NotContains(t, outBuf.String(), "Applied")
	assert.Empty(t, viper.GetStringMapString("aliases"))
}
