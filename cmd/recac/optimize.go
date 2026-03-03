package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	optInPlace bool
	optDiff    bool
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize [file]",
	Short: "Optimize source code for CPU and memory usage efficiency",
	Long: `Analyze and rewrite source code to improve CPU and memory usage efficiency.
Acts as the 'Bolt' persona.
Reads from the specified file, or from stdin if no file is provided.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOptimizeCmd,
}

func init() {
	optimizeCmd.Flags().BoolVarP(&optInPlace, "in-place", "i", false, "Rewrite the file directly")
	optimizeCmd.Flags().BoolVarP(&optDiff, "diff", "d", false, "Output a unified diff")
	rootCmd.AddCommand(optimizeCmd)
}

func runOptimizeCmd(cmd *cobra.Command, args []string) error {
	var inputContent []byte
	var err error
	var filePath string

	// Handle input from file or stdin
	if len(args) == 1 {
		filePath = args[0]
		inputContent, err = readFileFunc(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
	} else {
		// Read from stdin
		stdinFile, ok := cmd.InOrStdin().(*os.File)
		if ok {
			stat, err := stdinFile.Stat()
			if err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
				// In a terminal, prompt user that it's waiting for stdin
				fmt.Fprintln(cmd.ErrOrStderr(), "Reading from standard input... (Press Ctrl+D to finish)")
			}
		}

		inputContent, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		if len(inputContent) == 0 {
			return fmt.Errorf("no input provided")
		}
	}

	if optInPlace && filePath == "" {
		return fmt.Errorf("--in-place flag requires a file argument")
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	agent, err := agentClientFactory(ctx, provider, model, cwd, "recac-bolt")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 'Bolt' Persona prompt to optimize CPU and memory
	prompt := fmt.Sprintf(`You are 'Bolt', a performance-focused AI agent.
Analyze the following source code and rewrite it to improve CPU and memory usage efficiency.
Return ONLY the optimized source code, without any markdown formatting, explanations, or code blocks.

<source_code>
%s
</source_code>`, string(inputContent))

	optimizedContentStr, err := agent.SendStream(ctx, prompt, func(chunk string) {
		// We don't stream to stdout here to keep the output clean for files/diffs,
		// or if we do, it might mess up --in-place or diff output.
		// For now, we capture the full response.
	})
	if err != nil {
		return fmt.Errorf("agent failed to optimize code: %w", err)
	}

	optimizedContentStr = strings.TrimSpace(optimizedContentStr)
	// Some models might still return markdown code blocks despite instructions. Clean them up if they exist.
	if strings.HasPrefix(optimizedContentStr, "```") {
		lines := strings.Split(optimizedContentStr, "\n")
		if len(lines) > 1 {
			if strings.HasPrefix(lines[0], "```") {
				lines = lines[1:]
			}
			if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			optimizedContentStr = strings.Join(lines, "\n")
		}
	}

	optimizedBytes := []byte(optimizedContentStr)

	if optInPlace {
		err = writeFileFunc(filePath, optimizedBytes, 0644)
		if err != nil {
			return fmt.Errorf("failed to write optimized file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Successfully optimized %s in-place.\n", filePath)
		return nil
	}

	if optDiff {
		diff, err := generateDiff(filePath, string(inputContent), string(optimizedBytes))
		if err != nil {
			return fmt.Errorf("failed to generate diff: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), diff)
		return nil
	}

	// If no flags, just output the optimized code
	fmt.Fprintln(cmd.OutOrStdout(), string(optimizedBytes))
	return nil
}

func generateDiff(filePath, original, optimized string) (string, error) {
	if filePath == "" {
		filePath = "stdin"
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(original),
		B:        difflib.SplitLines(optimized),
		FromFile: filePath + " (original)",
		ToFile:   filePath + " (optimized)",
		Context:  3,
	}

	return difflib.GetUnifiedDiffString(diff)
}
