package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateChangelog(t *testing.T) {
	// Mock exitFunc
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	// Mock stdout
	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("SuccessStdout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/changelog/generate", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"changelog": "# Changelog\n\n- Fix bug"}`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		generateChangelog(server.URL, "-", "", "", "", "")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "# Changelog\n\n- Fix bug")
	})

	t.Run("SuccessFile", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"changelog": "# Output to file"}`))
		}))
		defer server.Close()

		tmpFile, _ := os.CreateTemp("", "changelog.md")
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		exitCode = 0
		buf.Reset()
		generateChangelog(server.URL, tmpFile.Name(), "", "", "", "")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Changelog successfully written to")

		content, _ := os.ReadFile(tmpFile.Name())
		assert.Contains(t, string(content), "# Output to file")
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		generateChangelog(server.URL, "-", "", "", "", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to generate changelog")
	})
}
