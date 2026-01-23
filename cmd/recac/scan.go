package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/db"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type ScanResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

var (
	scanJSON       bool
	scanExportPlan string
	markerRegex    = regexp.MustCompile(`(?i)\b(TODO|FIXME|BUG|HACK|XXX)\b(\((.+?)\))?:?\s*(.*)`)
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan codebase for technical debt markers",
	Long:  `Scans the current directory recursively for TODO, FIXME, BUG, HACK, and XXX markers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := runScan(".")
		if err != nil {
			return err
		}

		if len(results) == 0 {
			if !scanJSON {
				fmt.Fprintln(cmd.OutOrStdout(), "No markers found. Great job!")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
			}
			return nil
		}

		if scanExportPlan != "" {
			return exportPlan(results, scanExportPlan)
		}

		if scanJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}

		printTable(cmd, results)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().BoolVar(&scanJSON, "json", false, "Output results as JSON")
	scanCmd.Flags().StringVar(&scanExportPlan, "export-plan", "", "Export results as a feature list plan to the specified file (e.g., plan.json)")
}

func runScan(root string) ([]ScanResult, error) {
	var results []ScanResult

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

		// Check for markers
		fileResults, _ := scanFile(path)
		// We ignore the error but keep whatever results we found so far
		results = append(results, fileResults...)

		return nil
	})

	return results, err
}

func scanFile(path string) ([]ScanResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []ScanResult
	scanner := bufio.NewScanner(f)
	lineNum := 0

	// Reusable buffer for uppercase conversion
	var upperBuf []byte

	// Byte slices for keywords
	kwTODO := []byte("TODO")
	kwFIXME := []byte("FIXME")
	kwBUG := []byte("BUG")
	kwHACK := []byte("HACK")
	kwXXX := []byte("XXX")

	for scanner.Scan() {
		lineNum++
		lineBytes := scanner.Bytes()

		// Resize upperBuf if needed
		if cap(upperBuf) < len(lineBytes) {
			upperBuf = make([]byte, len(lineBytes)*2) // Grow with some slack
		}
		upperBuf = upperBuf[:len(lineBytes)]

		// Copy and ToUpper inline
		for i, b := range lineBytes {
			if 'a' <= b && b <= 'z' {
				upperBuf[i] = b - 'a' + 'A'
			} else {
				upperBuf[i] = b
			}
		}

		// Check keywords on upperBuf
		if !bytes.Contains(upperBuf, kwTODO) &&
			!bytes.Contains(upperBuf, kwFIXME) &&
			!bytes.Contains(upperBuf, kwBUG) &&
			!bytes.Contains(upperBuf, kwHACK) &&
			!bytes.Contains(upperBuf, kwXXX) {
			continue
		}

		// Match regex on original lineBytes
		matches := markerRegex.FindSubmatch(lineBytes)
		if matches != nil {
			// matches[1] = TYPE (e.g. TODO)
			// matches[3] = Author/Context (optional, inside parens)
			// matches[4] = Message

			var msg string
			if len(matches) > 4 && matches[4] != nil {
				msg = string(bytes.TrimSpace(matches[4]))
			}

			if len(matches) > 3 && matches[3] != nil {
				author := string(matches[3])
				if author != "" {
					msg = fmt.Sprintf("[%s] %s", author, msg)
				}
			}
			if msg == "" {
				msg = "No description provided"
			}

			results = append(results, ScanResult{
				File:    path,
				Line:    lineNum,
				Type:    strings.ToUpper(string(matches[1])),
				Message: msg,
			})
		}
	}

	return results, scanner.Err()
}

func printTable(cmd *cobra.Command, results []ScanResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tFILE\tLINE\tMESSAGE")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", r.Type, r.File, r.Line, r.Message)
	}
	w.Flush()
}

func exportPlan(results []ScanResult, outputFile string) error {
	var features []db.Feature
	projectName := filepath.Base(outputFile)
	if projectName == "." || projectName == "/" {
		cwd, _ := os.Getwd()
		projectName = filepath.Base(cwd)
	}

	for i, r := range results {
		feature := db.Feature{
			ID:          fmt.Sprintf("tech-debt-%d", i+1),
			Category:    "Technical Debt",
			Priority:    "Low", // Default to low
			Status:      "pending",
			Description: fmt.Sprintf("%s in %s:%d: %s", r.Type, r.File, r.Line, r.Message),
			Steps: []string{
				fmt.Sprintf("Locate the %s marker in %s at line %d", r.Type, r.File, r.Line),
				"Analyze the context and determine the necessary changes",
				"Implement the fix or improvement",
				"Remove the marker",
			},
		}

		if r.Type == "BUG" || r.Type == "FIXME" {
			feature.Priority = "Medium"
		}

		features = append(features, feature)
	}

	plan := db.FeatureList{
		ProjectName: projectName,
		Features:    features,
	}

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan file: %w", err)
	}

	fmt.Printf("Successfully exported %d items to %s\n", len(features), outputFile)
	return nil
}
