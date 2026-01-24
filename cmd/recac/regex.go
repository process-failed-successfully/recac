package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	regexExplain string
	regexMatch   []string
	regexNoMatch []string
	regexLang    string
)

var regexCmd = &cobra.Command{
	Use:   "regex [description]",
	Short: "Generate, explain, and verify regular expressions",
	Long: `Generate regular expressions from natural language descriptions,
explain existing regexes, and verify them against test cases.

Examples:
  recac regex "valid email address"
  recac regex "date in format YYYY-MM-DD" --match "2023-01-01" --no-match "01-01-2023"
  recac regex --explain "^[a-z]+$"`,
	RunE: runRegex,
}

func init() {
	rootCmd.AddCommand(regexCmd)
	regexCmd.Flags().StringVarP(&regexExplain, "explain", "e", "", "Regex to explain")
	regexCmd.Flags().StringSliceVarP(&regexMatch, "match", "m", nil, "Examples that SHOULD match")
	regexCmd.Flags().StringSliceVarP(&regexNoMatch, "no-match", "n", nil, "Examples that SHOULD NOT match")
	regexCmd.Flags().StringVarP(&regexLang, "lang", "l", "go", "Target language (go, js, python, pcre)")
}

func runRegex(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-regex")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Mode 1: Explain
	if regexExplain != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing regex...")
		prompt := fmt.Sprintf("Explain the following regular expression in simple terms:\n```\n%s\n```\nProvide a breakdown of what each part does.", regexExplain)
		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	// Mode 2: Generate
	if len(args) == 0 {
		return fmt.Errorf("please provide a description or use --explain")
	}
	description := strings.Join(args, " ")

	fmt.Fprintf(cmd.OutOrStdout(), "Generating regex for: %s\n", description)

	// Construct initial prompt
	prompt := fmt.Sprintf(`You are a regular expression expert.
Generate a regular expression for the following requirement:
"%s"

Target Language: %s

Requirements:
1. Return ONLY the raw regular expression. Do not wrap it in markdown code blocks.
2. Do not add explanations.
`, description, regexLang)

	if len(regexMatch) > 0 {
		prompt += "\nIt MUST match these examples:\n"
		for _, m := range regexMatch {
			prompt += fmt.Sprintf("- %s\n", m)
		}
	}
	if len(regexNoMatch) > 0 {
		prompt += "\nIt MUST NOT match these examples:\n"
		for _, m := range regexNoMatch {
			prompt += fmt.Sprintf("- %s\n", m)
		}
	}

	// Retry loop
	maxRetries := 3
	lastRegex := ""

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "🔄 Retrying (%d/%d)...\n", i+1, maxRetries)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "🤖 Consulting AI...")
		}

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		// Clean up response
		candidate := utils.CleanCodeBlock(resp)
		candidate = strings.TrimSpace(candidate)
		// Sometimes agents put it in /.../
		if strings.HasPrefix(candidate, "/") && strings.HasSuffix(candidate, "/") && len(candidate) > 2 {
			candidate = candidate[1 : len(candidate)-1]
		}
		lastRegex = candidate

		// Verify locally if possible
		// Go's regex engine is RE2. If user requested python/js, complex lookarounds won't work.
		// We attempt verification. If it fails to compile, we warn but don't fail the command.
		// If it compiles but fails tests, we recurse.

		re, compileErr := regexp.Compile(candidate)
		if compileErr != nil {
			if regexLang == "go" {
				// If asking for Go regex, it MUST compile in Go
				prompt += fmt.Sprintf("\n\nThe regex you provided '%s' failed to compile in Go: %v\nPlease fix it.", candidate, compileErr)
				continue
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Regex could not be verified locally (Go engine limitation): %v\n", compileErr)
				// We cannot verify logic if we can't compile. Assume best effort.
				break
			}
		}

		// Verification
		failed := false
		var failures []string

		for _, m := range regexMatch {
			if !re.MatchString(m) {
				failed = true
				msg := fmt.Sprintf("FAILED to match '%s'", m)
				failures = append(failures, msg)
				fmt.Fprintln(cmd.OutOrStdout(), "❌ "+msg)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✅ Matched '%s'\n", m)
			}
		}

		for _, m := range regexNoMatch {
			if re.MatchString(m) {
				failed = true
				msg := fmt.Sprintf("INCORRECTLY matched '%s'", m)
				failures = append(failures, msg)
				fmt.Fprintln(cmd.OutOrStdout(), "❌ "+msg)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✅ Correctly rejected '%s'\n", m)
			}
		}

		if !failed {
			fmt.Fprintln(cmd.OutOrStdout(), "\n🎉 Regex verified successfully!")
			break
		}

		// Feedback for next iteration
		prompt += fmt.Sprintf("\n\nThe regex you provided '%s' failed the following tests:\n%s\nPlease correct it.", candidate, strings.Join(failures, "\n"))
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nResult:")
	fmt.Fprintln(cmd.OutOrStdout(), lastRegex)

	return nil
}
