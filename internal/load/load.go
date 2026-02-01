package load

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds the configuration for the load test.
type Config struct {
	URL         string
	Method      string
	Headers     []string
	Body        []byte
	Concurrency int
	Duration    time.Duration
	Requests    int // Total requests to run (optional limit)
}

// Result represents the outcome of a single request.
type Result struct {
	Duration time.Duration
	Status   int
	Error    error
}

// Stats holds the aggregate statistics of the load test.
type Stats struct {
	TotalRequests int
	Success       int
	Failures      int
	RPS           float64
	AvgLatency    time.Duration
	MinLatency    time.Duration
	MaxLatency    time.Duration
	P50Latency    time.Duration
	P90Latency    time.Duration
	P99Latency    time.Duration
	Duration      time.Duration
}

// Tester executes the load test.
type Tester struct {
	client *http.Client
	config Config
}

// NewTester creates a new Tester with the given configuration.
func NewTester(cfg Config) *Tester {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.Method == "" {
		cfg.Method = "GET"
	}

	return &Tester{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.Concurrency,
				MaxIdleConnsPerHost: cfg.Concurrency,
				DisableKeepAlives:   false,
			},
		},
		config: cfg,
	}
}

// Run executes the load test and returns the statistics.
func (t *Tester) Run(ctx context.Context) Stats {
	// Unbuffered or small buffer is fine if we consume continuously
	results := make(chan Result, t.config.Concurrency*2)
	var wg sync.WaitGroup

	startTime := time.Now()

	// Create context with timeout if duration is set
	var cancel context.CancelFunc
	if t.config.Duration > 0 {
		ctx, cancel = context.WithTimeout(ctx, t.config.Duration)
		defer cancel()
	} else {
		// Ensure we have a cancelable context anyway
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
	}

	// Request counter for limiting total requests
	var requestCount int64

	// Worker function
	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Check request limit if set
				if t.config.Requests > 0 {
					current := atomic.AddInt64(&requestCount, 1)
					if current > int64(t.config.Requests) {
						// If we hit the limit, we should signal others or just stop?
						// Just stop this worker.
						return
					}
				}

				start := time.Now()
				req, err := http.NewRequestWithContext(ctx, t.config.Method, t.config.URL, bytes.NewReader(t.config.Body))
				if err != nil {
					// Context errors might happen if cancelled during creation
					results <- Result{Error: err, Duration: time.Since(start)}
					continue
				}

				for _, h := range t.config.Headers {
					parts := splitHeader(h)
					if len(parts) == 2 {
						req.Header.Add(parts[0], parts[1])
					}
				}

				resp, err := t.client.Do(req)
				duration := time.Since(start)

				if err != nil {
					// Check if error is due to context cancellation (timeout)
					if ctx.Err() != nil {
						// Don't report this as a failure if it's just the test ending
						return
					}
					results <- Result{Error: err, Duration: duration}
				} else {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					results <- Result{Status: resp.StatusCode, Duration: duration}
				}
			}
		}
	}

	// Start workers
	wg.Add(t.config.Concurrency)
	for i := 0; i < t.config.Concurrency; i++ {
		go worker()
	}

	// Wait for workers in a separate goroutine and close channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []Result
	for res := range results {
		allResults = append(allResults, res)
	}

	totalDuration := time.Since(startTime)
	// If the test ran for a fixed duration, the actual time might be slightly longer due to last requests finishing.
	// For RPS, using the configured duration might be more "correct" if it was saturated,
	// but using actual elapsed time is safer.

	return calculateStats(allResults, totalDuration)
}

func calculateStats(results []Result, duration time.Duration) Stats {
	stats := Stats{
		TotalRequests: len(results),
		Duration:      duration,
	}

	if len(results) == 0 {
		return stats
	}

	var totalLatency time.Duration
	var latencies []time.Duration
	latencies = make([]time.Duration, 0, len(results))

	// Init Min/Max with first element
	stats.MinLatency = results[0].Duration
	stats.MaxLatency = results[0].Duration

	for _, r := range results {
		if r.Error != nil || r.Status >= 400 {
			stats.Failures++
		} else {
			stats.Success++
		}

		latencies = append(latencies, r.Duration)
		totalLatency += r.Duration

		if r.Duration < stats.MinLatency {
			stats.MinLatency = r.Duration
		}
		if r.Duration > stats.MaxLatency {
			stats.MaxLatency = r.Duration
		}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	stats.AvgLatency = totalLatency / time.Duration(len(results))
	stats.RPS = float64(len(results)) / duration.Seconds()

	if len(latencies) > 0 {
		stats.P50Latency = latencies[len(latencies)*50/100]
		stats.P90Latency = latencies[len(latencies)*90/100]
		stats.P99Latency = latencies[len(latencies)*99/100]
	}

	return stats
}

func splitHeader(h string) []string {
	parts := strings.SplitN(h, ":", 2)
	if len(parts) == 2 {
		parts[0] = strings.TrimSpace(parts[0])
		parts[1] = strings.TrimSpace(parts[1])
	}
	return parts
}
