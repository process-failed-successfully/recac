package main

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"recac/internal/benchmark"

	"github.com/spf13/cobra"
)

var (
	// Factories for dependency injection in tests
	newRunnerFunc = func() benchmark.Runner { return benchmark.NewGoRunner() }
	newStoreFunc  = func(path string) (benchmark.Store, error) { return benchmark.NewFileStore(path) }
)

type benchOptions struct {
	Compare       bool
	FailThreshold float64
	Package       string
}

func newBenchCmd() *cobra.Command {
	opts := benchOptions{}

	cmd := &cobra.Command{
		Use:   "bench [package]",
		Short: "Run benchmarks and track performance regressions",
		Long:  `Runs Go benchmarks for the specified package (defaulting to current directory), stores the results, and optionally compares them against previous runs to detect regressions.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Package = args[0]
			} else {
				opts.Package = "."
			}

			return runBench(cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Compare, "compare", false, "Compare with previous run")
	cmd.Flags().Float64Var(&opts.FailThreshold, "fail-threshold", 0, "Fail if regression exceeds percentage (e.g. 10.0)")

	return cmd
}

func init() {
	rootCmd.AddCommand(newBenchCmd())
}

func runBench(cmd *cobra.Command, opts benchOptions) error {
	ctx := context.Background()
	runner := newRunnerFunc()

	fmt.Fprintf(cmd.OutOrStdout(), "Running benchmarks for %s...\n", opts.Package)
	results, err := runner.Run(ctx, opts.Package)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No benchmarks found.")
		return nil
	}

	// Create run record
	run := benchmark.Run{
		Timestamp: time.Now(),
		Results:   results,
	}
	// Try to get commit hash if inside git
	// We could import internal/git if available, but for now simple check or skip
	// Keeping it simple.

	// Store logic
	storePath := ".recac/benchmarks.json"
	store, err := newStoreFunc(storePath)
	if err != nil {
		// Just warn if we can't store? Or fail?
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not initialize storage: %v\n", err)
	}

	var prev *benchmark.Run
	if store != nil {
		prev, _ = store.LoadLatest()
		if err := store.Save(run); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save results: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Results saved to %s\n", storePath)
		}
	}

	// Display results
	printBenchResults(cmd, results)

	if opts.Compare && prev != nil {
		fmt.Fprintln(cmd.OutOrStdout(), "\nComparison with previous run:")
		comparisons := benchmark.Compare(*prev, run)
		printComparisonTable(cmd, comparisons)

		if opts.FailThreshold > 0 {
			for _, c := range comparisons {
				if c.NsPerOpDiff > opts.FailThreshold {
					return fmt.Errorf("performance regression detected: %s is %.2f%% slower (threshold: %.2f%%)", c.Name, c.NsPerOpDiff, opts.FailThreshold)
				}
			}
		}
	} else if opts.Compare {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNo previous run to compare with.")
	}

	return nil
}

func printBenchResults(cmd *cobra.Command, results []benchmark.Result) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tITERATIONS\tNS/OP\tB/OP\tALLOCS/OP")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%d\t%.2f\t%d\t%d\n", r.Name, r.Iterations, r.NsPerOp, r.BytesPerOp, r.AllocsPerOp)
	}
	w.Flush()
}

func printComparisonTable(cmd *cobra.Command, comparisons []benchmark.Comparison) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tNS/OP DIFF\tALLOCS/OP DIFF")
	for _, c := range comparisons {
		nsDiff := fmt.Sprintf("%.2f%%", c.NsPerOpDiff)
		if c.NsPerOpDiff > 0 {
			nsDiff = "+" + nsDiff // Slower
		}

		fmt.Fprintf(w, "%s\t%s\t%.2f%%\n", c.Name, nsDiff, c.AllocsPerOpDiff)
	}
	w.Flush()
}
