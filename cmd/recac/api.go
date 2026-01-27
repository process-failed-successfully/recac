package main

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	apiDescribe   bool
	apiSpecOutput string
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Manage and discover API endpoints",
	Long:  `Discover, analyze, and manage API endpoints within the codebase.`,
}

var apiScanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan for API endpoints in the codebase",
	Long:  `Scans Go files for standard net/http handlers and lists them.`,
	RunE:  runApiScan,
}

var apiSpecCmd = &cobra.Command{
	Use:   "spec [path]",
	Short: "Generate OpenAPI 3.0 specification from code",
	Long:  `Scans for API endpoints and uses AI to generate an OpenAPI 3.0 specification file (YAML).`,
	RunE:  runApiSpec,
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.AddCommand(apiScanCmd)
	apiCmd.AddCommand(apiSpecCmd)
	apiScanCmd.Flags().BoolVarP(&apiDescribe, "describe", "d", false, "Use AI to describe the endpoints")
	apiSpecCmd.Flags().StringVarP(&apiSpecOutput, "output", "o", "openapi.yaml", "Output file for OpenAPI spec")
}

type ApiEndpoint struct {
	Path        string
	Method      string // Often implicit in net/http, but we can try to guess or defaults to ANY
	HandlerName string
	File        string
	Line        int
	Description string
	SourceCode  string // For AI analysis
}

func runApiScan(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	endpoints, err := scanForEndpoints(root)
	if err != nil {
		return err
	}

	if len(endpoints) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No API endpoints found.")
		return nil
	}

	if apiDescribe {
		if err := describeEndpoints(cmd.Context(), endpoints); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: AI description failed: %v\n", err)
		}
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	if apiDescribe {
		fmt.Fprintln(w, "PATH\tHANDLER\tDESCRIPTION\tLOCATION")
	} else {
		fmt.Fprintln(w, "PATH\tHANDLER\tLOCATION")
	}

	for _, ep := range endpoints {
		loc := fmt.Sprintf("%s:%d", ep.File, ep.Line)
		if apiDescribe {
			// Truncate description
			desc := strings.Split(ep.Description, "\n")[0]
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ep.Path, ep.HandlerName, desc, loc)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", ep.Path, ep.HandlerName, loc)
		}
	}
	w.Flush()

	return nil
}

func runApiSpec(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	endpoints, err := scanForEndpoints(root)
	if err != nil {
		return err
	}

	if len(endpoints) == 0 {
		return fmt.Errorf("no API endpoints found to generate spec")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d endpoints. Analyzing source code...\n", len(endpoints))

	// Construct context for AI
	var sb strings.Builder
	for _, ep := range endpoints {
		sb.WriteString(fmt.Sprintf("Endpoint: %s\nHandler: %s\n", ep.Path, ep.HandlerName))
		if ep.SourceCode != "" {
			sb.WriteString("Source Code:\n```go\n")
			sb.WriteString(ep.SourceCode)
			sb.WriteString("\n```\n")
		} else {
			sb.WriteString("(Source code not found)\n")
		}
		sb.WriteString("---\n")
	}

	prompt := fmt.Sprintf(`Generate an OpenAPI 3.0 specification (YAML) for the following Go API endpoints.
Infer the request parameters and response schemas from the provided source code where possible.
Use generic schemas (Object, String) if the types are not clear.
Include a basic "info" section with title "Generated API" and version "1.0.0".

Endpoints Context:
%s

Output ONLY the raw YAML content. Do not wrap in markdown blocks.`, sb.String())

	ctx := cmd.Context()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-api-spec")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating OpenAPI spec...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	yamlContent := utils.CleanCodeBlock(resp)
	if strings.TrimSpace(yamlContent) == "" {
		return fmt.Errorf("agent returned empty response")
	}

	if err := os.WriteFile(apiSpecOutput, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ OpenAPI spec written to %s\n", apiSpecOutput)
	return nil
}

func scanForEndpoints(root string) ([]*ApiEndpoint, error) {
	var endpoints []*ApiEndpoint
	fset := token.NewFileSet()

	// Helper to print AST node
	printNode := func(n ast.Node) string {
		var buf bytes.Buffer
		printer.Fprint(&buf, fset, n)
		return buf.String()
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		// Collect declarations in the current file
		declarations := make(map[string]*ast.FuncDecl)
		for _, decl := range node.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				declarations[fn.Name.Name] = fn
			}
		}

		ast.Inspect(node, func(n ast.Node) bool {
			// Look for CallExpr
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check for http.HandleFunc(path, func)
			// or http.Handle(path, handler)
			// or mux.HandleFunc(...)
			fun, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			// Check method name
			if fun.Sel.Name != "HandleFunc" && fun.Sel.Name != "Handle" {
				return true
			}

			// Check args
			if len(call.Args) < 2 {
				return true
			}

			// First arg should be path string literal
			pathLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || pathLit.Kind != token.STRING {
				// Might be a variable, skip for now
				return true
			}
			endpointPath := strings.Trim(pathLit.Value, "\"")

			// Second arg is handler
			handlerName := "anonymous"
			var sourceCode string

			switch h := call.Args[1].(type) {
			case *ast.Ident:
				handlerName = h.Name
				// Try to find the function declaration in the same file
				if fnDecl, found := declarations[handlerName]; found {
					sourceCode = printNode(fnDecl)
				}
			case *ast.SelectorExpr:
				handlerName = fmt.Sprintf("%s.%s", h.X, h.Sel.Name)
			case *ast.FuncLit:
				handlerName = "func(...)"
				sourceCode = printNode(h)
			case *ast.CallExpr:
				// http.HandlerFunc(...) wrapper
				// We need to unwrap it
				if fun, ok := h.Fun.(*ast.SelectorExpr); ok && fun.Sel.Name == "HandlerFunc" {
					if len(h.Args) > 0 {
						if ident, ok := h.Args[0].(*ast.Ident); ok {
							handlerName = ident.Name
							if fnDecl, found := declarations[handlerName]; found {
								sourceCode = printNode(fnDecl)
							}
						} else if lit, ok := h.Args[0].(*ast.FuncLit); ok {
							handlerName = "func(...)"
							sourceCode = printNode(lit)
						}
					}
				}
			}

			ep := &ApiEndpoint{
				Path:        endpointPath,
				Method:      "ANY", // Default for HandleFunc
				HandlerName: handlerName,
				File:        path,
				Line:        fset.Position(call.Pos()).Line,
				SourceCode:  sourceCode,
			}
			endpoints = append(endpoints, ep)

			return true
		})

		return nil
	})

	return endpoints, err
}

func describeEndpoints(ctx context.Context, endpoints []*ApiEndpoint) error {
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-api")
	if err != nil {
		return err
	}

	for _, ep := range endpoints {
		prompt := fmt.Sprintf("Describe the purpose of the API endpoint '%s' handled by '%s'. Keep it very short (max 10 words).", ep.Path, ep.HandlerName)
		resp, err := ag.Send(ctx, prompt)
		if err == nil {
			ep.Description = strings.TrimSpace(resp)
		} else {
			ep.Description = "Analysis failed"
		}
	}
	return nil
}
