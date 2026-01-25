package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	apiOutput  string
	apiPort    int
	apiExclude []string
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Manage API documentation",
	Long:  `Scan codebase to generate OpenAPI specs or serve them via Swagger UI.`,
}

var apiScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Generate OpenAPI spec from codebase",
	Long:  `Analyzes the codebase to identify API endpoints and generates an OpenAPI 3.0 specification.`,
	RunE:  runApiScan,
}

var apiServeCmd = &cobra.Command{
	Use:   "serve [spec-file]",
	Short: "Serve OpenAPI spec with Swagger UI",
	Long:  `Starts a local web server to visualize the OpenAPI specification using Swagger UI.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runApiServe,
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.AddCommand(apiScanCmd)
	apiCmd.AddCommand(apiServeCmd)

	apiScanCmd.Flags().StringVarP(&apiOutput, "output", "o", "openapi.yaml", "Output file path")
	apiScanCmd.Flags().StringSliceVarP(&apiExclude, "exclude", "e", []string{}, "Glob patterns to exclude from scan")

	apiServeCmd.Flags().IntVarP(&apiPort, "port", "p", 8081, "Port to serve on")
}

func runApiScan(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Scanning codebase for API endpoints...")
	// Reusing collectProjectContext from spec.go (same package main)
	contextStr, err := collectProjectContext(cwd, apiExclude)
	if err != nil {
		return fmt.Errorf("failed to collect project context: %w", err)
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	projectName := filepath.Base(cwd)

	ag, err := agentClientFactory(ctx, provider, model, cwd, projectName)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert API Architect.
Analyze the following codebase to identify all API endpoints (HTTP, REST, GraphQL, or gRPC).
Generate a complete OpenAPI 3.0 specification (YAML) that describes these endpoints.

Include:
- Paths and Methods (GET, POST, etc.)
- Request Parameters and Body schemas (inferred from code)
- Response schemas (inferred from code)
- Summary and Description for each operation

Output ONLY the raw YAML content. Do not include markdown code blocks.

Codebase Context:
%s
`, contextStr)

	fmt.Fprintln(cmd.OutOrStdout(), "Generating OpenAPI spec...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Clean output
	content := utils.CleanCodeBlock(resp)

	if err := os.WriteFile(apiOutput, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ OpenAPI spec saved to %s\n", apiOutput)
	return nil
}

func runApiServe(cmd *cobra.Command, args []string) error {
	specFile := "openapi.yaml"
	if len(args) > 0 {
		specFile = args[0]
	}

	if _, err := os.Stat(specFile); os.IsNotExist(err) {
		return fmt.Errorf("spec file '%s' not found. Run 'recac api scan' first or provide a path", specFile)
	}

	addr := fmt.Sprintf(":%d", apiPort)
	fmt.Fprintf(cmd.OutOrStdout(), "Starting Swagger UI at http://localhost%s\n", addr)
	fmt.Fprintf(cmd.OutOrStdout(), "Serving spec from: %s\n", specFile)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <meta
    name="description"
    content="SwaggerUI"
  />
  <title>SwaggerUI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-bundle.js" crossorigin></script>
<script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-standalone-preset.js" crossorigin></script>
<script>
  window.onload = () => {
    window.ui = SwaggerUIBundle({
      url: '/spec',
      dom_id: '#swagger-ui',
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIStandalonePreset
      ],
      layout: "StandaloneLayout",
    });
  };
</script>
</body>
</html>
`)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	mux.HandleFunc("/spec", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, specFile)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	return server.ListenAndServe()
}
