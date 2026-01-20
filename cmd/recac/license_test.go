package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// LicenseTestMockAgent implements Agent for testing
type LicenseTestMockAgent struct {
	Response string
	Err      error
}

func (m *LicenseTestMockAgent) Send(ctx context.Context, input string) (string, error) {
	return m.Response, m.Err
}

func (m *LicenseTestMockAgent) SendStream(ctx context.Context, input string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestLicenseCheckCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()

	// Create vendor structure
	vendorDir := filepath.Join(tmpDir, "vendor", "github.com", "foo", "bar")
	err := os.MkdirAll(vendorDir, 0755)
	assert.NoError(t, err)

	// Create a clear MIT license file
	mitLicense := `MIT License
Copyright (c) 2023 Foo Bar
Permission is hereby granted...`
	err = os.WriteFile(filepath.Join(vendorDir, "LICENSE"), []byte(mitLicense), 0644)
	assert.NoError(t, err)

	// Create node_modules structure
	nodeDir := filepath.Join(tmpDir, "node_modules", "weird-pkg")
	err = os.MkdirAll(nodeDir, 0755)
	assert.NoError(t, err)

	// Create an ambiguous license file
	ambiguousLicense := `This software is free to use but you must buy me a beer.`
	err = os.WriteFile(filepath.Join(nodeDir, "LICENSE.txt"), []byte(ambiguousLicense), 0644)
	assert.NoError(t, err)

	// Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, cwd, agentType string) (agent.Agent, error) {
		// If we are asked about the ambiguous license, return "Beerware"
		return &LicenseTestMockAgent{
			Response: "Beerware",
		}, nil
	}

	// Helper to create command
	createCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "check"}
		cmd.SetOut(os.Stdout)
		// Re-bind flags because they are bound to the global `licenseCheckCmd` which persists state?
		// Actually, `licenseCheckCmd` is a global var.
		// Best practice is to use the global command but reset flags or vars.
		// `licenseAllow`, `licenseDeny` etc are global vars.
		licenseAllow = []string{"MIT", "Beerware"}
		licenseDeny = []string{"GPL"}
		licenseJSON = false
		licenseFail = false
		licenseNoAI = false
		return licenseCheckCmd
	}

	// Test 1: Standard Run
	t.Run("Standard Run", func(t *testing.T) {
		cmd := createCmd()
		// We capture output
		// For simplicity in this test environment, we just run it and check no error
		// and maybe intercept stdout if we wanted to be strict.

		err := runLicenseCheck(cmd, []string{tmpDir})
		assert.NoError(t, err)
	})

	// Test 2: Deny Policy
	t.Run("Deny Policy", func(t *testing.T) {
		cmd := createCmd()
		licenseDeny = []string{"Beerware"} // Now Beerware is denied
		licenseFail = true

		err := runLicenseCheck(cmd, []string{tmpDir})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "license compliance failed")
		}
	})

	// Test 3: JSON Output
	t.Run("JSON Output", func(t *testing.T) {
		cmd := createCmd()
		licenseJSON = true

		// Capture stdout
		var out bytes.Buffer
		cmd.SetOut(&out)

		err := runLicenseCheck(cmd, []string{tmpDir})
		assert.NoError(t, err)

		// Verify JSON
		var results []struct {
			Package string `json:"package"`
			License string `json:"license"`
		}
		err = json.Unmarshal(out.Bytes(), &results)
		assert.NoError(t, err, "Output should be valid JSON")
		assert.Len(t, results, 2)
	})

	// Test 4: No AI
	t.Run("No AI", func(t *testing.T) {
		cmd := createCmd()
		licenseNoAI = true
		licenseFail = false

		// Beerware should now be "Unknown" and thus "Review" status
		// "Review" status doesn't trigger fail unless we enforce strictness?
		// The code says fail only if Denied found.

		err := runLicenseCheck(cmd, []string{tmpDir})
		assert.NoError(t, err)
	})
}
