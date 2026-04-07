package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitPipelineInteractiveJob(t *testing.T) {
	// Create a temporary pipeline YAML file
	pipelineYAML := `
name: Test Pipeline
variables:
  APP_NAME: "MyApp"
  ENV: "prod"
jobs:
  build:
    summary: Build ${APP_NAME}
    task: echo building ${APP_NAME} in ${ENV}
`
	tmpDir := t.TempDir()
	pipelinePath := filepath.Join(tmpDir, "pipeline.yaml")
	err := os.WriteFile(pipelinePath, []byte(pipelineYAML), 0644)
	require.NoError(t, err)

	// Create a mock editor script
	// The editor script will receive the JSON file path as an argument.
	// It will read it, modify it, and write it back.
	editorScript := filepath.Join(tmpDir, "mock_editor.sh")
	scriptContent := `#!/bin/sh
FILE=$1
# We'll use jq to modify the file if available, or just cat a replacement
cat <<EOF > "$FILE"
{
  "APP_NAME": "InteractiveApp",
  "ENV": "dev",
  "NEW_VAR": "new_value"
}
EOF
`
	err = os.WriteFile(editorScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	// Setup environment for the test
	t.Setenv("EDITOR", editorScript)

	// Create a dummy host that doesn't actually do anything (we're going to use httptest)
	// But actually, we don't want to make an HTTP request and wait, we just want to verify
	// that submitPipelineJob is called with the *correct* finalVars.
	// Since submitPipelineJob performs an HTTP request, let's just set up a mock server.

	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/pipeline" {
			err := r.ParseForm()
			if err != nil {
				t.Fatalf("ParseForm error: %v", err)
			}

			vars := r.Form["var"]

			receivedPayload = make(map[string]interface{})
			for _, v := range vars {
				receivedPayload[v] = true // Just store what we got
			}

			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"submitted": ["job-1"], "errors": []}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Initial vars passed from CLI
	initialVars := map[string]string{
		"CLI_VAR": "cli_value",
		"ENV":     "should_be_overridden_by_interactive", // Testing priority
	}

	// Override exitFunc so we don't exit the test runner
	oldExitFunc := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = oldExitFunc }()

	// Call the function
	submitPipelineInteractiveJob(server.URL, pipelinePath, false, false, "", initialVars)

	// Verify the final variables that were sent via query string
	require.NotNil(t, receivedPayload, "Expected to receive a payload at /jobs/pipeline")

	assert.Contains(t, receivedPayload, "APP_NAME=InteractiveApp")
	assert.Contains(t, receivedPayload, "ENV=dev")
	assert.Contains(t, receivedPayload, "NEW_VAR=new_value")
	assert.Contains(t, receivedPayload, "CLI_VAR=cli_value")

	// Test missing file
	t.Run("MissingFile", func(t *testing.T) {
		exitCode := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		submitPipelineInteractiveJob(server.URL, filepath.Join(tmpDir, "does-not-exist.yaml"), false, false, "", nil)
		assert.Equal(t, 1, exitCode)
	})

	// Test invalid YAML
	t.Run("InvalidYAML", func(t *testing.T) {
		exitCode := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		invalidYAMLPath := filepath.Join(tmpDir, "invalid.yaml")
		os.WriteFile(invalidYAMLPath, []byte("invalid\n  yaml:\n-content"), 0644)

		submitPipelineInteractiveJob(server.URL, invalidYAMLPath, false, false, "", nil)
		assert.Equal(t, 1, exitCode)
	})

	// Test No Variables in YAML and Nil CLI Vars
	t.Run("NoVariables", func(t *testing.T) {
		exitCode := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		noVarsYAMLPath := filepath.Join(tmpDir, "no_vars.yaml")
		os.WriteFile(noVarsYAMLPath, []byte("name: No Vars\njobs:\n  test: {}\n"), 0644)

		submitPipelineInteractiveJob(server.URL, noVarsYAMLPath, false, false, "", nil)
		// It should successfully bypass interactive editing and submit directly
		// Since our mock server returns 202 Accepted, exitCode should remain 0
		assert.Equal(t, 0, exitCode)
	})

	// Test Editor Failure
	t.Run("EditorFailure", func(t *testing.T) {
		exitCode := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		failScript := filepath.Join(tmpDir, "fail_editor.sh")
		os.WriteFile(failScript, []byte("#!/bin/sh\nexit 1\n"), 0755)
		t.Setenv("EDITOR", failScript)

		submitPipelineInteractiveJob(server.URL, pipelinePath, false, false, "", nil)
		assert.Equal(t, 1, exitCode)
	})

	// Test Editor Outputs Invalid JSON
	t.Run("EditorInvalidJSON", func(t *testing.T) {
		exitCode := 0
		oldExit := exitFunc
		exitFunc = func(code int) { exitCode = code }
		defer func() { exitFunc = oldExit }()

		invalidJsonScript := filepath.Join(tmpDir, "invalid_json_editor.sh")
		os.WriteFile(invalidJsonScript, []byte("#!/bin/sh\necho \"invalid json\" > \"$1\"\n"), 0755)
		t.Setenv("EDITOR", invalidJsonScript)

		submitPipelineInteractiveJob(server.URL, pipelinePath, false, false, "", nil)
		assert.Equal(t, 1, exitCode)
	})
}
