package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyInitCmd(t *testing.T) {
	// Change CWD to temp dir to avoid messing with repo
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	cmd := policyInitCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, []string{})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Created default policy")

	assert.FileExists(t, ".recac/policies.yaml")
}

func TestPolicyCheckCmd(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create policy file in root
	policyContent := `
rules:
  - type: banned_content
    pattern: "bad"
`
	os.WriteFile("policy.yaml", []byte(policyContent), 0644)

	// Create src dir
	os.Mkdir("src", 0755)

	// Create bad file in src
	os.WriteFile("src/test.txt", []byte("this is bad"), 0644)

	// Set global flag variable manually since we aren't parsing flags via root
	oldPolicyFile := policyFile
	policyFile = "policy.yaml"
	defer func() { policyFile = oldPolicyFile }()

	cmd := policyCheckCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, []string{"src"}) // Check src dir only
	assert.Error(t, err)                  // Should fail
	assert.Contains(t, buf.String(), "Found 1 policy violations")
	assert.Contains(t, buf.String(), "Found banned content: bad")
}

func TestPolicyCheckCmd_Pass(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	policyContent := `
rules:
  - type: banned_content
    pattern: "bad"
`
	os.WriteFile("policy.yaml", []byte(policyContent), 0644)

	os.Mkdir("src", 0755)
	os.WriteFile("src/test.txt", []byte("this is good"), 0644)

	oldPolicyFile := policyFile
	policyFile = "policy.yaml"
	defer func() { policyFile = oldPolicyFile }()

	cmd := policyCheckCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, []string{"src"})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Policy check passed")
}
