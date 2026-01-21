package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// flakyExec allows mocking exec.Command in tests
var flakyExec = exec.Command

var (
	flakyCount   int
	flakyTimeout time.Duration
	flakyJSON    bool
)

var flakyCmd = &cobra.Command{
	Use:   "flaky [packages]",
	Short: "Detect flaky tests by running them multiple times",
	Long: `Runs tests multiple times to detect flakiness.
It executes 'go test -json' repeatedly and aggregates the results.
If a test passes some times and fails others, it is marked as flaky.

Example:
  recac flaky ./... --count 10
  recac flaky ./internal/mypkg --count 5
`,
	RunE: runFlaky,
}

func init() {
	rootCmd.AddCommand(flakyCmd)
	flakyCmd.Flags().IntVarP(&flakyCount, "count", "c", 5, "Number of times to run tests")
	flakyCmd.Flags().DurationVar(&flakyTimeout, "timeout", 30*time.Second, "Timeout per test run")
	flakyCmd.Flags().BoolVar(&flakyJSON, "json", false, "Output results as JSON")
}

type TestEvent struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

type TestStat struct {
	Name    string
	Package string
	Pass    int
	Fail    int
	Skip    int
}

type FlakyResult struct {
	FlakyTests []TestStat `json:"flaky_tests"`
	Stats      RunStats   `json:"stats"`
}

type RunStats struct {
	TotalRuns     int           `json:"total_runs"`
	TotalDuration time.Duration `json:"total_duration"`
	TestsFound    int           `json:"tests_found"`
	FlakyFound    int           `json:"flaky_found"`
}

func runFlaky(cmd *cobra.Command, args []string) error {
	packages := args
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	testStats := make(map[string]*TestStat)
	startTime := time.Now()

	fmt.Fprintf(cmd.ErrOrStderr(), "Running tests %d times (timeout: %s)...\n", flakyCount, flakyTimeout)

	for i := 1; i <= flakyCount; i++ {
		fmt.Fprintf(cmd.ErrOrStderr(), "Run %d/%d... ", i, flakyCount)

		// Run go test -json
		runArgs := []string{"test", "-json", fmt.Sprintf("-timeout=%s", flakyTimeout)}
		runArgs = append(runArgs, packages...)

		c := flakyExec("go", runArgs...)
		// We capture stdout because that's where -json output goes
		var out bytes.Buffer
		c.Stdout = &out
		// We ignore stderr for now, or maybe print it if verbose?
		// go test sends build errors to stderr, json to stdout.

		err := c.Run()
		// We don't return error immediately if tests fail, because that's what we are looking for.
		// However, if build failed, we should probably stop.
		// But 'go test -json' outputs "fail" action on build failure too.

		// Parse JSON output
		scanner := bufio.NewScanner(&out)
		runFailed := false
		for scanner.Scan() {
			var event TestEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				// Non-JSON line (maybe build output mixed in?)
				continue
			}

			if event.Test == "" {
				// Package-level event
				if event.Action == "fail" {
					runFailed = true
				}
				continue
			}

			key := event.Package + "." + event.Test
			stat, exists := testStats[key]
			if !exists {
				stat = &TestStat{Name: event.Test, Package: event.Package}
				testStats[key] = stat
			}

			if event.Action == "pass" {
				stat.Pass++
			} else if event.Action == "fail" {
				stat.Fail++
			} else if event.Action == "skip" {
				stat.Skip++
			}
		}

		if err != nil && !runFailed {
			// Cmd failed but we didn't see a "fail" action? Could be build error or other crash.
			fmt.Fprintf(cmd.ErrOrStderr(), "Process error: %v\n", err)
		} else if runFailed {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed\n")
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Passed\n")
		}
	}

	// Analyze results
	var flakyTests []TestStat
	for _, stat := range testStats {
		if stat.Fail > 0 && stat.Pass > 0 {
			flakyTests = append(flakyTests, *stat)
		}
	}

	// Sort for consistent output
	sort.Slice(flakyTests, func(i, j int) bool {
		if flakyTests[i].Package == flakyTests[j].Package {
			return flakyTests[i].Name < flakyTests[j].Name
		}
		return flakyTests[i].Package < flakyTests[j].Package
	})

	duration := time.Since(startTime)
	stats := RunStats{
		TotalRuns:     flakyCount,
		TotalDuration: duration,
		TestsFound:    len(testStats),
		FlakyFound:    len(flakyTests),
	}

	if flakyJSON {
		res := FlakyResult{
			FlakyTests: flakyTests,
			Stats:      stats,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n--- Flakiness Report ---")
	fmt.Fprintf(cmd.OutOrStdout(), "Total Runs: %d\n", flakyCount)
	fmt.Fprintf(cmd.OutOrStdout(), "Total Duration: %s\n", duration.Round(time.Millisecond))
	fmt.Fprintf(cmd.OutOrStdout(), "Tests Found: %d\n", len(testStats))
	fmt.Fprintf(cmd.OutOrStdout(), "Flaky Tests Found: %d\n", len(flakyTests))

	if len(flakyTests) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nDetected Flaky Tests:")
		for _, t := range flakyTests {
			flakiness := float64(t.Fail) / float64(t.Fail+t.Pass) * 100
			fmt.Fprintf(cmd.OutOrStdout(), "- %s.%s (Fail: %d, Pass: %d, Skip: %d) - %.1f%% Fail Rate\n",
				t.Package, t.Name, t.Fail, t.Pass, t.Skip, flakiness)
		}
		// Return error to indicate flakiness was found (useful for CI)
		return fmt.Errorf("found %d flaky tests", len(flakyTests))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\n✅ No flaky tests detected.")
	}

	return nil
}
