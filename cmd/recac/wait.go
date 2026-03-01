package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var (
	waitTimeout  time.Duration
	waitInterval time.Duration
)

var waitCmd = &cobra.Command{
	Use:   "wait <url>",
	Short: "Wait for a URL to return an HTTP 200 OK status",
	Long: `Polls a specified URL until it responds with an HTTP 200 OK status code.
Useful in CI/CD pipelines or local development to wait for a service to become available.

Example:
  recac wait http://localhost:8080/health
  recac wait --timeout 1m --interval 5s https://api.example.com/status`,
	Args: cobra.ExactArgs(1),
	RunE: runWait,
}

func init() {
	waitCmd.Flags().DurationVarP(&waitTimeout, "timeout", "t", 30*time.Second, "Maximum time to wait")
	waitCmd.Flags().DurationVarP(&waitInterval, "interval", "i", 2*time.Second, "Interval between polling requests")
	rootCmd.AddCommand(waitCmd)
}

func runWait(cmd *cobra.Command, args []string) error {
	targetURL := args[0]
	timeout := waitTimeout
	interval := waitInterval

	fmt.Fprintf(cmd.OutOrStdout(), "Waiting up to %v for %s to be ready...\n", timeout, targetURL)

	client := &http.Client{
		Timeout: 5 * time.Second, // Timeout for each individual request
	}

	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout reached after %v waiting for %s", timeout, targetURL)
		}

		resp, err := client.Get(targetURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Fprintf(cmd.OutOrStdout(), "✅ %s is ready! (took %v)\n", targetURL, time.Since(start).Round(time.Millisecond))
				return nil
			}
		}

		time.Sleep(interval)
	}
}
