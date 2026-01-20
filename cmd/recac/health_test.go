package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestHealthCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-health-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a complex Go file
	complexCode := `package main
    func complex(n int) int {
        if n > 0 {
            if n > 1 {
                if n > 2 {
                    if n > 3 {
                         for i:=0; i<10; i++ {
                             if i % 2 == 0 {
                                 return 1
                             }
                         }
                    }
                }
            }
        }
        return 0
    }
    `
	os.WriteFile(filepath.Join(tmpDir, "complex.go"), []byte(complexCode), 0644)

	// Create a file with TODO
	todoCode := `package main
    // TODO: Fix this later
    func foo() {}
    `
	os.WriteFile(filepath.Join(tmpDir, "todo.go"), []byte(todoCode), 0644)

	// Create a file with Security Issue (dummy secret)
	secCode := `package main
    var awsKey = "AKIAIOSFODNN7EXAMPLE"
    `
	os.WriteFile(filepath.Join(tmpDir, "sec.go"), []byte(secCode), 0644)

    // Call runHealth directly
    t.Run("JSON Output", func(t *testing.T) {
        healthJSON = true
        healthThreshold = 5

        cmd := &cobra.Command{}
        buf := new(bytes.Buffer)
        cmd.SetOut(buf)

        // Execute with args
        err = runHealth(cmd, []string{tmpDir})
        assert.NoError(t, err)

        // Verify Output
        var report HealthReport
        err = json.Unmarshal(buf.Bytes(), &report)
        assert.NoError(t, err, "Failed to parse JSON output: %s", buf.String())

        // Assert Overview
        assert.Equal(t, 1, report.Overview.TodoCount, "Should find 1 TODO")
        assert.GreaterOrEqual(t, report.Overview.SecurityIssues, 1, "Should find at least 1 security issue")
        assert.GreaterOrEqual(t, report.Overview.HighComplexityFunc, 1, "Should find at least 1 complex function")
    })

    t.Run("Text Output", func(t *testing.T) {
        healthJSON = false
        healthThreshold = 5

        cmd := &cobra.Command{}
        buf := new(bytes.Buffer)
        cmd.SetOut(buf)

        // Execute with args
        err = runHealth(cmd, []string{tmpDir})
        assert.NoError(t, err)

        output := buf.String()
        assert.Contains(t, output, "Project Health Report")
        assert.Contains(t, output, "Score:")
        assert.Contains(t, output, "TODOs:")
        assert.Contains(t, output, "Security Issues:")
        // Check for specific findings in text
        assert.Contains(t, output, "complex") // function name
        assert.Contains(t, output, "Fix this later") // TODO content
    })
}
