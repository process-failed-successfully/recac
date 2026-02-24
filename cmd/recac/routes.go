package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/analysis"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	routesFormat string
	routesOutput string
)

var routesCmd = &cobra.Command{
	Use:   "routes [path]",
	Short: "List API routes and generate OpenAPI specs",
	Long: `Scans the codebase for API route definitions (supports Gin, Echo, net/http).
Can output in table, JSON, or generate an OpenAPI 3.0 specification using AI.`,
	RunE: runRoutes,
}

func init() {
	rootCmd.AddCommand(routesCmd)
	routesCmd.Flags().StringVarP(&routesFormat, "format", "f", "table", "Output format (table, json, openapi)")
	routesCmd.Flags().StringVarP(&routesOutput, "output", "o", "", "Output file path")
}

func runRoutes(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Scan Routes
	routes, err := analysis.ScanRoutes(root)
	if err != nil {
		return fmt.Errorf("failed to scan routes: %w", err)
	}

	if len(routes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No API routes found.")
		return nil
	}

	var output string

	// 2. Format Output
	switch strings.ToLower(routesFormat) {
	case "json":
		data, err := json.MarshalIndent(routes, "", "  ")
		if err != nil {
			return err
		}
		output = string(data)

	case "openapi":
		spec, err := generateOpenAPI(cmd, routes)
		if err != nil {
			return err
		}
		output = spec

	case "table":
		var sb strings.Builder
		w := tabwriter.NewWriter(&sb, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "METHOD\tPATH\tHANDLER\tFILE:LINE")
		for _, r := range routes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s:%d\n", r.Method, r.Path, r.Handler, r.File, r.Line)
		}
		w.Flush()
		output = sb.String()

	default:
		return fmt.Errorf("unknown format: %s", routesFormat)
	}

	// 3. Output
	if routesOutput != "" {
		if err := os.WriteFile(routesOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Printf("Output written to %s\n", routesOutput)
	} else {
		fmt.Println(output)
	}

	return nil
}

func generateOpenAPI(cmd *cobra.Command, routes []analysis.Route) (string, error) {
	ctx := context.Background()
	cwd, _ := os.Getwd()

	// Prepare the prompt
	promptBuilder := strings.Builder{}
	promptBuilder.WriteString("Generate a valid OpenAPI 3.0 (YAML) specification for the following API routes found in a Go codebase.\n")
	promptBuilder.WriteString("Infer sensible summary, description, and responses based on the HTTP method and path.\n")
	promptBuilder.WriteString("Return ONLY the YAML content.\n\n")

	for _, r := range routes {
		promptBuilder.WriteString(fmt.Sprintf("- %s %s (Handler: %s, Source: `%s`)\n", r.Method, r.Path, r.Handler, r.Source))
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating OpenAPI spec using AI...")

	// Create Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-routes")
	if err != nil {
		return "", fmt.Errorf("failed to create agent: %w", err)
	}

	// Send Request
	resp, err := ag.Send(ctx, promptBuilder.String())
	if err != nil {
		return "", err
	}

	// Clean Markdown block if present
	if strings.HasPrefix(strings.TrimSpace(resp), "```") {
		lines := strings.Split(strings.TrimSpace(resp), "\n")
		if len(lines) > 2 {
			resp = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	return resp, nil
}
