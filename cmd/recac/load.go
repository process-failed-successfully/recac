package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	loadURL         string
	loadConcurrency int
	loadRequests    int
	loadDuration    time.Duration
)

var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Run a simple HTTP load test",
	Long:  `Executes a concurrent HTTP load test against a specified URL.
Calculates latency percentiles, throughput, and success rates.`,
	RunE: runLoadTest,
}

func init() {
	rootCmd.AddCommand(loadCmd)
	loadCmd.Flags().StringVarP(&loadURL, "url", "u", "", "Target URL (required)")
	loadCmd.Flags().IntVarP(&loadConcurrency, "concurrency", "c", 10, "Number of concurrent workers")
	loadCmd.Flags().IntVarP(&loadRequests, "requests", "n", 100, "Total number of requests to send")
	loadCmd.Flags().DurationVarP(&loadDuration, "duration", "d", 0, "Duration limit (overrides request count if reached)")

	loadCmd.MarkFlagRequired("url")
}

type requestResult struct {
	statusCode int
	duration   time.Duration
	err        error
}

func runLoadTest(cmd *cobra.Command, args []string) error {
	// If duration is set and requests is default, assume unlimited requests (time-based test)
	if loadDuration > 0 && !cmd.Flags().Changed("requests") {
		loadRequests = int(^uint(0) >> 1) // Max int
		fmt.Fprintf(cmd.OutOrStdout(), "Time-based test: running for %v (unlimited requests)\n", loadDuration)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting load test against %s\n", loadURL)
	fmt.Fprintf(cmd.OutOrStdout(), "Workers: %d, Max Requests: %d\n\n", loadConcurrency, loadRequests)

	// Better approach: collect results in a separate goroutine
	doneResults := make(chan struct{})
	var (
		totalReqs    int
		success      int
		failures     int
		statusCodes  = make(map[int]int)
		durations    []time.Duration
		totalLatency time.Duration
	)

	// Create an unbuffered or small buffered channel
	resultChan := make(chan requestResult, loadConcurrency*2)

	go func() {
		for res := range resultChan {
			totalReqs++
			if res.err != nil {
				failures++
			} else {
				statusCodes[res.statusCode]++
				if res.statusCode >= 200 && res.statusCode < 400 {
					success++
				} else {
					failures++
				}
			}
			durations = append(durations, res.duration)
			totalLatency += res.duration
		}
		close(doneResults)
	}()

	var wg sync.WaitGroup

	// Rate limiting / Job distribution
	jobs := make(chan struct{}, loadConcurrency) // Small buffer

	startTime := time.Now()

	// Start workers
	for i := 0; i < loadConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 10 * time.Second,
			}
			for range jobs {
				start := time.Now()
				resp, err := client.Get(loadURL)
				duration := time.Since(start)

				res := requestResult{
					duration: duration,
					err:      err,
				}
				if err == nil {
					res.statusCode = resp.StatusCode
					resp.Body.Close()
				}
				resultChan <- res
			}
		}()
	}

	// Dispatch jobs
	dispatch := func() {
		if loadDuration > 0 {
			timeout := time.After(loadDuration)
			count := 0
			for {
				select {
				case <-timeout:
					close(jobs)
					return
				default:
					if count >= loadRequests {
						close(jobs)
						return
					}
					// Check if workers are stuck or channel full?
					// This write blocks if workers are slow. That's fine.
					// But we must also check timeout while blocking?
					select {
					case jobs <- struct{}{}:
						count++
					case <-timeout:
						close(jobs)
						return
					}
				}
			}
		} else {
			for i := 0; i < loadRequests; i++ {
				jobs <- struct{}{}
			}
			close(jobs)
		}
	}

	dispatch()

	wg.Wait()
	close(resultChan)
	<-doneResults // Wait for collection

	totalDuration := time.Since(startTime)

	if totalReqs == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No requests sent.")
		return nil
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	// Metrics
	rps := float64(totalReqs) / totalDuration.Seconds()
	avgLatency := totalLatency / time.Duration(totalReqs)

	// Helper for percentile
	getPercentile := func(p int) time.Duration {
		idx := len(durations) * p / 100
		if idx >= len(durations) {
			idx = len(durations) - 1
		}
		return durations[idx]
	}

	p50 := getPercentile(50)
	p90 := getPercentile(90)
	p95 := getPercentile(95)
	p99 := getPercentile(99)

	// Report
	fmt.Fprintln(cmd.OutOrStdout(), "--- Results ---")
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Total Requests:\t%d\n", totalReqs)
	fmt.Fprintf(w, "Total Duration:\t%v\n", totalDuration)
	fmt.Fprintf(w, "Requests/sec:\t%.2f\n", rps)
	successRate := float64(0)
	if totalReqs > 0 {
		successRate = float64(success) / float64(totalReqs) * 100
	}
	fmt.Fprintf(w, "Success Rate:\t%.2f%%\n", successRate)
	fmt.Fprintf(w, "Avg Latency:\t%v\n", avgLatency)
	fmt.Fprintf(w, "P50 Latency:\t%v\n", p50)
	fmt.Fprintf(w, "P90 Latency:\t%v\n", p90)
	fmt.Fprintf(w, "P95 Latency:\t%v\n", p95)
	fmt.Fprintf(w, "P99 Latency:\t%v\n", p99)
	w.Flush()

	fmt.Fprintln(cmd.OutOrStdout(), "\n--- Status Codes ---")
	for code, count := range statusCodes {
		fmt.Fprintf(cmd.OutOrStdout(), "[%d]: %d\n", code, count)
	}

	if failures > 0 {
		fmt.Printf("\nErrors/Failures: %d\n", failures)
	}

	return nil
}
