package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportCommand(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "recac-report-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy Go file with some complexity and smells
	goContent := `package main

import "fmt"

func complexFunc(n int) int {
	// TODO: Simplify this
	if n > 0 {
		if n > 1 {
			if n > 2 {
				if n > 3 {
					if n > 4 {
						if n > 5 {
							if n > 6 {
								if n > 7 {
									if n > 8 {
										if n > 9 {
											return n
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return 0
}

func duplicatedFunc() {
	fmt.Println("line 1")
	fmt.Println("line 2")
	fmt.Println("line 3")
	fmt.Println("line 4")
	fmt.Println("line 5")
	fmt.Println("line 6")
	fmt.Println("line 7")
	fmt.Println("line 8")
	fmt.Println("line 9")
	fmt.Println("line 10")
}

func duplicatedFunc2() {
	fmt.Println("line 1")
	fmt.Println("line 2")
	fmt.Println("line 3")
	fmt.Println("line 4")
	fmt.Println("line 5")
	fmt.Println("line 6")
	fmt.Println("line 7")
	fmt.Println("line 8")
	fmt.Println("line 9")
	fmt.Println("line 10")
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goContent), 0644)
	require.NoError(t, err)

	// Reset flags
	reportFormat = "json"
	reportOutput = filepath.Join(tmpDir, "report.json")
	reportOpen = false

	// Capture output
	cmd := reportCmd
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	// Run report command
	err = runReport(cmd, []string{tmpDir})
	require.NoError(t, err)

	// Verify JSON output
	require.FileExists(t, reportOutput)
	jsonBytes, err := os.ReadFile(reportOutput)
	require.NoError(t, err)

	var report ReportData
	err = json.Unmarshal(jsonBytes, &report)
	require.NoError(t, err)

	// Check Stats
	assert.Equal(t, 1, report.Stats.TotalFiles)
	assert.Equal(t, 1, report.Stats.TotalGoFiles)

	// Check Complexity
	// complexFunc has many nested ifs, should be high complexity
	// Complexity = 1 (base) + 10 (ifs) = 11?
	assert.NotEmpty(t, report.Complexity)
	foundComplex := false
	for _, c := range report.Complexity {
		if c.Function == "complexFunc" {
			assert.GreaterOrEqual(t, c.Complexity, 10)
			foundComplex = true
		}
	}
	assert.True(t, foundComplex, "Should detect complexFunc")

	// Check Smells
	// complexFunc is also deeply nested
	assert.NotEmpty(t, report.Smells)
	foundDeepNesting := false
	for _, s := range report.Smells {
		if s.Function == "complexFunc" && s.Type == "Deep Nesting" {
			foundDeepNesting = true
		}
	}
	assert.True(t, foundDeepNesting, "Should detect Deep Nesting")

	// Check CPD
	// duplicatedFunc and duplicatedFunc2 are identical and > 10 lines
	assert.NotEmpty(t, report.Duplications)
	assert.Equal(t, 1, len(report.Duplications))
	assert.Equal(t, 11, report.Duplications[0].LineCount)

	// Check TODOs
	assert.NotEmpty(t, report.Todos)
	assert.Equal(t, 1, len(report.Todos))
	assert.Equal(t, "TODO", report.Todos[0].Keyword)
	assert.Equal(t, "Simplify this", report.Todos[0].Content)

	// Check Health Score
	assert.Less(t, report.HealthScore, 100, "Health score should be reduced")
}

func TestReportHTMLGeneration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-report-html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	outFile := filepath.Join(tmpDir, "report.html")

	data := ReportData{
		GeneratedAt: time.Now(),
		ProjectName: "TestProject",
		Stats: ProjectStats{
			TotalFiles: 10,
		},
		HealthScore: 95,
	}

	err = generateHTMLReport(data, outFile)
	require.NoError(t, err)

	content, err := os.ReadFile(outFile)
	require.NoError(t, err)
	html := string(content)

	assert.Contains(t, html, "TestProject")
	assert.Contains(t, html, "Health Score")
	assert.Contains(t, html, "95")
}

// Mocking exec.Command for openBrowser is tricky in same package if not using a helper variable.
// But openBrowser is not called if reportOpen is false.
