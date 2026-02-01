package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	loadUrl         string
	loadConcurrency int
	loadRequests    int
	loadDuration    time.Duration
	loadMethod      string
	loadBody        string
	loadHeaders     []string
)

type loadResult struct {
	statusCode int
	latency    time.Duration
	err        error
}

var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Perform HTTP load testing on a target URL",
	Long: `Executes a concurrent HTTP load test against a specified URL.
Reports RPS (Requests Per Second), success rate, and latency percentiles.`,
	RunE: runLoad,
}

func init() {
	rootCmd.AddCommand(loadCmd)
	loadCmd.Flags().StringVar(&loadUrl, "url", "", "Target URL (required)")
	loadCmd.Flags().IntVarP(&loadConcurrency, "concurrency", "c", 10, "Number of concurrent workers")
	loadCmd.Flags().IntVarP(&loadRequests, "requests", "n", 100, "Total number of requests to run")
	loadCmd.Flags().DurationVarP(&loadDuration, "duration", "d", 0, "Duration of the test (overrides --requests if set)")
	loadCmd.Flags().StringVar(&loadMethod, "method", "GET", "HTTP method (GET, POST, etc.)")
	loadCmd.Flags().StringVar(&loadBody, "body", "", "Request body")
	loadCmd.Flags().StringSliceVarP(&loadHeaders, "header", "H", nil, "Custom headers (e.g. 'Content-Type: application/json')")
}

func runLoad(cmd *cobra.Command, args []string) error {
	if loadUrl == "" {
		return fmt.Errorf("--url is required")
	}

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nStopping...")
		cancel()
	}()

	fmt.Printf("Running load test against %s\n", loadUrl)
	if loadDuration > 0 {
		fmt.Printf("Duration: %s, Concurrency: %d\n", loadDuration, loadConcurrency)
		time.AfterFunc(loadDuration, func() {
			cancel()
		})
	} else {
		fmt.Printf("Requests: %d, Concurrency: %d\n", loadRequests, loadConcurrency)
	}

	results := make(chan loadResult, 10000)
	var wg sync.WaitGroup

	startTime := time.Now()

	jobs := make(chan struct{}, loadRequests)

	if loadDuration == 0 {
		for i := 0; i < loadRequests; i++ {
			jobs <- struct{}{}
		}
		close(jobs)
	}

	for i := 0; i < loadConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			for {
				select {
				case <-ctx.Done():
					return
				default:
					if loadDuration == 0 {
						_, ok := <-jobs
						if !ok {
							return
						}
					}

					doRequest(ctx, client, results)
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var totalRequests int64
	var successCount int64
	var errorCount int64
	var latencies []time.Duration
	statusCodes := make(map[int]int)

	for res := range results {
		totalRequests++
		if res.err != nil {
			errorCount++
		} else {
			statusCodes[res.statusCode]++
			if res.statusCode >= 200 && res.statusCode < 300 {
				successCount++
			}
			latencies = append(latencies, res.latency)
		}
	}

	elapsed := time.Since(startTime)
	if elapsed == 0 {
		elapsed = 1 * time.Nanosecond
	}

	if totalRequests == 0 {
		fmt.Println("No requests completed.")
		return nil
	}

	rps := float64(totalRequests) / elapsed.Seconds()

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	fmt.Printf("\n--- Results ---\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Total Requests:\t%d\n", totalRequests)
	fmt.Fprintf(w, "Duration:\t%v\n", elapsed)
	fmt.Fprintf(w, "RPS:\t%.2f\n", rps)
	fmt.Fprintf(w, "Success Rate:\t%.2f%%\n", float64(successCount)/float64(totalRequests)*100)
	fmt.Fprintf(w, "Error Rate:\t%.2f%%\n", float64(errorCount)/float64(totalRequests)*100)

	if len(latencies) > 0 {
		fmt.Fprintf(w, "Latency Min:\t%v\n", latencies[0])
		fmt.Fprintf(w, "Latency P50:\t%v\n", latencies[len(latencies)*50/100])
		fmt.Fprintf(w, "Latency P90:\t%v\n", latencies[len(latencies)*90/100])
		fmt.Fprintf(w, "Latency P99:\t%v\n", latencies[len(latencies)*99/100])
		fmt.Fprintf(w, "Latency Max:\t%v\n", latencies[len(latencies)-1])
	}

	if len(statusCodes) > 0 {
		fmt.Fprintf(w, "\nStatus Codes:\n")
		var codes []int
		for code := range statusCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		for _, code := range codes {
			fmt.Fprintf(w, "  %d:\t%d\n", code, statusCodes[code])
		}
	}
	w.Flush()

	return nil
}

func doRequest(ctx context.Context, client *http.Client, results chan<- loadResult) {
	req, err := http.NewRequestWithContext(ctx, loadMethod, loadUrl, strings.NewReader(loadBody))
	if err != nil {
		results <- loadResult{err: err}
		return
	}

	for _, h := range loadHeaders {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		results <- loadResult{err: err}
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	results <- loadResult{
		statusCode: resp.StatusCode,
		latency:    latency,
	}
}
