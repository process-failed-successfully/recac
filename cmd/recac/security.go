package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/security"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	securityJSON bool
	securityFail bool
)

var securityCmd = &cobra.Command{
	Use:   "security",
	Short: "Scan codebase for security vulnerabilities and secrets",
	Long:  `Scans the current directory recursively for potential security issues, including hardcoded secrets, keys, and dangerous command patterns.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scanner := security.NewRegexScanner()
		results, err := runSecurityScan(".", scanner)
		if err != nil {
			return err
		}

		if len(results) == 0 {
			if !securityJSON {
				fmt.Fprintln(cmd.OutOrStdout(), "No security issues found. Great job!")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
			}
			return nil
		}

		if securityJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return err
			}
		} else {
			printSecurityTable(cmd, results)
		}

		if securityFail {
			return fmt.Errorf("security scan failed with %d findings", len(results))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(securityCmd)
	securityCmd.Flags().BoolVar(&securityJSON, "json", false, "Output results as JSON")
	securityCmd.Flags().BoolVar(&securityFail, "fail", false, "Exit with error code if findings are detected")
}

type SecurityResult struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Match       string `json:"match"`
}

func runSecurityScan(root string, scanner *security.RegexScanner) ([]SecurityResult, error) {
	var results []SecurityResult

	// Common directories to ignore
	ignoredDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		".recac":       true,
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary files and likely large files (simple check)
		if info.Size() > 1024*1024 { // Skip files > 1MB
			return nil
		}

		// Scan file
		fileResults, err := scanFileForSecurity(path, scanner)
		if err != nil {
			// Log error but continue scanning other files?
			// For now, let's just ignore read errors on single files
			return nil
		}
		results = append(results, fileResults...)

		return nil
	})

	return results, err
}

func scanFileForSecurity(path string, scanner *security.RegexScanner) ([]SecurityResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read entire file content
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	findings, err := scanner.Scan(string(content))
	if err != nil {
		return nil, err
	}

	var results []SecurityResult
	for _, finding := range findings {
		results = append(results, SecurityResult{
			File:        path,
			Line:        finding.Line,
			Type:        finding.Type,
			Description: finding.Description,
			Match:       finding.Match,
		})
	}

	return results, nil
}

func printSecurityTable(cmd *cobra.Command, results []SecurityResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tFILE\tLINE\tDESCRIPTION")
	for _, r := range results {
		// Truncate description if too long?
		desc := r.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", r.Type, r.File, r.Line, desc)
	}
	w.Flush()
}
