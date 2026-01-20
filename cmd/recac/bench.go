package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// BenchResult represents a single benchmark line result
type BenchResult struct {
	Name        string  `json:"name"`
	Iterations  int64   `json:"iterations"`
	NsPerOp     float64 `json:"ns_per_op"`
	MBPerSec    float64 `json:"mb_per_sec,omitempty"`
	BytesPerOp  int64   `json:"bytes_per_op,omitempty"`
	AllocsPerOp int64   `json:"allocs_per_op,omitempty"`
}

// BenchRun represents a collection of results from one run
type BenchRun struct {
	Timestamp time.Time     `json:"timestamp"`
	Commit    string        `json:"commit,omitempty"`
	Results   []BenchResult `json:"results"`
}

var (
	benchSave      bool
	benchCompare   bool
	benchThreshold float64
	benchFile      string
)

// benchExecCommand allows mocking in tests.
// We use a specific variable to avoid conflict with other commands using execCommand.
var benchExecCommand = exec.Command

var benchCmd = &cobra.Command{
	Use:   "bench [packages]",
	Short: "Run benchmarks and track performance over time",
	Long: `Executes 'go test -bench' for the specified packages (defaulting to ./...)
and parses the output. It can save results to a file and compare them against
previous runs to detect performance regressions.`,
	RunE: runBench,
}

func init() {
	rootCmd.AddCommand(benchCmd)
	benchCmd.Flags().BoolVar(&benchSave, "save", false, "Save results to history")
	benchCmd.Flags().BoolVar(&benchCompare, "compare", true, "Compare with previous saved results")
	benchCmd.Flags().Float64Var(&benchThreshold, "threshold", 10.0, "Percentage threshold for regression warning")
	benchCmd.Flags().StringVar(&benchFile, "file", ".recac/benchmarks.json", "File to store benchmark history")
}

func runBench(cmd *cobra.Command, args []string) error {
	packages := args
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	// 1. Prepare command
	// go test -bench=. -benchmem -run=^$ <packages>
	benchArgs := []string{"test", "-bench=.", "-benchmem", "-run=^$"}
	benchArgs = append(benchArgs, packages...)

	fmt.Fprintf(cmd.OutOrStdout(), "Running benchmarks: go %s\n", strings.Join(benchArgs, " "))

	runCmd := benchExecCommand("go", benchArgs...)

	// We capture stdout to parse it, but also want to show it to the user?
	// The requirement implies parsing. We can pipe it.
	// For simplicity, let's capture it.
	var outBuf bytes.Buffer
	runCmd.Stdout = &outBuf
	runCmd.Stderr = cmd.ErrOrStderr()

	if err := runCmd.Run(); err != nil {
		// print output if failed
		fmt.Fprintln(cmd.OutOrStdout(), outBuf.String())
		return fmt.Errorf("benchmark command failed: %w", err)
	}

	output := outBuf.String()
	//fmt.Fprintln(cmd.OutOrStdout(), output) // Optional: show raw output

	// 2. Parse output
	results, err := parseBenchOutput(output)
	if err != nil {
		return fmt.Errorf("failed to parse benchmark output: %w", err)
	}

	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No benchmarks found.")
		return nil
	}

	// 3. Load history if comparing or saving
	var history []BenchRun
	if benchCompare || benchSave {
		history, err = loadBenchHistory(benchFile)
		if err != nil {
			// If file doesn't exist, that's fine for first run
			if !os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to load history: %v\n", err)
			}
		}
	}

	// 4. Compare
	if benchCompare && len(history) > 0 {
		lastRun := history[len(history)-1]
		printComparison(cmd, results, lastRun.Results, benchThreshold)
	} else {
		printResults(cmd, results)
	}

	// 5. Save
	if benchSave {
		newRun := BenchRun{
			Timestamp: time.Now(),
			Results:   results,
		}
		// Try to get git commit hash
		if commit, err := getGitCommit(); err == nil {
			newRun.Commit = commit
		}

		history = append(history, newRun)
		if err := saveBenchHistory(benchFile, history); err != nil {
			return fmt.Errorf("failed to save history: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nResults saved to %s\n", benchFile)
	}

	return nil
}

func parseBenchOutput(output string) ([]BenchResult, error) {
	var results []BenchResult
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Regex for standard go bench output
	// BenchmarkName-8   	10000000	       123 ns/op	      10 B/op	       1 allocs/op
	// Optional fields: MB/s
	// We use (Benchmark.*?) to capture the name non-greedily until the optional -N suffix or whitespace
	re := regexp.MustCompile(`^(Benchmark.*?)(?:-\d+)?\s+(\d+)\s+(\d+(?:\.\d+)?)\s+ns/op(?:\s+(\d+(?:\.\d+)?)\s+MB/s)?(?:\s+(\d+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		res := BenchResult{
			Name: strings.TrimSpace(matches[1]),
		}

		// Iterations
		if iter, err := strconv.ParseInt(matches[2], 10, 64); err == nil {
			res.Iterations = iter
		}

		// NsPerOp
		if ns, err := strconv.ParseFloat(matches[3], 64); err == nil {
			res.NsPerOp = ns
		}

		// MB/s (index 4)
		if len(matches) > 4 && matches[4] != "" {
			if mb, err := strconv.ParseFloat(matches[4], 64); err == nil {
				res.MBPerSec = mb
			}
		}

		// B/op (index 5)
		if len(matches) > 5 && matches[5] != "" {
			if b, err := strconv.ParseInt(matches[5], 10, 64); err == nil {
				res.BytesPerOp = b
			}
		}

		// Allocs/op (index 6)
		if len(matches) > 6 && matches[6] != "" {
			if a, err := strconv.ParseInt(matches[6], 10, 64); err == nil {
				res.AllocsPerOp = a
			}
		}

		results = append(results, res)
	}

	return results, scanner.Err()
}

func loadBenchHistory(path string) ([]BenchRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var history []BenchRun
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return history, nil
}

func saveBenchHistory(path string, history []BenchRun) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func getGitCommit() (string, error) {
	cmd := benchExecCommand("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func printResults(cmd *cobra.Command, results []BenchResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "BENCHMARK\tITER\tNS/OP\tB/OP\tALLOCS/OP")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%d\t%.2f\t%d\t%d\n",
			r.Name, r.Iterations, r.NsPerOp, r.BytesPerOp, r.AllocsPerOp)
	}
	w.Flush()
}

func printComparison(cmd *cobra.Command, current, previous []BenchResult, threshold float64) {
	prevMap := make(map[string]BenchResult)
	for _, r := range previous {
		prevMap[r.Name] = r
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "BENCHMARK\tNS/OP\tDIFF %\tSTATUS")

	for _, curr := range current {
		prev, ok := prevMap[curr.Name]
		if !ok {
			fmt.Fprintf(w, "%s\t%.2f\t-\tNEW\n", curr.Name, curr.NsPerOp)
			continue
		}

		diff := (curr.NsPerOp - prev.NsPerOp) / prev.NsPerOp * 100
		status := "PASS"

		// If faster (negative diff), good.
		// If slower (positive diff), check threshold.
		if diff > threshold {
			status = "FAIL 🔴"
		} else if diff < -threshold {
			status = "IMPR 🟢"
		}

		fmt.Fprintf(w, "%s\t%.2f\t%+.2f%%\t%s\n", curr.Name, curr.NsPerOp, diff, status)
	}
	w.Flush()
}
