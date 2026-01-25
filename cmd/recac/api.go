package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	apiOutputDir string
	apiScanDir   string
	apiGenSpec   bool
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Discover API endpoints and generate OpenAPI specs",
	Long:  `Scans the codebase for API endpoints (Go http handlers) and uses AI to generate an OpenAPI 3.0 specification.`,
	RunE:  runApi,
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.Flags().StringVarP(&apiOutputDir, "output", "o", "", "Output file for OpenAPI spec (e.g. openapi.yaml)")
	apiCmd.Flags().StringVarP(&apiScanDir, "dir", "d", ".", "Directory to scan for handlers")
	apiCmd.Flags().BoolVar(&apiGenSpec, "spec", false, "Generate OpenAPI spec using AI")
}

type Endpoint struct {
	Method      string
	Path        string
	HandlerName string
	HandlerCode string
	File        string
	Line        int
}

func runApi(cmd *cobra.Command, args []string) error {
	endpoints, err := findEndpoints(apiScanDir)
	if err != nil {
		return fmt.Errorf("failed to scan endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No API endpoints found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d endpoints:\n", len(endpoints))
	for _, ep := range endpoints {
		fmt.Fprintf(cmd.OutOrStdout(), "- %-7s %-30s (%s)\n", ep.Method, ep.Path, ep.HandlerName)
	}

	if apiGenSpec {
		if err := generateSpec(cmd, endpoints); err != nil {
			return err
		}
	}

	return nil
}

func findEndpoints(root string) ([]Endpoint, error) {
	var endpoints []Endpoint
	fset := token.NewFileSet()

	// Walk directories to find packages
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "node_modules" || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".") {
				return filepath.SkipDir
			}

			// Parse the package in this directory
			pkgs, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
			if err != nil {
				return nil // Skip unparsable dirs
			}

			for _, pkg := range pkgs {
				// 1. Build Index of Functions/Methods in this package
				funcIndex := make(map[string]*ast.FuncDecl)
				// We also need file content to extract source code later
				fileContents := make(map[string][]byte)

				for filename, f := range pkg.Files {
					content, err := os.ReadFile(filename)
					if err == nil {
						fileContents[filename] = content
					}

					for _, decl := range f.Decls {
						if fn, ok := decl.(*ast.FuncDecl); ok {
							name := fn.Name.Name
							if fn.Recv != nil && len(fn.Recv.List) > 0 {
								// Method: Type.Method
								typeExpr := fn.Recv.List[0].Type
								typeName := ""
								if star, ok := typeExpr.(*ast.StarExpr); ok {
									if ident, ok := star.X.(*ast.Ident); ok {
										typeName = ident.Name
									}
								} else if ident, ok := typeExpr.(*ast.Ident); ok {
									typeName = ident.Name
								}
								if typeName != "" {
									name = typeName + "." + name
								}
							}
							funcIndex[name] = fn
						}
					}
				}

				// 2. Scan for Routes
				for filename, f := range pkg.Files {
					ast.Inspect(f, func(n ast.Node) bool {
						call, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}

						var method string
						var pathArg ast.Expr
						var handlerArg ast.Expr

						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							name := sel.Sel.Name
							switch name {
							case "HandleFunc":
								if len(call.Args) >= 2 {
									method = "ANY"
									pathArg = call.Args[0]
									handlerArg = call.Args[1]
								}
							case "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD":
								if len(call.Args) >= 2 {
									method = name
									pathArg = call.Args[0]
									// Assume last arg is handler for middleware support
									handlerArg = call.Args[len(call.Args)-1]
								}
							case "Handle":
								if len(call.Args) == 2 {
									method = "ANY"
									pathArg = call.Args[0]
									handlerArg = call.Args[1]
								} else if len(call.Args) >= 3 {
									// r.Handle("METHOD", "/path", handler)
									if bl, ok := call.Args[0].(*ast.BasicLit); ok && bl.Kind == token.STRING {
										method = strings.Trim(bl.Value, "\"")
										pathArg = call.Args[1]
										handlerArg = call.Args[2]
									}
								}
							}
						}

						if method != "" && pathArg != nil && handlerArg != nil {
							// Extract Path
							pathStr := ""
							if bl, ok := pathArg.(*ast.BasicLit); ok && bl.Kind == token.STRING {
								pathStr = strings.Trim(bl.Value, "\"")
							} else {
								pathStr = "{dynamic}"
							}

							// Extract Handler Name & Code
							handlerName := "anonymous"
							handlerCode := ""

							// Unwrap CallExpr (e.g. http.HandlerFunc(handler))
							unwrappedHandlerArg := handlerArg
							if callExpr, ok := handlerArg.(*ast.CallExpr); ok && len(callExpr.Args) > 0 {
								unwrappedHandlerArg = callExpr.Args[0]
							}

							if fl, ok := unwrappedHandlerArg.(*ast.FuncLit); ok {
								handlerName = "func(...)"
								// Extract code from FuncLit
								if content, ok := fileContents[filename]; ok {
									start := fset.Position(fl.Pos()).Offset
									end := fset.Position(fl.End()).Offset
									if start < len(content) && end <= len(content) {
										handlerCode = string(content[start:end])
									}
								}
							} else if ident, ok := unwrappedHandlerArg.(*ast.Ident); ok {
								handlerName = ident.Name
								if fn, found := funcIndex[handlerName]; found {
									// Extract code from FuncDecl
									fPos := fset.Position(fn.Pos())
									fEnd := fset.Position(fn.End())
									if content, ok := fileContents[fPos.Filename]; ok {
										if fPos.Offset < len(content) && fEnd.Offset <= len(content) {
											handlerCode = string(content[fPos.Offset:fEnd.Offset])
										}
									}
								}
							} else if sel, ok := unwrappedHandlerArg.(*ast.SelectorExpr); ok {
								// Method call or package function
								// We only support methods in same package for now
								// x.Method -> we need to know type of x. Hard.
								// But if it matches a known "Type.Method" in our index, we might get lucky?
								// Only if X is an Ident that maps to a Type? No, X is a variable.
								// So we rely on method name uniqueness or skip code extraction.
								handlerName = sel.Sel.Name // Just the method name
								// Heuristic: check if any function ends with .Name
								for k, fn := range funcIndex {
									if strings.HasSuffix(k, "."+handlerName) {
										// Found a candidate
										fPos := fset.Position(fn.Pos())
										fEnd := fset.Position(fn.End())
										if content, ok := fileContents[fPos.Filename]; ok {
											if fPos.Offset < len(content) && fEnd.Offset <= len(content) {
												handlerCode = string(content[fPos.Offset:fEnd.Offset])
											}
										}
										break
									}
								}
							}

							endpoints = append(endpoints, Endpoint{
								Method:      method,
								Path:        pathStr,
								HandlerName: handlerName,
								HandlerCode: handlerCode,
								File:        filename,
								Line:        fset.Position(n.Pos()).Line,
							})
						}
						return true
					})
				}
			}
		}
		return nil
	})

	return endpoints, err
}

func generateSpec(cmd *cobra.Command, endpoints []Endpoint) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-api")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n🤖 Generating OpenAPI spec for %d endpoints...\n", len(endpoints))

	// Open output file if needed
	var out io.Writer = cmd.OutOrStdout()
	if apiOutputDir != "" {
		f, err := os.Create(apiOutputDir)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	// We'll process endpoints in parallel to speed up?
	// But agent might be rate limited. Let's do it sequentially or bounded parallel.
	// Sequential for now to avoid complexity and rate limits.

	type PathItem map[string]interface{}
	paths := make(map[string]PathItem)
	var pathsMutex sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3) // concurrency limit

	for _, ep := range endpoints {
		wg.Add(1)
		sem <- struct{}{} // acquire before spawning to limit goroutines
		go func(ep Endpoint) {
			defer wg.Done()
			defer func() { <-sem }() // release

			prompt := fmt.Sprintf(`Generate an OpenAPI 3.0 path item for the following API endpoint.
Method: %s
Path: %s
Handler Code:
%s

Instructions:
1. Return ONLY the YAML snippet for the method key (e.g. 'get:', 'post:').
2. Infer request body and response schemas from the code if possible.
3. If code is empty, generate a generic definition.
4. Do NOT include the path key (e.g. '/users'), just the method object.
5. Do NOT include markdown code blocks.
`, ep.Method, ep.Path, ep.HandlerCode)

			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to generate spec for %s %s: %v\n", ep.Method, ep.Path, err)
				return
			}

			// Clean cleanup
			resp = strings.TrimSpace(resp)
			resp = strings.TrimPrefix(resp, "```yaml")
			resp = strings.TrimPrefix(resp, "```")
			resp = strings.TrimSuffix(resp, "```")

			// We need to parse it to merge into paths
			var snippet map[string]interface{}
			if err := yaml.Unmarshal([]byte(resp), &snippet); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: invalid YAML for %s %s\n", ep.Method, ep.Path)
				return
			}

			pathsMutex.Lock()
			if paths[ep.Path] == nil {
				paths[ep.Path] = make(PathItem)
			}
			// Merge methods
			for k, v := range snippet {
				paths[ep.Path][k] = v
			}
			pathsMutex.Unlock()

			fmt.Fprintf(cmd.ErrOrStderr(), ".")
		}(ep)
	}

	wg.Wait()
	fmt.Fprintln(cmd.ErrOrStderr(), " Done!")

	// Build Final Spec
	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":   "Generated API",
			"version": "1.0.0",
		},
		"paths": paths,
	}

	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	return enc.Encode(spec)
}
