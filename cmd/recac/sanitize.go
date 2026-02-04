package main

import (
	"fmt"
	"io"
	"os"
	"recac/internal/security"

	"github.com/spf13/cobra"
)

var (
	sanitizeOutput string
)

var sanitizeCmd = &cobra.Command{
	Use:   "sanitize [file]",
	Short: "Redact PII from files or input",
	Long: `Redacts Personally Identifiable Information (PII) like emails, phone numbers,
IP addresses, and credit card numbers from the provided file or stdin.
Useful for sharing logs or code snippets safely.`,
	RunE: runSanitize,
}

func init() {
	rootCmd.AddCommand(sanitizeCmd)
	sanitizeCmd.Flags().StringVarP(&sanitizeOutput, "output", "o", "", "Output file path (default: stdout)")
}

func runSanitize(cmd *cobra.Command, args []string) error {
	var content []byte
	var err error

	if len(args) > 0 {
		content, err = os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
	} else {
		// Check for stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			content, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
		} else {
			return fmt.Errorf("please provide a file or pipe content via stdin")
		}
	}

	s := security.NewSanitizer()
	sanitized := s.Sanitize(string(content))

	if sanitizeOutput != "" {
		if err := os.WriteFile(sanitizeOutput, []byte(sanitized), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Redacted content written to %s\n", sanitizeOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), sanitized)
	}

	return nil
}
