package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/load"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	loadConcurrency int
	loadDuration    time.Duration
	loadRequests    int
	loadMethod      string
	loadHeaders     []string
	loadBody        string
)

var loadCmd = &cobra.Command{
	Use:   "load [url]",
	Short: "Perform HTTP load testing",
	Long: `Perform concurrent HTTP load testing on a specified URL.

Example:
  recac load http://localhost:8080 -c 10 -d 30s
  recac load https://api.example.com/v1/users -c 5 -n 100 --header "Authorization: Bearer token"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		fmt.Printf("🚀 Starting load test on %s\n", url)
		fmt.Printf("   Concurrency: %d\n", loadConcurrency)
		if loadRequests > 0 {
			fmt.Printf("   Requests:    %d\n", loadRequests)
		} else if loadDuration > 0 {
			fmt.Printf("   Duration:    %s\n", loadDuration)
		}

		cfg := load.Config{
			URL:         url,
			Method:      loadMethod,
			Headers:     loadHeaders,
			Body:        []byte(loadBody),
			Concurrency: loadConcurrency,
			Duration:    loadDuration,
			Requests:    loadRequests,
		}

		tester := load.NewTester(cfg)
		stats := tester.Run(context.Background())

		printStats(stats)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)

	loadCmd.Flags().IntVarP(&loadConcurrency, "concurrency", "c", 10, "Number of concurrent workers")
	loadCmd.Flags().DurationVarP(&loadDuration, "duration", "d", 0, "Duration of the test (e.g. 10s, 1m)")
	loadCmd.Flags().IntVarP(&loadRequests, "requests", "n", 0, "Number of total requests to run (overrides duration if set)")
	loadCmd.Flags().StringVarP(&loadMethod, "method", "X", "GET", "HTTP method to use")
	loadCmd.Flags().StringArrayVarP(&loadHeaders, "header", "H", []string{}, "HTTP headers to send (can be repeated)")
	loadCmd.Flags().StringVarP(&loadBody, "body", "b", "", "Request body")
}

func printStats(stats load.Stats) {
	fmt.Println("\n🏁 Load Test Completed")
	fmt.Println("------------------------------------------------")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Total Requests:\t%d\n", stats.TotalRequests)
	fmt.Fprintf(w, "Success:\t%d\n", stats.Success)
	fmt.Fprintf(w, "Failures:\t%d\n", stats.Failures)
	fmt.Fprintf(w, "Duration:\t%v\n", stats.Duration)
	fmt.Fprintf(w, "RPS:\t%.2f\n", stats.RPS)
	fmt.Fprintln(w, "\nLatency Distribution:")
	fmt.Fprintf(w, "  Min:\t%v\n", stats.MinLatency)
	fmt.Fprintf(w, "  P50:\t%v\n", stats.P50Latency)
	fmt.Fprintf(w, "  P90:\t%v\n", stats.P90Latency)
	fmt.Fprintf(w, "  P99:\t%v\n", stats.P99Latency)
	fmt.Fprintf(w, "  Max:\t%v\n", stats.MaxLatency)
	fmt.Fprintf(w, "  Avg:\t%v\n", stats.AvgLatency)

	w.Flush()
}
