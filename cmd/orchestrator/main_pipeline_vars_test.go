package main

import (
	"io"
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

func TestListPipelineVarsJob_Success(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\nrequired_variables:\n  - REQ_VAR\nvariables:\n  DEC_VAR: val\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	listPipelineVarsJob(f.Name(), "")

	pw.Close()
	out, _ := io.ReadAll(pr)

	outStr := string(out)
	assert.Contains(t, outStr, "Pipeline Variables (2):")
	assert.Contains(t, outStr, "- DEC_VAR")
	assert.Contains(t, outStr, "- REQ_VAR")
	assert.Equal(t, 0, exitCode)
}

func TestListPipelineVarsJob_Empty(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	listPipelineVarsJob(f.Name(), "")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "No variables found in the pipeline.")
	assert.Equal(t, 0, exitCode)
}

func TestListPipelineVarsJob_JSON(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\nrequired_variables:\n  - VAR1\njobs:\n  build:\n    summary: build\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	listPipelineVarsJob(f.Name(), "json")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "[\n  \"VAR1\"\n]")
	assert.Equal(t, 0, exitCode)
}

func TestListPipelineVarsJob_InvalidFile(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	listPipelineVarsJob("non_existent_pipeline_file.yaml", "")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to read pipeline file")
	assert.Equal(t, 1, exitCode)
}

func TestListPipelineVarsJob_InvalidYaml(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	f, _ := os.CreateTemp("", "pipeline_*.yaml")
	f.Write([]byte("name: pipeline\njobs:\n  build:\n    summary: build\n    depends_on: [\n"))
	f.Close()
	defer os.Remove(f.Name())

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	listPipelineVarsJob(f.Name(), "")

	pw.Close()
	out, _ := io.ReadAll(pr)

	assert.Contains(t, string(out), "Failed to parse pipeline variables")
	assert.Equal(t, 1, exitCode)
}
