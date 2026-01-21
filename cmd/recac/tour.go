package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tourCmd = &cobra.Command{
	Use:   "tour [dir]",
	Short: "Interactive guided tour of the codebase",
	Long:  `Analyzes the Go package in the current (or specified) directory and lets you interactively navigate the function call graph. You can step into function calls, view code, and ask the AI to explain logic.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

// TourState maintains the current navigation stack
type TourState struct {
	Stack    []string // Function names
	Funcs    map[string]*FunctionNode
	Current  string
	FileSet  *token.FileSet
	RootPath string
}

// FunctionNode represents a function in the codebase
type FunctionNode struct {
	Name      string
	Package   string
	FilePath  string
	Doc       string
	Body      string
	Calls     []string // Names of functions called by this one
	StartLine int
	EndLine   int
}

func runTour(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Analyzing package in %s...\n", absDir)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, absDir, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse directory: %w", err)
	}

	funcs := make(map[string]*FunctionNode)
	var mainFunc string

	// Analyze AST
	for _, pkg := range pkgs {
		for filePath, file := range pkg.Files {
			// Read file content for extracting body source
			content, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}

			ast.Inspect(file, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}

				name := fn.Name.Name
				// Handle receiver methods: Type.Method
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					typeExpr := fn.Recv.List[0].Type
					typeName := ""
					// Handle pointer receivers (*Type)
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

				// Extract body source
				start := fset.Position(fn.Pos()).Offset
				end := fset.Position(fn.End()).Offset
				bodySrc := string(content[start:end])

				// Find called functions
				var calls []string
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}

					// Simple function call: foo()
					if ident, ok := call.Fun.(*ast.Ident); ok {
						calls = append(calls, ident.Name)
					} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						// Method call: x.foo() or pkg.foo()
						// We try to capture "Type.Method" if possible, but identifying Type requires type checking which is complex.
						// For now, we just capture the Method name "foo" or "pkg.foo".
						// If X is an identifier (e.g. pkg name), we might get "pkg.Func".
						if xIdent, ok := sel.X.(*ast.Ident); ok {
							// Check if xIdent is a package import? We don't have that info easily without type check.
							// Heuristic: Store "X.Sel"
							calls = append(calls, xIdent.Name+"."+sel.Sel.Name)
						} else {
							// Just the method name
							calls = append(calls, sel.Sel.Name)
						}
					}
					return true
				})

				doc := ""
				if fn.Doc != nil {
					doc = fn.Doc.Text()
				}

				node := &FunctionNode{
					Name:      name,
					Package:   pkg.Name,
					FilePath:  filePath,
					Doc:       doc,
					Body:      bodySrc,
					Calls:     uniqueStrings(calls),
					StartLine: fset.Position(fn.Pos()).Line,
					EndLine:   fset.Position(fn.End()).Line,
				}
				funcs[name] = node

				if name == "main" && pkg.Name == "main" {
					mainFunc = name
				}
				return false // Don't traverse deep into FuncDecl body with top-level Inspect, we did inner Inspect
			})
		}
	}

	if len(funcs) == 0 {
		return fmt.Errorf("no functions found in %s", absDir)
	}

	// Determine entry point
	current := mainFunc
	if current == "" {
		// Pick the first declared function alphabetically
		var names []string
		for n := range funcs {
			names = append(names, n)
		}
		sort.Strings(names)
		current = names[0]
	}

	state := &TourState{
		Funcs:    funcs,
		Current:  current,
		FileSet:  fset,
		RootPath: absDir,
	}

	return startTourLoop(cmd, state)
}

func startTourLoop(cmd *cobra.Command, state *TourState) error {
	for {
		node, exists := state.Funcs[state.Current]
		if !exists {
			fmt.Printf("\nError: Function '%s' not found in analysis. (It might be external)\n", state.Current)
			if len(state.Stack) > 0 {
				state.Current = state.Stack[len(state.Stack)-1]
				state.Stack = state.Stack[:len(state.Stack)-1]
				continue
			}
			return nil
		}

		// Display Header
		fmt.Fprintln(cmd.OutOrStdout(), "\n==================================================")
		fmt.Fprintf(cmd.OutOrStdout(), "📍 Current Function: %s (Pkg: %s)\n", node.Name, node.Package)
		fmt.Fprintf(cmd.OutOrStdout(), "📄 File: %s:%d\n", filepath.Base(node.FilePath), node.StartLine)
		if node.Doc != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "💡 Doc: %s", strings.TrimSpace(node.Doc))
		}
		fmt.Fprintln(cmd.OutOrStdout(), "--------------------------------------------------")

		// Identify known internal calls
		var internalCalls []string
		var externalCalls []string

		// Map display label -> target function name
		// We might have multiple targets for one call (ambiguous methods)
		stepOptions := make(map[string]string)

		for _, call := range node.Calls {
			if _, ok := state.Funcs[call]; ok {
				// Exact match
				internalCalls = append(internalCalls, call)
				stepOptions[fmt.Sprintf("Step into: %s()", call)] = call
			} else {
				// Heuristic: Check for method suffix match
				// Call might be "var.Method" or just "Method" (if inside struct)
				// Target is "Type.Method"

				callSuffix := "." + call
				if !strings.Contains(call, ".") {
					callSuffix = "." + call
				} else {
					// e.g. "repo.Save" -> suffix ".Save"
					parts := strings.Split(call, ".")
					if len(parts) > 0 {
						callSuffix = "." + parts[len(parts)-1]
					}
				}

				var candidates []string
				for knownName := range state.Funcs {
					if strings.HasSuffix(knownName, callSuffix) {
						candidates = append(candidates, knownName)
					}
				}

				if len(candidates) > 0 {
					// Found potential matches
					for _, c := range candidates {
						internalCalls = append(internalCalls, c)
						label := fmt.Sprintf("Step into: %s() (via %s)", c, call)
						stepOptions[label] = c
					}
				} else {
					externalCalls = append(externalCalls, call)
				}
			}
		}

		// Construct Menu
		var options []string
		actionMap := make(map[string]string)

		// 1. Calls (Step Into)
		// Sort keys of stepOptions for stability
		var stepLabels []string
		for label := range stepOptions {
			stepLabels = append(stepLabels, label)
		}
		sort.Strings(stepLabels)

		if len(stepLabels) > 0 {
			for _, label := range stepLabels {
				options = append(options, label)
				actionMap[label] = "call:" + stepOptions[label]
			}
		}

		// 2. Main Actions
		options = append(options, "Show Code")
		options = append(options, "Explain with AI")

		if len(state.Stack) > 0 {
			options = append(options, "Back to Caller")
		}

		if len(externalCalls) > 0 {
			options = append(options, fmt.Sprintf("List External Calls (%d)", len(externalCalls)))
		}

		options = append(options, "Quit")

		var selection string
		err := askOneFunc(&survey.Select{
			Message: "What would you like to do?",
			Options: options,
			PageSize: 10,
		}, &selection)

		if err != nil {
			return nil // User cancelled
		}

		action := selection
		if mapped, ok := actionMap[selection]; ok {
			action = mapped
		}

		switch {
		case strings.HasPrefix(action, "call:"):
			target := strings.TrimPrefix(action, "call:")
			state.Stack = append(state.Stack, state.Current)
			state.Current = target

		case action == "Show Code":
			fmt.Fprintln(cmd.OutOrStdout(), "\n```go")
			fmt.Fprintln(cmd.OutOrStdout(), node.Body)
			fmt.Fprintln(cmd.OutOrStdout(), "```")
			// Pause?
			var ignore string
			askOneFunc(&survey.Input{Message: "Press Enter to continue..."}, &ignore)

		case action == "Explain with AI":
			if err := explainFunction(cmd, node); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			}
			var ignore string
			askOneFunc(&survey.Input{Message: "Press Enter to continue..."}, &ignore)

		case action == "Back to Caller":
			if len(state.Stack) > 0 {
				state.Current = state.Stack[len(state.Stack)-1]
				state.Stack = state.Stack[:len(state.Stack)-1]
			}

		case strings.HasPrefix(action, "List External Calls"):
			fmt.Fprintln(cmd.OutOrStdout(), "\nExternal Calls (not in this package):")
			for _, c := range externalCalls {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s()\n", c)
			}
			var ignore string
			askOneFunc(&survey.Input{Message: "Press Enter to continue..."}, &ignore)

		case action == "Quit":
			fmt.Fprintln(cmd.OutOrStdout(), "Tour ended.")
			return nil
		}
	}
}

func explainFunction(cmd *cobra.Command, node *FunctionNode) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(`Explain the following Go function concisely.
Describe its purpose, inputs, outputs, and what it does.

Function Name: %s
Code:
%s`, node.Name, node.Body)

	fmt.Fprintln(cmd.OutOrStdout(), "\n🤖 Asking Agent...")
	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Println()
	return err
}

func uniqueStrings(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)

	for _, val := range input {
		if !m[val] {
			m[val] = true
			u = append(u, val)
		}
	}
	return u
}
