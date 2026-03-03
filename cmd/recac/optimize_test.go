package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent"
)

func TestOptimizeCmd(t *testing.T) {
	// Setup mock agent client factory
	origAgentFactory := agentClientFactory
	defer func() { agentClientFactory = origAgentFactory }()

	origReadFileFunc := readFileFunc
	origWriteFileFunc := writeFileFunc
	defer func() {
		readFileFunc = origReadFileFunc
		writeFileFunc = origWriteFileFunc
	}()

	mockOptResponse := "func optimized() {}\n"

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		assert.Equal(t, "recac-bolt", projectName)
		mockAg := new(MockAgent)
		mockAg.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			cb := args.Get(2).(func(string))
			cb(mockOptResponse)
		}).Return(mockOptResponse, nil)
		return mockAg, nil
	}

	t.Run("optimize from stdin", func(t *testing.T) {
		cmd := optimizeCmd
		cmd.Flags().Set("in-place", "false")
		cmd.Flags().Set("diff", "false")

		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)

		inBuf := bytes.NewBufferString("func unoptimized() {}")
		cmd.SetIn(inBuf)

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)

		outStr := outBuf.String()
		assert.Contains(t, outStr, "func optimized() {}")
	})

	t.Run("optimize file in-place", func(t *testing.T) {
		cmd := optimizeCmd
		cmd.Flags().Set("in-place", "true")
		cmd.Flags().Set("diff", "false")

		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)

		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.go")

		err := os.WriteFile(testFile, []byte("func unoptimized() {}\n"), 0644)
		assert.NoError(t, err)

		// Set real file io funcs for this test
		readFileFunc = os.ReadFile
		writeFileFunc = os.WriteFile

		err = cmd.RunE(cmd, []string{testFile})
		assert.NoError(t, err)

		content, err := os.ReadFile(testFile)
		assert.NoError(t, err)
		assert.Equal(t, "func optimized() {}", string(content))
		assert.Contains(t, outBuf.String(), "Successfully optimized")
	})

	t.Run("optimize file diff", func(t *testing.T) {
		cmd := optimizeCmd
		cmd.Flags().Set("in-place", "false")
		cmd.Flags().Set("diff", "true")

		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)

		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.go")

		err := os.WriteFile(testFile, []byte("func unoptimized() {}\n"), 0644)
		assert.NoError(t, err)

		readFileFunc = os.ReadFile
		writeFileFunc = os.WriteFile

		err = cmd.RunE(cmd, []string{testFile})
		assert.NoError(t, err)

		content, err := os.ReadFile(testFile)
		assert.NoError(t, err)

		// Should not be modified in place
		assert.Equal(t, "func unoptimized() {}\n", string(content))

		outStr := outBuf.String()
		assert.Contains(t, outStr, "--- "+testFile+" (original)")
		assert.Contains(t, outStr, "+++ "+testFile+" (optimized)")
	})

	t.Run("error when using --in-place without file", func(t *testing.T) {
		cmd := optimizeCmd
		cmd.Flags().Set("in-place", "true")
		cmd.Flags().Set("diff", "false")

		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)

		inBuf := bytes.NewBufferString("func unoptimized() {}")
		cmd.SetIn(inBuf)

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--in-place flag requires a file argument")
	})
}
