package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	apiDescribe bool
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

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.AddCommand(apiScanCmd)
	apiScanCmd.Flags().BoolVarP(&apiDescribe, "describe", "d", false, "Use AI to describe the endpoints")
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

func scanForEndpoints(root string) ([]*ApiEndpoint, error) {
	var endpoints []*ApiEndpoint
	fset := token.NewFileSet()

	// Helper to find function source
	findFuncSource := func(path, funcName string) string {
		// This is a naive implementation. Ideally we parse the file again or keep the AST.
		// For now, we will just return empty string if not easy to find.
		// A better approach is to keep AST nodes.
		return ""
	}

	// We need to keep ASTs to extract source code later if needed
	files := make(map[string]*ast.File)

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
		files[path] = node

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
				sourceCode = findFuncSource(path, handlerName)
			case *ast.SelectorExpr:
				handlerName = fmt.Sprintf("%s.%s", h.X, h.Sel.Name)
			case *ast.FuncLit:
				handlerName = "func(...)"
				// We can try to get source of anonymous func
				// pos := fset.Position(h.Pos())
				// end := fset.Position(h.End())
			case *ast.CallExpr:
				// http.HandlerFunc(...) wrapper
				// We need to unwrap it
				if fun, ok := h.Fun.(*ast.SelectorExpr); ok && fun.Sel.Name == "HandlerFunc" {
					if len(h.Args) > 0 {
						if ident, ok := h.Args[0].(*ast.Ident); ok {
							handlerName = ident.Name
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
				SourceCode:  sourceCode, // Placeholder
			}
			endpoints = append(endpoints, ep)

			return true
		})

		return nil
	})

	// If we need source code for descriptions, we need a second pass or better AST handling.
	// For this MVP, we will only extract source if we can find the function declaration in the same package/file.
	// To simplify, let's just use the function name for the prompt if source is hard to get.

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
