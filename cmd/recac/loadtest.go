package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ltConcurrency int
	ltRequests    int
	ltDuration    time.Duration
	ltMethod      string
	ltBody        string
	ltAnalyze     bool
)

var loadtestCmd = &cobra.Command{
	Use:   "loadtest [url]",
	Short: "Perform a simple load test against a URL",
	Long: `Sends concurrent HTTP requests to a specified URL and measures performance.
Can also use AI to analyze the results and suggest improvements.

Examples:
  recac loadtest http://localhost:8080/api/v1/users --concurrency 10 --requests 1000
  recac loadtest http://example.com --duration 30s --analyze`,
	Args: cobra.ExactArgs(1),
	RunE: runLoadTest,
}

func init() {
	rootCmd.AddCommand(loadtestCmd)
	loadtestCmd.Flags().IntVarP(&ltConcurrency, "concurrency", "c", 10, "Number of concurrent workers")
	loadtestCmd.Flags().IntVarP(&ltRequests, "requests", "n", 0, "Total number of requests to send (0 for duration-based)")
	loadtestCmd.Flags().DurationVarP(&ltDuration, "duration", "d", 10*time.Second, "Duration of the test (if requests is 0)")
	loadtestCmd.Flags().StringVarP(&ltMethod, "method", "m", "GET", "HTTP method to use")
	loadtestCmd.Flags().StringVarP(&ltBody, "body", "b", "", "Request body string")
	loadtestCmd.Flags().BoolVar(&ltAnalyze, "analyze", false, "Use AI to analyze results")
}

type LoadTestResult struct {
	TotalRequests int
	SuccessCount  int
	ErrorCount    int
	StatusCodes   map[int]int
	Latencies     []time.Duration
	TotalDuration time.Duration
	RPS           float64
}

func runLoadTest(cmd *cobra.Command, args []string) error {
	url := args[0]
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting load test against %s\n", url)
	fmt.Fprintf(cmd.OutOrStdout(), "Configuration: Concurrency=%d, Method=%s", ltConcurrency, ltMethod)
	if ltRequests > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), ", Requests=%d\n", ltRequests)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), ", Duration=%s\n", ltDuration)
	}

	result, err := executeLoadTest(url)
	if err != nil {
		return err
	}

	printLoadTestReport(cmd.OutOrStdout(), result)

	if ltAnalyze {
		return analyzeLoadTest(cmd, url, result)
	}

	return nil
}

func executeLoadTest(url string) (*LoadTestResult, error) {
	var wg sync.WaitGroup
	// Buffered channels to avoid blocking
	bufferSize := 10000
	if ltRequests > 0 {
		bufferSize = ltRequests
	}
	resultsChan := make(chan time.Duration, bufferSize)
	errorsChan := make(chan int, bufferSize) // Stores status codes (0 for net error)

	ctx, cancel := context.WithCancel(context.Background())
	if ltRequests == 0 {
		// Duration based
		time.AfterFunc(ltDuration, cancel)
	}
	defer cancel()

	startTime := time.Now()

	// Determine requests per worker if fixed count
	requestsPerWorker := 0
	if ltRequests > 0 {
		requestsPerWorker = ltRequests / ltConcurrency
	}
	remainder := 0
	if ltRequests > 0 {
		remainder = ltRequests % ltConcurrency
	}

	for i := 0; i < ltConcurrency; i++ {
		wg.Add(1)

		workerRequests := requestsPerWorker
		if i == 0 {
			workerRequests += remainder
		}

		go func(id, limit int) {
			defer wg.Done()
			client := &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					MaxIdleConnsPerHost: ltConcurrency,
				},
			}

			count := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if ltRequests > 0 && count >= limit {
						return
					}

					start := time.Now()
					var req *http.Request
					var err error

					if ltBody != "" {
						req, err = http.NewRequest(ltMethod, url, strings.NewReader(ltBody))
					} else {
						req, err = http.NewRequest(ltMethod, url, nil)
					}

					if err != nil {
						// Should not happen for valid URL
						errorsChan <- 0
						count++
						continue
					}

					resp, err := client.Do(req)
					latency := time.Since(start)

					if err != nil {
						errorsChan <- 0
					} else {
						errorsChan <- resp.StatusCode
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						resultsChan <- latency
					}
					count++
				}
			}
		}(i, workerRequests)
	}

	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	totalDuration := time.Since(startTime)

	// Aggregate results
	res := &LoadTestResult{
		StatusCodes:   make(map[int]int),
		TotalDuration: totalDuration,
	}

	for l := range resultsChan {
		res.Latencies = append(res.Latencies, l)
	}

	networkErrors := 0
	for code := range errorsChan {
		if code == 0 {
			networkErrors++
		} else {
			res.StatusCodes[code]++
		}
	}

	// Calculate counts
	res.SuccessCount = 0 // Will count 2xx
	res.ErrorCount = networkErrors

	for code, count := range res.StatusCodes {
		if code >= 200 && code < 300 {
			res.SuccessCount += count
		} else {
			res.ErrorCount += count
		}
	}

	res.TotalRequests = res.SuccessCount + res.ErrorCount // Note: redirects (3xx) count as errors here unless we handle them. Default client follows redirects.

	if res.TotalDuration.Seconds() > 0 {
		res.RPS = float64(res.TotalRequests) / res.TotalDuration.Seconds()
	}

	sort.Slice(res.Latencies, func(i, j int) bool {
		return res.Latencies[i] < res.Latencies[j]
	})

	return res, nil
}

func printLoadTestReport(w io.Writer, res *LoadTestResult) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\n📊 Load Test Results:")
	fmt.Fprintf(tw, "Total Requests:\t%d\n", res.TotalRequests)
	fmt.Fprintf(tw, "Total Duration:\t%s\n", res.TotalDuration)
	fmt.Fprintf(tw, "Requests/Sec:\t%.2f\n", res.RPS)

	errorRate := 0.0
	if res.TotalRequests > 0 {
		errorRate = float64(res.ErrorCount) / float64(res.TotalRequests) * 100
	}
	fmt.Fprintf(tw, "Error Rate:\t%.2f%%\n", errorRate)

	if len(res.Latencies) > 0 {
		min := res.Latencies[0]
		max := res.Latencies[len(res.Latencies)-1]

		sum := int64(0)
		for _, l := range res.Latencies {
			sum += int64(l)
		}
		avg := time.Duration(sum / int64(len(res.Latencies)))

		p50 := res.Latencies[len(res.Latencies)/2]
		p95 := res.Latencies[int(float64(len(res.Latencies))*0.95)]
		p99 := res.Latencies[int(float64(len(res.Latencies))*0.99)]

		fmt.Fprintln(tw, "\nLatencies:\t")
		fmt.Fprintf(tw, "  Min:\t%s\n", min)
		fmt.Fprintf(tw, "  Avg:\t%s\n", avg)
		fmt.Fprintf(tw, "  P50:\t%s\n", p50)
		fmt.Fprintf(tw, "  P95:\t%s\n", p95)
		fmt.Fprintf(tw, "  P99:\t%s\n", p99)
		fmt.Fprintf(tw, "  Max:\t%s\n", max)
	}

	if len(res.StatusCodes) > 0 {
		fmt.Fprintln(tw, "\nStatus Codes:\t")
		// Sort codes for consistent output
		var codes []int
		for code := range res.StatusCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)

		for _, code := range codes {
			fmt.Fprintf(tw, "  %d:\t%d\n", code, res.StatusCodes[code])
		}
	}
	tw.Flush()
}

func analyzeLoadTest(cmd *cobra.Command, url string, res *LoadTestResult) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-loadtest")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Safe latency access
	var min, avg, p95, p99, max time.Duration
	if len(res.Latencies) > 0 {
		min = res.Latencies[0]
		max = res.Latencies[len(res.Latencies)-1]
		p95 = res.Latencies[int(float64(len(res.Latencies))*0.95)]
		p99 = res.Latencies[int(float64(len(res.Latencies))*0.99)]

		sum := int64(0)
		for _, l := range res.Latencies {
			sum += int64(l)
		}
		avg = time.Duration(sum / int64(len(res.Latencies)))
	}

	prompt := fmt.Sprintf(`Analyze the following load test results and suggest improvements.
Target URL: %s
Requests: %d
Concurrency: %d
RPS: %.2f
Errors: %d

Latencies:
Min: %s
Avg: %s
P95: %s
P99: %s
Max: %s

Status Codes:
%v
`, url, res.TotalRequests, ltConcurrency, res.RPS, res.ErrorCount,
		min, avg, p95, p99, max, res.StatusCodes)

	fmt.Fprintln(cmd.OutOrStdout(), "\n🤖 Analyzing performance...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), resp)
	return nil
}
