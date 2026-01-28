package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	knowledgeLimit int
	knowledgeFocus string
	knowledgeDiff  bool
)

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge",
	Short: "Manage project knowledge and rules (AGENTS.md)",
	Long:  `Manage project-specific rules, conventions, and context stored in AGENTS.md.`,
}

var knowledgeAddCmd = &cobra.Command{
	Use:   "add [rule]",
	Short: "Add a new rule to AGENTS.md",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rule := strings.Join(args, " ")
		return appendRule(rule)
	},
}

var knowledgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all rules in AGENTS.md",
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := readRules()
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No AGENTS.md found.")
				return nil
			}
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	},
}

var knowledgeCheckCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if code violates rules in AGENTS.md",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rules, err := readRules()
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No AGENTS.md rules to check against.")
				return nil
			}
			return err
		}

		var content string
		if knowledgeDiff {
			content, err = getGitDiff()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Checking current git changes against rules...")
		} else if len(args) > 0 {
			b, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			content = string(b)
		} else {
			// Check Stdin
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				content = string(b)
			} else {
				return fmt.Errorf("please provide a file to check or use --diff")
			}
		}

		return checkRules(cmd, rules, content)
	},
}

var knowledgeLearnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Analyze codebase to infer and add rules",
	Long: `Scans the codebase (focused on specific directories if provided) and uses AI to infer coding conventions,
architectural patterns, and common pitfalls. These are then reviewed and added to AGENTS.md.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return learnRules(cmd)
	},
}

func init() {
	rootCmd.AddCommand(knowledgeCmd)
	knowledgeCmd.AddCommand(knowledgeAddCmd)
	knowledgeCmd.AddCommand(knowledgeListCmd)
	knowledgeCmd.AddCommand(knowledgeCheckCmd)
	knowledgeCmd.AddCommand(knowledgeLearnCmd)

	knowledgeLearnCmd.Flags().IntVarP(&knowledgeLimit, "limit", "l", 5, "Maximum number of rules to learn")
	knowledgeLearnCmd.Flags().StringVarP(&knowledgeFocus, "focus", "f", ".", "Focus analysis on a specific path")

	knowledgeCheckCmd.Flags().BoolVar(&knowledgeDiff, "diff", false, "Check current git changes against rules")
}

func getAgentsMdPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, "AGENTS.md"), nil
}

func readRules() (string, error) {
	path, err := getAgentsMdPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func appendRule(rule string) error {
	path, err := getAgentsMdPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Ensure newline before appending if file is not empty
	stat, _ := f.Stat()
	if stat.Size() > 0 {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}

	// Format as a bullet point if not already
	if !strings.HasPrefix(rule, "-") && !strings.HasPrefix(rule, "*") {
		rule = "- " + rule
	}

	if _, err := f.WriteString(rule + "\n"); err != nil {
		return err
	}

	fmt.Printf("Added rule to %s\n", path)
	return nil
}

func checkRules(cmd *cobra.Command, rules string, content string) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-knowledge-check")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a code compliance officer.
Check the following code against the Project Rules.

<project_rules>
%s
</project_rules>

<code_to_check>
%s
</code_to_check>

Identify any violations.
If violations are found, list them with line numbers (if possible) and explanations.
If no violations are found, reply with exactly "PASS".
`, rules, content)

	fmt.Fprintln(cmd.OutOrStdout(), "Checking code against rules...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	if strings.Contains(strings.ToUpper(resp), "PASS") && len(resp) < 20 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ PASS: Code adheres to AGENTS.md rules.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "❌ VIOLATIONS FOUND:")
	fmt.Fprintln(cmd.OutOrStdout(), resp)
	return fmt.Errorf("compliance check failed")
}

func learnRules(cmd *cobra.Command) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Generate Context
	opts := ContextOptions{
		Roots:   []string{knowledgeFocus},
		MaxSize: 100 * 1024,
		Tree:    true,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Scanning codebase...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate context: %w", err)
	}

	// 2. Ask Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-knowledge-learn")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze the following codebase to infer %d coding conventions, architectural patterns, or rules.
Focus on:
- Error handling patterns
- Naming conventions
- Library usage (e.g., logging, config)
- Architectural boundaries

Return ONLY the inferred rules as a list of bullet points. Do not include introductory text.

Codebase:
%s`, knowledgeLimit, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Analyzing patterns (this may take a while)...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	// 3. Output
	cleaned := utils.CleanCodeBlock(resp) // Reuse to strip md blocks if any

	fmt.Fprintln(cmd.OutOrStdout(), "\nInferred Rules:")
	fmt.Fprintln(cmd.OutOrStdout(), cleaned)
	fmt.Fprintln(cmd.OutOrStdout(), "\n------------------------------------------------")

	// Interactive confirm
	// We can use a simple prompt here
	fmt.Print("Append these rules to AGENTS.md? [y/N]: ")
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) == "y" {
		path, _ := getAgentsMdPath()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		// Ensure separation
		f.WriteString("\n")
		f.WriteString(cleaned)
		f.WriteString("\n")
		fmt.Printf("Rules appended to %s\n", path)
	} else {
		fmt.Println("Aborted.")
	}

	return nil
}
