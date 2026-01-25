package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	proxyPort       int
	proxyTarget     string
	proxyRecordFile string
	proxyGenerate   bool
	proxyOutput     string
	proxyLanguage   string
)

type Interaction struct {
	Timestamp time.Time `json:"timestamp"`
	Request   ReqDump   `json:"request"`
	Response  ResDump   `json:"response"`
}

type ReqDump struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"`
}

type ResDump struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"`
}

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Reverse proxy to record traffic and generate tests",
	Long: `Starts a reverse proxy that sits between your client and a target API.
It records all HTTP interactions to a JSON file.
You can optionally use the --generate flag to immediately generate integration tests
from the recorded session using AI.

Example:
  recac proxy --target http://localhost:8000 --record traffic.json
  recac proxy --record traffic.json --generate --output integration_test.go`,
	RunE: runProxy,
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.Flags().IntVarP(&proxyPort, "port", "p", 8080, "Port to listen on")
	proxyCmd.Flags().StringVarP(&proxyTarget, "target", "t", "", "Target URL to proxy to (required for recording)")
	proxyCmd.Flags().StringVarP(&proxyRecordFile, "record", "r", "recording.json", "File to save recordings to (or read from for generation)")
	proxyCmd.Flags().BoolVarP(&proxyGenerate, "generate", "g", false, "Generate tests from the recording file")
	proxyCmd.Flags().StringVarP(&proxyOutput, "output", "o", "proxy_test_gen.go", "Output file for generated tests")
	proxyCmd.Flags().StringVarP(&proxyLanguage, "lang", "l", "go", "Language for generated tests (go, python, js)")
}

func runProxy(cmd *cobra.Command, args []string) error {
	// Mode 1: Generate only (if target is missing but generate is set)
	if proxyGenerate && proxyTarget == "" {
		return runProxyGeneration(cmd)
	}

	// Mode 2: Proxy and Record (and optional generate after)
	if proxyTarget == "" {
		return fmt.Errorf("--target is required to start proxy")
	}

	targetURL, err := url.Parse(proxyTarget)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	interactions := make([]Interaction, 0)
	var mu sync.Mutex

	proxy := NewProxyHandler(targetURL, func(i Interaction) {
		mu.Lock()
		defer mu.Unlock()
		interactions = append(interactions, i)
		// Print summary
		fmt.Printf("[%s] %s %s -> %d\n", i.Timestamp.Format(time.TimeOnly), i.Request.Method, i.Request.URL, i.Response.Status)
	}, proxyRecordFile)

	addr := fmt.Sprintf(":%d", proxyPort)
	fmt.Fprintf(cmd.OutOrStdout(), "🚀 Proxy listening on %s forwarding to %s\n", addr, proxyTarget)
	fmt.Fprintf(cmd.OutOrStdout(), "🔴 Recording to %s\n", proxyRecordFile)
	fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to stop recording and save.")

	// Start Server
	server := &http.Server{Addr: addr, Handler: proxy}

	// Handle graceful shutdown on interrupt is usually handled by main's signal handling,
	// but here we are blocking. To allow saving on Ctrl-C, we need to handle signals or defer save?
	// The main function doesn't seem to pass signals down easily to blocking RunE.
	// We'll trust defer to save on exit if panic or return, but OS.Exit terminates immediately.
	// Cobra commands usually block until done.
	// Let's rely on user stopping it, but saving needs to happen.
	// Ideally we hook into signals.
	// Since I can't easily change main.go to handle signals and pass to me,
	// I will save on every request (append) OR handle signals here.

	// Let's save continuously or on error?
	// Appending to file is safer.

	// For simplicity in this "MVP" feature:
	// We will just run the server. When user kills it, we might lose in-memory data if we don't handle signal.
	// Let's use a channel for signals.

	// Actually, let's just write to file on every request to be safe.

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// NewProxyHandler creates a reverse proxy handler with recording capabilities
func NewProxyHandler(targetURL *url.URL, onRecord func(Interaction), recordFile string) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom Director to ensure host header is set correctly
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
	}

	var fileMu sync.Mutex

	// Wrap the transport to capture response
	proxy.Transport = &recordingTransport{
		transport: http.DefaultTransport,
		onRecord: func(i Interaction) {
			onRecord(i)
			fileMu.Lock()
			saveInteractionJSONL(i, recordFile)
			fileMu.Unlock()
		},
	}

	return proxy
}

func saveInteractionJSONL(i Interaction, file string) {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	data, _ := json.Marshal(i)
	f.Write(data)
	f.WriteString("\n")
}

// runProxyGeneration generates tests from the recording file
func runProxyGeneration(cmd *cobra.Command) error {
	lines, err := readLines(proxyRecordFile)
	if err != nil {
		return fmt.Errorf("failed to read recording file %s: %w", proxyRecordFile, err)
	}

	var interactions []Interaction
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var i Interaction
		if err := json.Unmarshal([]byte(line), &i); err != nil {
			// Try fallback to array if legacy?
			continue
		}
		interactions = append(interactions, i)
	}

	// If no interactions found via JSONL, maybe it's a JSON array (legacy/old test)
	if len(interactions) == 0 {
		content, _ := os.ReadFile(proxyRecordFile)
		json.Unmarshal(content, &interactions)
	}

	if len(interactions) == 0 {
		return fmt.Errorf("recording is empty")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Generating %s tests from %d interactions...\n", proxyLanguage, len(interactions))

	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-proxy-gen")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// limit interactions to avoid context window blowup
	maxInt := 10
	if len(interactions) > maxInt {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: Truncating to first %d interactions for prompt size.\n", maxInt)
		interactions = interactions[:maxInt]
	}

	interactionsJSON, _ := json.MarshalIndent(interactions, "", "  ")

	prompt := fmt.Sprintf(`You are an expert Test Engineer.
Generate comprehensive integration tests in %s based on the following recorded API interactions.

Interactions:
'''
%s
'''

Requirements:
- Use standard testing libraries for %s (e.g. 'testing' for Go, 'pytest' for Python).
- Assert status codes and key body fields.
- Handle authentication headers if present in the recording.
- Make the tests standalone/runnable if possible (mocking might be needed if state depends on order, but prefer real integration style against a target URL).
- NOTE: The generated tests should target the original API URL found in the recording or a configurable base URL.

Return the code in a markdown block.
`, proxyLanguage, string(interactionsJSON), proxyLanguage)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	code := utils.CleanCodeBlock(resp)
	if err := os.WriteFile(proxyOutput, []byte(code), 0644); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Generated tests saved to %s\n", proxyOutput)
	return nil
}

type recordingTransport struct {
	transport http.RoundTripper
	onRecord  func(Interaction)
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Capture Request
	reqDump := ReqDump{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: req.Header,
	}

	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore
		reqDump.Body = string(bodyBytes)
	}

	// Execute
	start := time.Now()
	res, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Capture Response
	resDump := ResDump{
		Status:  res.StatusCode,
		Headers: res.Header,
	}

	if res.Body != nil {
		bodyBytes, _ := io.ReadAll(res.Body)
		res.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore
		resDump.Body = string(bodyBytes)
	}

	interaction := Interaction{
		Timestamp: start,
		Request:   reqDump,
		Response:  resDump,
	}

	t.onRecord(interaction)

	return res, nil
}
