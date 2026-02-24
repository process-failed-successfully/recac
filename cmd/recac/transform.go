package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	transformOutput string
)

var transformCmd = &cobra.Command{
	Use:   "transform [instruction] [file]",
	Short: "Transform data using natural language instructions",
	Long: `A generic data transformation tool powered by AI.
Reads input from a file or stdin and transforms it based on your natural language instruction.
Useful for converting formats (JSON to YAML), extracting data (emails from text), or filtering logs.

Examples:
  recac transform "convert to yaml" data.json
  cat logs.txt | recac transform "extract error messages as JSON list"
  recac transform "filter rows where age > 21" users.csv --output adults.csv`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTransform,
}

func init() {
	rootCmd.AddCommand(transformCmd)
	transformCmd.Flags().StringVarP(&transformOutput, "output", "o", "", "Output file path (default: stdout)")
}

func runTransform(cmd *cobra.Command, args []string) error {
	instruction := args[0]
	var content []byte
	var err error

	// Determine input source
	if len(args) == 2 {
		// Read from file
		filePath := args[1]
		content, err = os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
	} else {
		// Read from stdin
		in := cmd.InOrStdin()
		// Check if input is available if needed, but for pipes we just read.
		// If interactive user provides no input, it might hang.
		// We can check if it's a terminal and warn?
		if f, ok := in.(*os.File); ok && f == os.Stdin {
			stat, _ := f.Stat()
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				return errors.New("please provide a file path or pipe content via stdin")
			}
		}

		content, err = io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("failed to read from input: %w", err)
		}
	}

	if len(content) == 0 {
		return errors.New("input is empty")
	}

	// Limit input size to prevent context overflow (e.g. 100KB warning)
	if len(content) > 100*1024 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Input is large (%d bytes). AI processing might be slow or truncated.\n", len(content))
	}

	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory for dependency injection
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-transform")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Construct Prompt
	// We want raw data output.
	prompt := fmt.Sprintf(`You are a data transformation engine.
Process the following input data according to the instruction.

Instruction: "%s"

Input Data:
'''
%s
'''

IMPORTANT requirements:
1. Return ONLY the transformed data.
2. Do NOT include any explanations, introductory text, or markdown formatting (like '''json ... ''').
3. If the output is structured (JSON, YAML, SQL, CSV), ensure it is valid syntax.
`, instruction, string(content))

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Transforming data...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Clean Output (remove markdown blocks if agent adds them despite instructions)
	// We use CleanCodeBlock which handles ``` blocks.
	output := utils.CleanCodeBlock(resp)

	// Write Output
	if transformOutput != "" {
		if err := os.WriteFile(transformOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Output written to %s\n", transformOutput)
	} else {
		// Write to stdout
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	return nil
}
