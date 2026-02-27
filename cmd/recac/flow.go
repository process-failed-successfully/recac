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

func NewFlowCmd() *cobra.Command {
	cmd := &cobra.Command{
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

			// We can't rely on global vars if we want instance-isolation,
			// but Cobra flags bind to vars.
			// Ideally we bind flags to local vars of the struct or closure.
			// But for now, sticking to globals is easier if we reset them,
			// OR we can query flags directly.

			d, _ := cmd.Flags().GetString("dir")
			if d != "" {
				searchDir = d
			}
			f, _ := cmd.Flags().GetString("file")
			o, _ := cmd.Flags().GetString("output")

			sourceCode, err := findFunctionSource(searchDir, f, targetFunc)
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
			if o != "" {
				if err := os.WriteFile(o, []byte(diagram), 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Flowchart saved to %s\n", o)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), diagram)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flowOutput, "output", "o", "", "Output file path")
	cmd.Flags().StringVar(&flowDir, "dir", ".", "Directory to search (default: current)")
	cmd.Flags().StringVarP(&flowFile, "file", "f", "", "Specific file to analyze (optional)")

	return cmd
}

var flowCmd = NewFlowCmd()

func init() {
	rootCmd.AddCommand(flowCmd)
}

func findFunctionSource(root, specificFile, funcName string) (string, error) {
	fset := token.NewFileSet()
	var foundSource string

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
