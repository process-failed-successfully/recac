package main

import (
	"bytes"
	"recac/internal/vuln"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPrintVulnReport(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	t.Run("No Vulnerabilities", func(t *testing.T) {
		buf.Reset()
		printVulnReport(cmd, []vuln.Vulnerability{})
		assert.Contains(t, buf.String(), "No vulnerabilities found")
	})

	t.Run("With Vulnerabilities", func(t *testing.T) {
		buf.Reset()
		vulns := []vuln.Vulnerability{
			{ID: "GHSA-123", PackageName: "pkg1", Severity: "HIGH", Summary: "Bad bug"},
			{ID: "GHSA-456", PackageName: "pkg2", Severity: "LOW", Summary: "Minor bug"},
		}
		printVulnReport(cmd, vulns)
		assert.Contains(t, buf.String(), "GHSA-123")
		assert.Contains(t, buf.String(), "pkg1")
		assert.Contains(t, buf.String(), "HIGH")
		assert.Contains(t, buf.String(), "Bad bug")
		assert.Contains(t, buf.String(), "Found 2 vulnerabilities")
	})

	t.Run("Long Summary Truncation", func(t *testing.T) {
		buf.Reset()
		longSummary := "This is a very long summary that should be truncated because it exceeds the limit of characters allowed in the table output."
		vulns := []vuln.Vulnerability{
			{ID: "GHSA-789", PackageName: "pkg3", Severity: "MEDIUM", Summary: longSummary},
		}
		printVulnReport(cmd, vulns)
		assert.Contains(t, buf.String(), "...")
		assert.NotContains(t, buf.String(), "output.") // Last word shouldn't be there if truncated
	})

	t.Run("Empty Summary", func(t *testing.T) {
		buf.Reset()
		vulns := []vuln.Vulnerability{
			{ID: "GHSA-000", PackageName: "pkg4", Severity: "LOW", Summary: ""},
		}
		printVulnReport(cmd, vulns)
		assert.Contains(t, buf.String(), "No summary available")
	})
}
