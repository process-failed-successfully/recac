package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	loadRate      int
	loadDuration  time.Duration
	loadMethod    string
	loadBody      string
	loadHeaders   []string
	loadThreshold string
	loadTimeout   time.Duration
)

var loadCmd = &cobra.Command{
	Use:   "load [url]",
	Short: "Perform HTTP load testing",
	Long: `Perform a simple HTTP load test against a target URL.
Generates requests at a specified rate for a given duration and reports latency metrics.

Example:
  recac load http://localhost:8080/api/users --rate 50 --duration 10s
  recac load http://example.com -m POST -b '{"key":"value"}' -H "Content-Type: application/json" --threshold "p95<500ms"`,
	Args: cobra.ExactArgs(1),
	RunE: runLoad,
}

func init() {
	rootCmd.AddCommand(loadCmd)
	loadCmd.Flags().IntVarP(&loadRate, "rate", "r", 10, "Requests per second")
	loadCmd.Flags().DurationVarP(&loadDuration, "duration", "d", 10*time.Second, "Duration of the test")
	loadCmd.Flags().StringVarP(&loadMethod, "method", "m", "GET", "HTTP method")
	loadCmd.Flags().StringVarP(&loadBody, "body", "b", "", "Request body (string)")
	loadCmd.Flags().StringSliceVarP(&loadHeaders, "header", "H", nil, "Request headers (key:value)")
	loadCmd.Flags().StringVar(&loadThreshold, "threshold", "", "Pass/Fail threshold (e.g., 'p95<500ms', 'error<1%')")
	loadCmd.Flags().DurationVar(&loadTimeout, "timeout", 5*time.Second, "Request timeout")
}

type LoadResult struct {
	Timestamp time.Time
	Latency   time.Duration
	Status    int
	Error     error
}

type LoadStats struct {
	TotalRequests int
	Success       int
	Errors        int
	StatusCodes   map[int]int
	Latencies     []time.Duration
	Duration      time.Duration
	RPS           float64
}

func runLoad(cmd *cobra.Command, args []string) error {
	targetURL := args[0]

	fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting load test on %s\n", targetURL)
	fmt.Fprintf(cmd.OutOrStdout(), "   Rate: %d req/s | Duration: %v | Method: %s\n\n", loadRate, loadDuration, loadMethod)

	// Optimize HTTP Client for Load Testing
	transport := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   loadTimeout,
	}

	results := make(chan LoadResult, 1000)
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), loadDuration)
	defer cancel()

	// Parse Headers
	parsedHeaders := make(http.Header)
	for _, h := range loadHeaders {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			parsedHeaders.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	startTime := time.Now()

	// Start Stats Processor
	statsCh := make(chan LoadStats)
	go func() {
		statsCh <- processResultsStreaming(results)
	}()

	ticker := time.NewTicker(time.Second / time.Duration(loadRate))
	defer ticker.Stop()

	// Dispatcher
	dispatcherDone := make(chan struct{})
	go func() {
		defer close(dispatcherDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				wg.Add(1)
				go func() {
					defer wg.Done()
					res := doRequest(client, targetURL, loadMethod, loadBody, parsedHeaders)
					select {
					case results <- res:
					default:
						// If channel is full, we drop the result to avoid blocking workers
						// Ideally we'd log this or count dropped requests
					}
				}()
			}
		}
	}()

	// Wait for duration
	<-ctx.Done()
	<-dispatcherDone

	// Wait for workers to finish and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Wait for stats
	stats := <-statsCh
	stats.Duration = time.Since(startTime)
	if stats.Duration.Seconds() > 0 {
		stats.RPS = float64(stats.TotalRequests) / stats.Duration.Seconds()
	}

	// Display
	printLoadStats(cmd.OutOrStdout(), stats)

	// Check Thresholds
	if loadThreshold != "" {
		if err := checkThreshold(stats, loadThreshold); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✅ Threshold check passed")
	}

	return nil
}

func doRequest(client *http.Client, url, method, body string, headers http.Header) LoadResult {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return LoadResult{Timestamp: time.Now(), Error: err}
	}
	req.Header = headers

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return LoadResult{Timestamp: start, Latency: latency, Error: err}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // Read body to ensure reuse

	return LoadResult{
		Timestamp: start,
		Latency:   latency,
		Status:    resp.StatusCode,
	}
}

func processResultsStreaming(results chan LoadResult) LoadStats {
	stats := LoadStats{
		StatusCodes: make(map[int]int),
	}

	for res := range results {
		stats.TotalRequests++
		if res.Error != nil {
			stats.Errors++
		} else {
			stats.Success++
			stats.StatusCodes[res.Status]++
			stats.Latencies = append(stats.Latencies, res.Latency)
		}
	}

	sort.Slice(stats.Latencies, func(i, j int) bool {
		return stats.Latencies[i] < stats.Latencies[j]
	})

	return stats
}

func printLoadStats(w io.Writer, stats LoadStats) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "METRIC\tVALUE")
	fmt.Fprintln(tw, "------\t-----")
	fmt.Fprintf(tw, "Duration\t%v\n", stats.Duration)
	fmt.Fprintf(tw, "Total Requests\t%d\n", stats.TotalRequests)
	rate := 0.0
	if stats.TotalRequests > 0 {
		rate = float64(stats.Success) / float64(stats.TotalRequests) * 100
	}
	fmt.Fprintf(tw, "Success Rate\t%.2f%%\n", rate)
	fmt.Fprintf(tw, "RPS\t%.2f\n", stats.RPS)
	fmt.Fprintf(tw, "Errors\t%d\n", stats.Errors)

	if len(stats.Latencies) > 0 {
		fmt.Fprintf(tw, "Latency (Mean)\t%v\n", mean(stats.Latencies))
		fmt.Fprintf(tw, "Latency (P50)\t%v\n", percentile(stats.Latencies, 0.50))
		fmt.Fprintf(tw, "Latency (P90)\t%v\n", percentile(stats.Latencies, 0.90))
		fmt.Fprintf(tw, "Latency (P95)\t%v\n", percentile(stats.Latencies, 0.95))
		fmt.Fprintf(tw, "Latency (P99)\t%v\n", percentile(stats.Latencies, 0.99))
		fmt.Fprintf(tw, "Latency (Max)\t%v\n", stats.Latencies[len(stats.Latencies)-1])
	}

	fmt.Fprintln(tw, "")
	fmt.Fprintln(tw, "STATUS CODES")
	for code, count := range stats.StatusCodes {
		fmt.Fprintf(tw, "%d\t%d\n", code, count)
	}

	tw.Flush()
}

func mean(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	var sum time.Duration
	for _, v := range d {
		sum += v
	}
	return sum / time.Duration(len(d))
}

func percentile(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	index := int(float64(len(d)) * p)
	if index >= len(d) {
		index = len(d) - 1
	}
	return d[index]
}

func checkThreshold(stats LoadStats, threshold string) error {
	parts := strings.Split(threshold, "<")
	operator := "<"
	if len(parts) != 2 {
		parts = strings.Split(threshold, ">")
		operator = ">"
		if len(parts) != 2 {
			return fmt.Errorf("invalid threshold format: %s", threshold)
		}
	}

	metric := strings.TrimSpace(parts[0])
	valueStr := strings.TrimSpace(parts[1])

	var actual float64
	var target float64
	var err error

	// Determine actual value
	switch metric {
	case "p50":
		actual = float64(percentile(stats.Latencies, 0.50).Milliseconds())
	case "p90":
		actual = float64(percentile(stats.Latencies, 0.90).Milliseconds())
	case "p95":
		actual = float64(percentile(stats.Latencies, 0.95).Milliseconds())
	case "p99":
		actual = float64(percentile(stats.Latencies, 0.99).Milliseconds())
	case "mean":
		actual = float64(mean(stats.Latencies).Milliseconds())
	case "error":
		if stats.TotalRequests > 0 {
			actual = float64(stats.Errors) / float64(stats.TotalRequests) * 100
		}
	case "rps":
		actual = stats.RPS
	default:
		return fmt.Errorf("unknown metric: %s", metric)
	}

	// Parse target
	if metric == "error" || metric == "rps" {
		// Expect number
		valueStr = strings.TrimSuffix(valueStr, "%")
		target, err = strconv.ParseFloat(valueStr, 64)
	} else {
		// Expect duration, convert to ms
		dur, parseErr := time.ParseDuration(valueStr)
		if parseErr != nil {
			return parseErr
		}
		target = float64(dur.Milliseconds())
	}

	if err != nil {
		return fmt.Errorf("invalid value: %s", valueStr)
	}

	// Compare
	failed := false
	if operator == "<" {
		if actual >= target {
			failed = true
		}
	} else {
		if actual <= target {
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("threshold failed: %s (actual: %.2f, target: %s %.2f)", threshold, actual, operator, target)
	}

	return nil
}
