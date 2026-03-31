package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"bytes"
	"recac/internal/analysis"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDepsCommand(t *testing.T) {
	// Backup original functions
	origAnalyze := analyzeDependenciesFunc
	origStart := startDepsFunc
	defer func() {
		analyzeDependenciesFunc = origAnalyze
		startDepsFunc = origStart
	}()

	// Mock AnalyzeDependencies
	mockDeps := analysis.DepMap{
		"pkgA": {"pkgB"},
		"pkgB": {"pkgC", "pkgD"},
		"pkgC": {},
		"pkgD": {},
	}
	analyzeDependenciesFunc = func(opts analysis.DependencyOptions) (analysis.DepMap, error) {
		return mockDeps, nil
	}

	// Mock StartDeps
	var capturedDeps map[string][]string
	startDepsFunc = func(deps map[string][]string, opts ...tea.ProgramOption) error {
		capturedDeps = deps
		return nil
	}

	// Execute command
	cmd := depsCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// We call runDeps directly to avoid Cobra flag parsing complexity in unit test
	err := runDeps(cmd, []string{"."})

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Analyzing dependencies")

	// Verify captured deps
	assert.NotNil(t, capturedDeps)
	assert.Equal(t, 4, len(capturedDeps))
	assert.Equal(t, []string{"pkgB"}, capturedDeps["pkgA"])
	assert.ElementsMatch(t, []string{"pkgC", "pkgD"}, capturedDeps["pkgB"])
}
