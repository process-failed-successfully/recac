package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	contractSpec   string
	contractTarget string
	contractAI     bool
	contractOutput string
)

var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "API Contract Testing",
	Long:  `Validate API implementations against their OpenAPI/Swagger specifications using AI.`,
}

var contractVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify API implementation against spec",
	Long: `Parses an OpenAPI spec and verifies that a running API implementation conforms to it.
It uses AI to generate realistic request payloads and to validate complex response logic.

Example:
  recac contract verify --spec openapi.yaml --target http://localhost:8080`,
	RunE: runContractVerify,
}

func init() {
	rootCmd.AddCommand(contractCmd)
	contractCmd.AddCommand(contractVerifyCmd)

	contractVerifyCmd.Flags().StringVarP(&contractSpec, "spec", "s", "", "Path to OpenAPI specification file (required)")
	contractVerifyCmd.Flags().StringVarP(&contractTarget, "target", "t", "", "Target API URL (required)")
	contractVerifyCmd.Flags().BoolVar(&contractAI, "ai", true, "Use AI for generation and verification")
	contractVerifyCmd.Flags().StringVarP(&contractOutput, "output", "o", "", "Output report to JSON file")
}

type ContractCheckResult struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason"`
	Duration string `json:"duration"`
}

type AIRequestParams struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    interface{}       `json:"body"`
}

type AIVerificationResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

func runContractVerify(cmd *cobra.Command, args []string) error {
	if contractSpec == "" || contractTarget == "" {
		return fmt.Errorf("--spec and --target are required")
	}

	// 1. Parse Spec
	data, err := os.ReadFile(contractSpec)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	var specMap map[string]interface{}
	if err := yaml.Unmarshal(data, &specMap); err != nil {
		return fmt.Errorf("failed to parse YAML spec: %w", err)
	}

	paths, ok := specMap["paths"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid OpenAPI spec: 'paths' not found or invalid format")
	}

	// 2. Setup Agent
	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-contract")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	var results []ContractCheckResult
	client := &http.Client{Timeout: 10 * time.Second}

	// Iterate deterministically
	var pathKeys []string
	for p := range paths {
		pathKeys = append(pathKeys, p)
	}
	sort.Strings(pathKeys)

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Verifying contract for %s against %s...\n\n", contractSpec, contractTarget)

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tMETHOD\tPATH\tREASON")

	for _, pathKey := range pathKeys {
		pathItem, ok := paths[pathKey].(map[string]interface{})
		if !ok {
			continue
		}

		var methods []string
		for m := range pathItem {
			method := strings.ToUpper(m)
			if method == "GET" || method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH" {
				methods = append(methods, m)
			}
		}
		sort.Strings(methods)

		for _, methodKey := range methods {
			method := strings.ToUpper(methodKey)
			start := time.Now()

			// Prepare Prompt for Request Generation
			// We convert the path item part to YAML string to give context
			pathBytes, _ := yaml.Marshal(pathItem[methodKey])

			// A. Generate Request
			genPrompt := fmt.Sprintf(`You are an API Tester.
Generate a valid HTTP request for the following OpenAPI endpoint.
Target Base URL: %s
Path: %s
Method: %s

Spec:
'''
%s
'''

Return ONLY a JSON object with keys: "method", "path" (replacing parameters like {id} with values), "headers", "query", "body".
Example:
{
  "method": "POST",
  "path": "/users",
  "headers": {"Content-Type": "application/json"},
  "query": {},
  "body": {"name": "Test"}
}`, contractTarget, pathKey, method, string(pathBytes))

			genResp, err := ag.Send(ctx, genPrompt)
			if err != nil {
				recordResult(w, method, pathKey, false, fmt.Sprintf("Agent gen failed: %v", err), start, &results)
				continue
			}

			var reqParams AIRequestParams
			cleanGenResp := utils.CleanJSONBlock(genResp)
			if err := json.Unmarshal([]byte(cleanGenResp), &reqParams); err != nil {
				recordResult(w, method, pathKey, false, fmt.Sprintf("Invalid agent JSON: %v", err), start, &results)
				continue
			}

			// Construct URL
			// reqParams.Path should be fully formed path (e.g. /users/1)
			// Assume target has no trailing slash, path has leading slash
			target := strings.TrimRight(contractTarget, "/")
			reqPath := reqParams.Path
			if !strings.HasPrefix(reqPath, "/") {
				reqPath = "/" + reqPath
			}
			fullURL := target + reqPath

			// Append query params
			if len(reqParams.Query) > 0 {
				params := make([]string, 0)
				for k, v := range reqParams.Query {
					params = append(params, fmt.Sprintf("%s=%s", k, v))
				}
				if strings.Contains(fullURL, "?") {
					fullURL += "&" + strings.Join(params, "&")
				} else {
					fullURL += "?" + strings.Join(params, "&")
				}
			}

			// Body
			var bodyReader io.Reader
			if reqParams.Body != nil {
				bodyBytes, _ := json.Marshal(reqParams.Body)
				bodyReader = bytes.NewBuffer(bodyBytes)
			}

			req, err := http.NewRequest(reqParams.Method, fullURL, bodyReader)
			if err != nil {
				recordResult(w, method, pathKey, false, fmt.Sprintf("Failed to create request: %v", err), start, &results)
				continue
			}

			for k, v := range reqParams.Headers {
				req.Header.Set(k, v)
			}

			// B. Execute Request
			resp, err := client.Do(req)
			if err != nil {
				recordResult(w, method, pathKey, false, fmt.Sprintf("Request failed: %v", err), start, &results)
				continue
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			// C. Verify Response
			verifyPrompt := fmt.Sprintf(`You are an API Verifier.
Check if the actual API response conforms to the OpenAPI spec.

Spec:
'''
%s
'''

Actual Response:
Status: %d
Headers: %v
Body:
'''
%s
'''

Return ONLY a JSON object: {"passed": boolean, "reason": "string explanation"}
`, string(pathBytes), resp.StatusCode, resp.Header, string(respBody))

			verifyResp, err := ag.Send(ctx, verifyPrompt)
			if err != nil {
				recordResult(w, method, pathKey, false, fmt.Sprintf("Agent verify failed: %v", err), start, &results)
				continue
			}

			var verifyResult AIVerificationResult
			cleanVerifyResp := utils.CleanJSONBlock(verifyResp)
			if err := json.Unmarshal([]byte(cleanVerifyResp), &verifyResult); err != nil {
				// Fallback: assume failure if JSON parse fails
				recordResult(w, method, pathKey, false, fmt.Sprintf("Invalid verify JSON: %v", err), start, &results)
				continue
			}

			recordResult(w, method, pathKey, verifyResult.Passed, verifyResult.Reason, start, &results)
		}
	}

	w.Flush()
	fmt.Fprintln(cmd.OutOrStdout(), "")

	// Summary
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Summary: %d/%d passed.\n", passed, len(results))

	if contractOutput != "" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(contractOutput, data, 0644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("Report saved to %s\n", contractOutput)
	}

	if passed < len(results) {
		return fmt.Errorf("contract verification failed")
	}

	return nil
}

func recordResult(w io.Writer, method, path string, passed bool, reason string, start time.Time, results *[]ContractCheckResult) {
	duration := time.Since(start).String()
	icon := "❌"
	if passed {
		icon = "✅"
	}

	// Truncate reason for display
	displayReason := reason
	if len(displayReason) > 50 {
		displayReason = displayReason[:47] + "..."
	}

	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", icon, method, path, displayReason)

	*results = append(*results, ContractCheckResult{
		Method:   method,
		Path:     path,
		Passed:   passed,
		Reason:   reason,
		Duration: duration,
	})
}
