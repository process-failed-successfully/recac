package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadPipelineVars_SuccessEnv(t *testing.T) {
	// Create a temp .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "vars.env")
	envContent := `ENV_KEY1=env_val1
ENV_KEY2=env_val2
`
	err := os.WriteFile(envFile, []byte(envContent), 0644)
	assert.NoError(t, err)

	// Combine with flag args
	varList := []string{"FLAG_KEY1=flag_val1", "ENV_KEY2=flag_override_val2"}

	vars, err := loadPipelineVars(varList, envFile)
	assert.NoError(t, err)
	assert.Equal(t, "env_val1", vars["ENV_KEY1"])
	assert.Equal(t, "flag_override_val2", vars["ENV_KEY2"]) // flag overrides file
	assert.Equal(t, "flag_val1", vars["FLAG_KEY1"])
}

func TestLoadPipelineVars_SuccessJSON(t *testing.T) {
	// Create a temp .json file
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "vars.json")
	jsonContent := `{
		"JSON_KEY1": "json_val1",
		"JSON_KEY2": "json_val2"
	}`
	err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	assert.NoError(t, err)

	// Combine with flag args
	varList := []string{"FLAG_KEY1=flag_val1", "JSON_KEY2=flag_override_val2"}

	vars, err := loadPipelineVars(varList, jsonFile)
	assert.NoError(t, err)
	assert.Equal(t, "json_val1", vars["JSON_KEY1"])
	assert.Equal(t, "flag_override_val2", vars["JSON_KEY2"]) // flag overrides file
	assert.Equal(t, "flag_val1", vars["FLAG_KEY1"])
}

func TestLoadPipelineVars_SuccessYAML(t *testing.T) {
	// Create a temp .yaml file
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "vars.yaml")
	yamlContent := `YAML_KEY1: yaml_val1
YAML_KEY2: yaml_val2
`
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	// Combine with flag args
	varList := []string{"FLAG_KEY1=flag_val1", "YAML_KEY2=flag_override_val2"}

	vars, err := loadPipelineVars(varList, yamlFile)
	assert.NoError(t, err)
	assert.Equal(t, "yaml_val1", vars["YAML_KEY1"])
	assert.Equal(t, "flag_override_val2", vars["YAML_KEY2"]) // flag overrides file
	assert.Equal(t, "flag_val1", vars["FLAG_KEY1"])
}

func TestLoadPipelineVars_FileNotFound(t *testing.T) {
	varList := []string{"FLAG_KEY1=flag_val1"}
	vars, err := loadPipelineVars(varList, "non_existent_file.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read variable file")
	assert.Nil(t, vars)
}

func TestLoadPipelineVars_UnsupportedFormat(t *testing.T) {
	// Create a temp .txt file
	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "vars.txt")
	err := os.WriteFile(txtFile, []byte("some text"), 0644)
	assert.NoError(t, err)

	varList := []string{"FLAG_KEY1=flag_val1"}
	vars, err := loadPipelineVars(varList, txtFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported variable file format")
	assert.Nil(t, vars)
}

func TestLoadPipelineVars_EmptyFileWithArgs(t *testing.T) {
	varList := []string{"FLAG_KEY1=flag_val1"}
	vars, err := loadPipelineVars(varList, "")
	assert.NoError(t, err)
	assert.Equal(t, "flag_val1", vars["FLAG_KEY1"])
}
