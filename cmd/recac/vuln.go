package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/vuln"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	vulnJson         bool
	vulnFailCritical bool
	vulnFile         string
)

var vulnCmd = &cobra.Command{
	Use:   "vuln",
	Short: "Scan dependencies for known vulnerabilities",
	Long:  `Scans 'go.mod' and 'package.json' files for known vulnerabilities using the OSV (Open Source Vulnerability) database.`,
	RunE:  runVulnScan,
}

func init() {
	rootCmd.AddCommand(vulnCmd)
	vulnCmd.Flags().BoolVar(&vulnJson, "json", false, "Output results as JSON")
	vulnCmd.Flags().BoolVar(&vulnFailCritical, "fail-critical", false, "Exit with error code if vulnerabilities are found")
	vulnCmd.Flags().StringVarP(&vulnFile, "file", "f", "", "Specific file to scan (go.mod or package.json)")
}

func runVulnScan(cmd *cobra.Command, args []string) error {
	var packages []vuln.Package
	var err error

	// Determine files to scan
	filesToScan := []string{}
	if vulnFile != "" {
		filesToScan = append(filesToScan, vulnFile)
	} else {
		// Auto-detect
		if _, err := os.Stat("go.mod"); err == nil {
			filesToScan = append(filesToScan, "go.mod")
		}
		if _, err := os.Stat("package.json"); err == nil {
			filesToScan = append(filesToScan, "package.json")
		}
	}

	if len(filesToScan) == 0 {
		return fmt.Errorf("no dependency files found (go.mod, package.json)")
	}

	// Parse files
	for _, file := range filesToScan {
		var pkgs []vuln.Package
		base := filepath.Base(file)
		if base == "go.mod" || strings.HasSuffix(file, ".mod") {
			parser := &vuln.GoModParser{}
			pkgs, err = parser.Parse(file)
		} else if base == "package.json" || strings.HasSuffix(file, ".json") {
			parser := &vuln.PackageJsonParser{}
			pkgs, err = parser.Parse(file)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: unsupported file type: %s\n", file)
			continue
		}

		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", file, err)
		}
		packages = append(packages, pkgs...)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Scanning %d packages for vulnerabilities...\n", len(packages))

	// Scan
	scanner := vuln.NewOSVClient()
	vulnerabilities, err := scanner.Scan(packages)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Output
	if vulnJson {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(vulnerabilities)
	}

	printVulnReport(cmd, vulnerabilities)

	if vulnFailCritical && len(vulnerabilities) > 0 {
		return fmt.Errorf("found %d vulnerabilities", len(vulnerabilities))
	}

	return nil
}

func printVulnReport(cmd *cobra.Command, vulns []vuln.Vulnerability) {
	if len(vulns) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No vulnerabilities found!")
		return
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPACKAGE\tSEVERITY\tSUMMARY")
	fmt.Fprintln(w, "--\t-------\t--------\t-------")

	for _, v := range vulns {
		summary := v.Summary
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}
		if summary == "" {
			summary = "No summary available"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.ID, v.PackageName, v.Severity, summary)
	}
	w.Flush()
	fmt.Fprintf(cmd.OutOrStdout(), "\nFound %d vulnerabilities.\n", len(vulns))
}
