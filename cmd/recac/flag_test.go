package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlagList(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	// Create a file with flag usage
	content := `package main
import "github.com/spf13/viper"

func main() {
	if viper.GetBool("features.testFlag") {
		println("Feature on")
	}
}
`
	err := os.WriteFile("main.go", []byte(content), 0644)
	require.NoError(t, err)

	// Capture output
	cmd := flagListCmd
	cmd.SetArgs([]string{})
	outputFile := filepath.Join(tmpDir, "output.txt")
	f, _ := os.Create(outputFile)
	cmd.SetOut(f)

	// Run
	err = runFlagList(cmd, []string{})
	require.NoError(t, err)
	f.Close()

	// Verify
	out, _ := os.ReadFile(outputFile)
	assert.Contains(t, string(out), "testFlag")
}

func TestFlagAdd(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	// Create dummy config
	configPath := filepath.Join(tmpDir, "config.yaml")
	// viper.WriteConfig requires the file to exist or use SafeWriteConfig
	// Our code uses SafeWriteConfig if not found, so we can start empty or just let it create.
	// But viper needs to know the config type.
	viper.Reset()
	viper.AddConfigPath(tmpDir)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Run Add
	cmd := flagAddCmd
	outputFile := filepath.Join(tmpDir, "output_add.txt")
	f, _ := os.Create(outputFile)
	cmd.SetOut(f)

	err := runFlagAdd(cmd, []string{"newFeature"})
	require.NoError(t, err)
	f.Close()

	// Verify Config Content on Disk
	// viper.WriteConfig() should have written it.
	content, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		// Maybe it wrote to where?
		// runFlagAdd uses SafeWriteConfigAs("config.yaml") if not found.
		content, err = os.ReadFile("config.yaml")
	}
	require.NoError(t, err)

	// Viper might format it differently, but "newFeature" and "false" should be there
	// Viper lowercases keys by default
	assert.Contains(t, strings.ToLower(string(content)), strings.ToLower("newFeature"))
	assert.Contains(t, string(content), "false")
}

func TestFlagCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	// Setup file
	filePath := filepath.Join(tmpDir, "feature.go")
	originalContent := `package main
import "github.com/spf13/viper"

func foo() {
	if viper.GetBool("features.oldFeature") {
		doNew()
	} else {
		doOld()
	}
}
`
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	// Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mock := agent.NewMockAgent()
	mock.SetResponse(`package main

func foo() {
	doNew()
}
`)
	agentClientFactory = func(ctx context.Context, p, m, wd, proj string) (agent.Agent, error) {
		return mock, nil
	}

	// 1. Test Dry Run
	cmd := flagCleanupCmd
	flagDryRun = true
	flagKeep = true
	cmd.SetArgs([]string{"oldFeature"})
	outputFile := filepath.Join(tmpDir, "output_dryrun.txt")
	f, _ := os.Create(outputFile)
	cmd.SetOut(f)

	err = runFlagCleanup(cmd, []string{"oldFeature"})
	require.NoError(t, err)
	f.Close()

	// Verify File Content NOT changed
	currentContent, _ := os.ReadFile(filePath)
	assert.Equal(t, originalContent, string(currentContent))

	// Verify Output contains "DRY RUN"
	out, _ := os.ReadFile(outputFile)
	assert.Contains(t, string(out), "DRY RUN")
	assert.Contains(t, string(out), "doNew()")

	// 2. Test Real Run
	flagDryRun = false
	outputFileReal := filepath.Join(tmpDir, "output_real.txt")
	f2, _ := os.Create(outputFileReal)
	cmd.SetOut(f2)

	err = runFlagCleanup(cmd, []string{"oldFeature"})
	require.NoError(t, err)
	f2.Close()

	// Verify File Content CHANGED
	newContent, _ := os.ReadFile(filePath)
	assert.Contains(t, string(newContent), "doNew()")
	assert.NotContains(t, string(newContent), "viper.GetBool")
}
