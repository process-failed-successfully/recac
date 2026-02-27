package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"recac/internal/utils"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flowOutput string
	flowDir    string
	flowFile   string
)

var flowCmd = &cobra.Command{
	Use:   "flow [function]",
	Short: "Generate a Mermaid flowchart for a function",
	Long:  `Analyzes the Go code to extract a function's logic and uses AI to generate a Mermaid flowchart.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetFunc := args[0]
		ctx := cmd.Context()
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		// 1. Locate Function Source
		searchDir := flowDir
		if searchDir == "" {
			searchDir = cwd
		}

		// fmt.Fprintf(cmd.ErrOrStderr(), "DEBUG: searchDir=%s target=%s\n", searchDir, targetFunc)

		if flowFile != "" {
			// If file specified, just check that file (or dir containing it if path is relative)
			// But to reuse logic, we can just walk.
			// Actually, let's implement a specific finder.
		}

		sourceCode, err := findFunctionSource(searchDir, flowFile, targetFunc)
		if err != nil {
			return fmt.Errorf("failed to find function '%s' in %s: %w", targetFunc, searchDir, err)
		}

		// 2. Generate Flowchart with AI
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-flow")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		prompt := fmt.Sprintf(`You are an expert in code visualization.
Generate a Mermaid flowchart (graph TD) that represents the logic of the following Go function.
Use standard flowchart shapes (diamonds for decisions, rectangles for processes).
Label the decision branches (Yes/No, True/False).
Keep it clean and readable.

Function Code:
'''go
%s
'''

Return ONLY the Mermaid code. Do not include markdown blocks.`, sourceCode)

		fmt.Fprintf(cmd.ErrOrStderr(), "🤖 Generating flowchart for '%s'...\n", targetFunc)
		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		diagram := utils.CleanCodeBlock(resp)

		// 3. Output
		if flowOutput != "" {
			if err := os.WriteFile(flowOutput, []byte(diagram), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Flowchart saved to %s\n", flowOutput)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), diagram)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(flowCmd)
	flowCmd.Flags().StringVarP(&flowOutput, "output", "o", "", "Output file path")
	flowCmd.Flags().StringVar(&flowDir, "dir", ".", "Directory to search (default: current)")
	flowCmd.Flags().StringVarP(&flowFile, "file", "f", "", "Specific file to analyze (optional)")
}

func findFunctionSource(root, specificFile, funcName string) (string, error) {
	fset := token.NewFileSet()
	var foundSource string
    // fmt.Fprintf(os.Stderr, "DEBUG: Walking %s for %s\n", root, funcName)

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if (strings.HasPrefix(info.Name(), ".") && info.Name() != ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// If specific file provided, skip others
		if specificFile != "" {
			absPath, _ := filepath.Abs(path)
			absSpecific, _ := filepath.Abs(specificFile)
			if absPath != absSpecific {
				return nil
			}
		}

		// Parse
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return nil
		}

		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if fn.Name.Name == funcName {
					// Extract source
					start := fset.Position(fn.Pos()).Offset
					end := fset.Position(fn.End()).Offset
					foundSource = string(content[start:end])
					return filepath.SkipAll // Stop searching
				}
			}
		}
		return nil
	}

	err := filepath.Walk(root, walkFunc)
	if err == filepath.SkipAll {
		err = nil
	}

	if err != nil {
		return "", err
	}

	if foundSource == "" {
		return "", fmt.Errorf("function not found")
	}

	return foundSource, nil
}
