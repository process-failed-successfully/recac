package main

import (
	"bytes"
	"os"
	"recac/internal/vuln"
	"strings"
	"testing"
	"github.com/spf13/cobra"
)

func TestVulnCmd(t *testing.T) {
	// Create a temporary directory for test
	tempDir, err := os.MkdirTemp("", "vuln-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp dir
	wd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}
	defer os.Chdir(wd)

	// Create dummy go.mod
	goMod := `module test
go 1.20
require github.com/gin-gonic/gin v1.9.0
`
	if err := os.WriteFile("go.mod", []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create dummy package.json
	pkgJson := `{"dependencies": {"express": "4.17.1"}}`
	if err := os.WriteFile("package.json", []byte(pkgJson), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	cmd := vulnCmd
	if cmd.Use != "vuln" {
		t.Errorf("Expected use 'vuln', got '%s'", cmd.Use)
	}

	// Verify flags
	if flag := cmd.Flags().Lookup("json"); flag == nil {
		t.Error("Missing json flag")
	}
	if flag := cmd.Flags().Lookup("fail-critical"); flag == nil {
		t.Error("Missing fail-critical flag")
	}
}

func TestVulnCmd_NoFiles(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "vuln-test-empty")
	defer os.RemoveAll(tempDir)

	wd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(wd)

	cmd := vulnCmd
	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetIn(nil)

	err := runVulnScan(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no files found, got nil")
	}
	if !strings.Contains(err.Error(), "no dependency files found") {
		t.Errorf("Expected 'no dependency files found' error, got: %v", err)
	}
}

func TestVulnCmd_SpecificFile(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "vuln-test-file")
	defer os.RemoveAll(tempDir)

	wd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(wd)

	// Create custom file
	os.WriteFile("my-go.mod", []byte("module test\nrequire pkg v1.0.0"), 0644)

	// Set flag
	vulnCmd.Flags().Set("file", "my-go.mod")
	defer vulnCmd.Flags().Set("file", "") // Reset

	// We expect this to try to run scan. Network may fail.
	err := runVulnScan(vulnCmd, []string{})
	// We just want to ensure it parses the file.
	// If it failed to scan (network), error would be "scan failed".
	if err != nil {
		if strings.Contains(err.Error(), "unsupported file type") {
			t.Error("Should support .mod extension")
		}
	}
}

func TestPrintVulnReport(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Case 1: No vulnerabilities
	printVulnReport(cmd, []vuln.Vulnerability{})
	if !strings.Contains(buf.String(), "No vulnerabilities found") {
		t.Errorf("Expected 'No vulnerabilities found', got: %s", buf.String())
	}

	// Case 2: Vulnerabilities found
	buf.Reset()
	vulns := []vuln.Vulnerability{
		{
			ID:          "GHSA-1234",
			PackageName: "example-lib",
			Severity:    "HIGH",
			Summary:     "A bad vulnerability",
		},
		{
			ID:          "GHSA-5678",
			PackageName: "other-lib",
			Severity:    "LOW",
			Summary:     strings.Repeat("A", 100), // Long summary
		},
	}
	printVulnReport(cmd, vulns)

	output := buf.String()
	if !strings.Contains(output, "GHSA-1234") {
		t.Error("Expected output to contain GHSA-1234")
	}
	if !strings.Contains(output, "example-lib") {
		t.Error("Expected output to contain example-lib")
	}
	if !strings.Contains(output, "HIGH") {
		t.Error("Expected output to contain HIGH")
	}
	if !strings.Contains(output, "Found 2 vulnerabilities") {
		t.Error("Expected output to contain summary count")
	}
	// Check truncation
	if strings.Contains(output, strings.Repeat("A", 100)) {
		t.Error("Expected summary to be truncated")
	}
	if !strings.Contains(output, "AAAAA...") { // Check for ellipsis
		t.Error("Expected ellipsis in truncated summary")
	}
}
