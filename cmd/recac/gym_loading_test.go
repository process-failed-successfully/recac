package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadChallenges_Directory(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// 1. Create a valid challenge file (YAML)
	validYaml := `
name: "Challenge 1"
description: "Desc 1"
language: "python"
tests: "print('Hello')"
test_file: "test1.py"
timeout: 10
`
	err := os.WriteFile(filepath.Join(tmpDir, "challenge1.yaml"), []byte(validYaml), 0644)
	assert.NoError(t, err)

	// 2. Create a valid challenge file (JSON)
	validJson := `
{
  "name": "Challenge 2",
  "description": "Desc 2",
  "language": "go",
  "tests": "package main",
  "test_file": "test2.go",
  "timeout": 20
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "challenge2.json"), []byte(validJson), 0644)
	assert.NoError(t, err)

	// 3. Create an irrelevant file (txt) - should be ignored
	err = os.WriteFile(filepath.Join(tmpDir, "README.txt"), []byte("Ignore me"), 0644)
	assert.NoError(t, err)

	// 4. Create a YAML file that is valid syntax but NOT a challenge (e.g. config)
	// This should be ignored (skipped with warning)
	invalidYamlStruct := `
foo: bar
baz: 123
`
	err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(invalidYamlStruct), 0644)
	assert.NoError(t, err)

	// 5. Create a YAML file that is INVALID syntax
	// This should be ignored (skipped with warning)
	invalidYamlSyntax := `
foo: [unclosed list
`
	err = os.WriteFile(filepath.Join(tmpDir, "broken.yaml"), []byte(invalidYamlSyntax), 0644)
	assert.NoError(t, err)


	// Run loadChallenges
	challenges, err := loadChallenges(tmpDir)

	// Assert
	if err != nil {
		t.Fatalf("loadChallenges failed: %v", err)
	}

	// Should have 2 challenges
	assert.Equal(t, 2, len(challenges), "Expected 2 challenges, got %d", len(challenges))

	names := make(map[string]bool)
	for _, c := range challenges {
		names[c.Name] = true
	}
	assert.True(t, names["Challenge 1"])
	assert.True(t, names["Challenge 2"])
}
