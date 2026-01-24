package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"recac/internal/agent"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	mockServerPort    int
	mockServerSpec    string
	mockServerPrompt  string
	mockServerLatency time.Duration
)

var mockServerCmd = &cobra.Command{
	Use:   "mock-server",
	Short: "Start a mock API server backed by AI",
	Long: `Starts a local HTTP server that responds to requests by generating realistic JSON responses using the configured AI agent.
You can provide an OpenAPI spec (file) or a natural language description (prompt) to guide the agent.

Example:
  recac mock-server --port 8080 --spec openapi.yaml
  recac mock-server --port 8080 --prompt "A user management API with /users and /auth endpoints"`,
	RunE: runMockServer,
}

func init() {
	rootCmd.AddCommand(mockServerCmd)
	mockServerCmd.Flags().IntVarP(&mockServerPort, "port", "p", 8080, "Port to listen on")
	mockServerCmd.Flags().StringVarP(&mockServerSpec, "spec", "s", "", "Path to OpenAPI spec or description file")
	mockServerCmd.Flags().StringVarP(&mockServerPrompt, "prompt", "P", "", "Natural language description of the API")
	mockServerCmd.Flags().DurationVarP(&mockServerLatency, "latency", "l", 0, "Artificial latency (e.g. 500ms)")
}

func runMockServer(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Load Spec Content
	var contextContent string
	if mockServerSpec != "" {
		content, err := os.ReadFile(mockServerSpec)
		if err != nil {
			return fmt.Errorf("failed to read spec file: %w", err)
		}
		contextContent = fmt.Sprintf("API Specification:\n'''\n%s\n'''", string(content))
	} else if mockServerPrompt != "" {
		contextContent = fmt.Sprintf("API Description:\n%s", mockServerPrompt)
	} else {
		contextContent = "API Description: A generic RESTful API."
	}

	// Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-mock-server")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Create Handler
	handler := NewMockServerHandler(ctx, ag, contextContent, mockServerLatency, cmd.OutOrStdout())

	addr := fmt.Sprintf(":%d", mockServerPort)
	fmt.Fprintf(cmd.OutOrStdout(), "🚀 Mock Server listening on http://localhost%s\n", addr)
	if mockServerSpec != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "📘 Loaded spec: %s\n", mockServerSpec)
	}

	// Use http.Server for control if needed, but simple ListenAndServe is fine for CLI
	return http.ListenAndServe(addr, handler)
}

// NewMockServerHandler creates the HTTP handler for the mock server.
// Exported for testing purposes.
func NewMockServerHandler(ctx context.Context, ag agent.Agent, contextContent string, latency time.Duration, logOut io.Writer) http.HandlerFunc {
	if logOut == nil {
		logOut = io.Discard
	}
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			_, _ = fmt.Fprintf(logOut, "[%s] %s %s (%v)\n", time.Now().Format(time.TimeOnly), r.Method, r.URL.Path, time.Since(start))
		}()

		if latency > 0 {
			time.Sleep(latency)
		}

		// Read Body
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 1000 {
			bodyStr = bodyStr[:1000] + "... (truncated)"
		}

		// Construct Prompt
		prompt := fmt.Sprintf(`You are a high-fidelity Mock API Server.
Your job is to generate a realistic JSON response for the following HTTP request, based on the provided API Context.

%s

Incoming Request:
Method: %s
Path: %s
Headers: %v
Body:
'''
%s
'''

Instructions:
1. Analyze the request and the API Context.
2. Determine the appropriate HTTP Status Code (e.g. 200, 201, 400, 404).
3. Generate a realistic JSON response body.
4. Return the result in the following format:
STATUS: <StatusCode>
BODY:
<JSON_CONTENT>

Example:
STATUS: 200
BODY:
{"id": 1, "name": "Alice"}
`, contextContent, r.Method, r.URL.Path, r.Header, bodyStr)

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			http.Error(w, fmt.Sprintf("Agent error: %v", err), http.StatusInternalServerError)
			return
		}

		// Parse Response
		status := 200
		responseBody := "{}"

		lines := strings.Split(resp, "\n")
		var bodyLines []string
		inBody := false

		for _, line := range lines {
			if strings.HasPrefix(line, "STATUS:") {
				fmt.Sscanf(line, "STATUS: %d", &status)
			} else if strings.HasPrefix(line, "BODY:") {
				inBody = true
				continue
			} else if inBody {
				bodyLines = append(bodyLines, line)
			}
		}

		// If we collected body lines, join them.
		// If we didn't find "BODY:", maybe the agent just returned JSON?
		if len(bodyLines) > 0 {
			responseBody = strings.Join(bodyLines, "\n")
		} else if !strings.Contains(resp, "STATUS:") {
			// Fallback: assume the whole response is the body if it looks like JSON
			responseBody = resp
		}

		// Clean JSON block if present
		responseBody = utils.CleanJSONBlock(responseBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(responseBody))
	}
}
