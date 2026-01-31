package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyInitCmd(t *testing.T) {
	// Configure init to use temp dir
	tmpDir := t.TempDir()
	oldConfigDir := policyConfigDir
	policyConfigDir = tmpDir
	defer func() { policyConfigDir = oldConfigDir }()

	cmd := policyInitCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, []string{})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Created default policy")

	assert.FileExists(t, filepath.Join(tmpDir, "policies.yaml"))
}

func TestPolicyCheckCmd(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy file in temp dir
	policyContent := `
rules:
  - type: banned_content
    pattern: "bad"
`
	policyPath := filepath.Join(tmpDir, "policy.yaml")
	os.WriteFile(policyPath, []byte(policyContent), 0644)

	// Create src dir in temp dir
	srcDir := filepath.Join(tmpDir, "src")
	os.Mkdir(srcDir, 0755)

	// Create bad file in src
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("this is bad"), 0644)

	// Set global flag variable manually since we aren't parsing flags via root
	oldPolicyFile := policyFile
	policyFile = policyPath
	defer func() { policyFile = oldPolicyFile }()

	cmd := policyCheckCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, []string{srcDir}) // Check src dir only
	assert.Error(t, err)                   // Should fail
	assert.Contains(t, buf.String(), "Found 1 policy violations")
	assert.Contains(t, buf.String(), "Found banned content: bad")
}

func TestPolicyCheckCmd_Pass(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `
rules:
  - type: banned_content
    pattern: "bad"
`
	policyPath := filepath.Join(tmpDir, "policy.yaml")
	os.WriteFile(policyPath, []byte(policyContent), 0644)

	srcDir := filepath.Join(tmpDir, "src")
	os.Mkdir(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("this is good"), 0644)

	oldPolicyFile := policyFile
	policyFile = policyPath
	defer func() { policyFile = oldPolicyFile }()

	cmd := policyCheckCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, []string{srcDir})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Policy check passed")
}
