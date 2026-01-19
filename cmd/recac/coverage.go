package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	coverageHTML      bool
	coverageAnalyze   bool
	coverageThreshold float64
	coveragePackage   string
	coverageKeep      bool
)

// Mockable functions for testing
var (
	runTestsFunc = func(args []string) error {
		cmd := exec.Command("go", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	runCoverToolFunc = func(args []string) ([]byte, error) {
		cmd := exec.Command("go", args...)
		return cmd.Output()
	}
)

var coverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Analyze test coverage",
	Long:  `Runs tests with coverage and provides analysis.
Can open an HTML report or use AI to analyze uncovered functions.

Note: The default package is "." (current directory).
To test multiple packages (e.g., "./..."), ensure your Go version supports
generating a single profile for multiple packages, or use the tool
in each directory.`,
	RunE: runCoverage,
}

func init() {
	rootCmd.AddCommand(coverageCmd)
	coverageCmd.Flags().BoolVar(&coverageHTML, "html", false, "Open HTML coverage report")
	coverageCmd.Flags().BoolVar(&coverageAnalyze, "analyze", false, "Use AI to analyze coverage gaps")
	coverageCmd.Flags().Float64Var(&coverageThreshold, "threshold", 80.0, "Coverage threshold percentage to flag")
	coverageCmd.Flags().StringVarP(&coveragePackage, "package", "p", ".", "Package to test (default: current directory)")
	coverageCmd.Flags().BoolVar(&coverageKeep, "keep", false, "Keep the coverage.out file after running")
}

func runCoverage(cmd *cobra.Command, args []string) error {
	coverageFile := "coverage.out"
	if !coverageKeep {
		defer os.Remove(coverageFile)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Running tests for %s...\n", coveragePackage)

	// 1. Run Tests
	testArgs := []string{"test", "-coverprofile=" + coverageFile, coveragePackage}
	if err := runTestsFunc(testArgs); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	// 2. Open HTML if requested
	if coverageHTML {
		fmt.Fprintln(cmd.OutOrStdout(), "Opening HTML report...")
		return runTestsFunc([]string{"tool", "cover", "-html=" + coverageFile})
	}

	// 3. Parse Coverage
	output, err := runCoverToolFunc([]string{"tool", "cover", "-func=" + coverageFile})
	if err != nil {
		return fmt.Errorf("failed to parse coverage profile: %w", err)
	}

	funcs, totalCov, err := parseCoverageOutput(string(output))
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal Coverage: %.1f%%\n", totalCov)

	// Filter low coverage functions
	var lowCovFuncs []CoverageEntry
	for _, f := range funcs {
		if f.Percent < coverageThreshold {
			lowCovFuncs = append(lowCovFuncs, f)
		}
	}

	if len(lowCovFuncs) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nFunctions below %.1f%% coverage:\n", coverageThreshold)
		w := bufio.NewWriter(cmd.OutOrStdout())
		fmt.Fprintln(w, "pkg\tfunction\tcoverage")
		for _, f := range lowCovFuncs {
			fmt.Fprintf(w, "%s\t%s\t%.1f%%\n", shortenPkg(f.Pkg), f.Func, f.Percent)
		}
		w.Flush()
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nAll functions meet the coverage threshold! 🎉")
	}

	// 4. AI Analysis
	if coverageAnalyze && len(lowCovFuncs) > 0 {
		return analyzeCoverageGaps(cmd, lowCovFuncs)
	}

	return nil
}

type CoverageEntry struct {
	Pkg     string
	Func    string
	Percent float64
}

func parseCoverageOutput(output string) ([]CoverageEntry, float64, error) {
	var entries []CoverageEntry
	var total float64

	scanner := bufio.NewScanner(bytes.NewBufferString(output))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Example: github.com/user/repo/pkg/file.go:10:	MyFunc		80.0%
		// Last line: total:							(statements)	75.0%

		pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			continue
		}

		if fields[0] == "total:" {
			total = pct
			continue
		}

		// fields[0] is file:line
		// fields[1] is function name
		// But sometimes package path is involved. `go tool cover -func` output format:
		// package/path/file.go:line: functionName    100.0%

		// We assume standard format
		if len(fields) == 3 {
			// pkg/file.go:line function percent
			fileLine := fields[0]
			parts := strings.Split(fileLine, "/")
			pkg := strings.Join(parts[:len(parts)-1], "/") // rough pkg guess

			entries = append(entries, CoverageEntry{
				Pkg:     pkg,
				Func:    fields[1],
				Percent: pct,
			})
		}
	}

	return entries, total, nil
}

func shortenPkg(pkg string) string {
	parts := strings.Split(pkg, "/")
	if len(parts) > 2 {
		return ".../" + strings.Join(parts[len(parts)-2:], "/")
	}
	return pkg
}

func analyzeCoverageGaps(cmd *cobra.Command, funcs []CoverageEntry) error {
	ctx := context.Background()
	cwd, _ := os.Getwd()

	// Limit to top 20 to save tokens
	if len(funcs) > 20 {
		funcs = funcs[:20]
	}

	var sb strings.Builder
	for _, f := range funcs {
		sb.WriteString(fmt.Sprintf("- %s: %s (%.1f%%)\n", f.Pkg, f.Func, f.Percent))
	}

	prompt := fmt.Sprintf(`Analyze the following list of Go functions with low test coverage.
Identify which ones are likely critical business logic, security-sensitive, or complex error handling that MUST be tested.
Prioritize the top 5 functions to write tests for.

Low Coverage Functions:
%s

Format the output as a prioritized list with a brief reason for each.`, sb.String())

	fmt.Fprintln(cmd.OutOrStdout(), "\n🤖 Analyzing coverage gaps with AI...")

	ag, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "recac-coverage")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Println("")

	return err
}
