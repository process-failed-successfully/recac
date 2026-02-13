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
	instrumentType    string
	instrumentInPlace bool
	instrumentDiff    bool
)

func NewInstrumentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instrument [file]",
		Short: "Automatically add OpenTelemetry or logging instrumentation to code",
		Long:  `Analyzes the provided source file and injects OpenTelemetry tracing spans or logging calls.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var content []byte
			var filePath string
			var err error

			if len(args) > 0 {
				filePath = args[0]
				content, err = os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
			} else {
				// Stdin logic
				in := cmd.InOrStdin()
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

			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-instrument")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			// Construct the prompt based on instrumentType
			var instruction string
			switch instrumentType {
			case "otel":
				instruction = `Add OpenTelemetry tracing to the following code.
Inject 'tracer.Start' spans at the beginning of significant functions.
Ensure context is propagated correctly.
Record errors using 'span.RecordError(err)'.
Add necessary imports (e.g., "go.opentelemetry.io/otel").`
			case "log":
				instruction = `Add structured logging to the following code.
Log entry and exit of significant functions.
Log errors with context.
Use standard logging library or a common structured logger (e.g., slog, zap).`
			default:
				return fmt.Errorf("unknown instrumentation type: %s", instrumentType)
			}

			prompt := fmt.Sprintf(`%s
Do not change the business logic.
IMPORTANT: Return ONLY the full instrumented code. Do not include any explanations, markdown formatting (like '''go ... '''), or conversational text.

Code to instrument:
'''
%s
'''`, instruction, string(content))

			fmt.Fprintln(cmd.ErrOrStderr(), "Analyzing and instrumenting code...")

			// We need the full response to process it
			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed to instrument code: %w", err)
			}

			instrumentedCode := utils.CleanCodeBlock(resp)

			if instrumentDiff {
				diff, err := utils.GenerateDiff(filePath, string(content), instrumentedCode)
				if err != nil {
					return fmt.Errorf("failed to generate diff: %w", err)
				}
				fmt.Fprint(cmd.OutOrStdout(), diff)
				return nil
			}

			if instrumentInPlace {
				if filePath == "" {
					return errors.New("cannot use --in-place with stdin input")
				}

				// Preserve file permissions
				info, err := os.Stat(filePath)
				if err != nil {
					return fmt.Errorf("failed to stat file: %w", err)
				}
				mode := info.Mode()

				if err := os.WriteFile(filePath, []byte(instrumentedCode), mode); err != nil {
					return fmt.Errorf("failed to write back to file: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Successfully updated %s\n", filePath)
				return nil
			}

			// Default: print instrumented code to stdout
			fmt.Fprint(cmd.OutOrStdout(), instrumentedCode)

			return nil
		},
	}

	cmd.Flags().StringVar(&instrumentType, "type", "otel", "Instrumentation type: 'otel' (OpenTelemetry) or 'log'")
	cmd.Flags().BoolVarP(&instrumentInPlace, "in-place", "i", false, "Modify the file in place")
	cmd.Flags().BoolVar(&instrumentDiff, "diff", false, "Show diff between original and instrumented code")

	return cmd
}

var instrumentCmd = NewInstrumentCmd()

func init() {
	rootCmd.AddCommand(instrumentCmd)
}
