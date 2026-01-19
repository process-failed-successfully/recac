package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const agentsFile = "AGENTS.md"

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge",
	Short: "Manage and enforce knowledge rules in AGENTS.md",
	Long:  `Manage the AGENTS.md file which contains rules and guidelines for the AI agent, and enforce these rules against your code.`,
}

var knowledgeAddCmd = &cobra.Command{
	Use:   "add [rule]",
	Short: "Add a new rule to AGENTS.md",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rule := strings.Join(args, " ")
		return appendRule(rule, cmd)
	},
}

var knowledgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List rules from AGENTS.md",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listRules(cmd)
	},
}

var knowledgeCheckCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check code against rules in AGENTS.md",
	Long:  `Checks the specified file or the current git diff against the rules defined in AGENTS.md using the AI agent.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Read AGENTS.md
		rules, err := os.ReadFile(agentsFile)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%s not found. Run 'recac knowledge add' to create one", agentsFile)
			}
			return fmt.Errorf("failed to read %s: %w", agentsFile, err)
		}

		if len(rules) == 0 {
			return fmt.Errorf("%s is empty", agentsFile)
		}

		// 2. Get Code Content
		var content string
		var sourceDescription string

		if len(args) > 0 {
			filePath := args[0]
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}
			content = string(fileContent)
			sourceDescription = fmt.Sprintf("file: %s", filePath)
		} else {
			// Git diff logic
			diffCmd := exec.Command("git", "diff", "HEAD")
			var out bytes.Buffer
			diffCmd.Stdout = &out
			// We ignore stderr for now
			if err := diffCmd.Run(); err != nil {
				// Fallback for fresh repos or no HEAD
				diffCmd = exec.Command("git", "diff")
				out.Reset()
				diffCmd.Stdout = &out
				if err := diffCmd.Run(); err != nil {
					return fmt.Errorf("failed to get git diff: %w", err)
				}
			}
			content = out.String()
			if len(content) == 0 {
				return errors.New("no changes detected to check")
			}
			sourceDescription = "current git changes"
		}

		// 3. Consult Agent
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-knowledge")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		prompt := fmt.Sprintf(`You are a code compliance officer.
Verify if the following code adheres to the project rules.

<project_rules>
%s
</project_rules>

<code_to_check description="%s">
%s
</code_to_check>

INSTRUCTIONS:
1. Analyze the code against the rules.
2. If there are violations, list them clearly with line numbers if possible.
3. If the code is compliant, strictly output: "✅ Compliant".
4. Be concise.`, string(rules), sourceDescription, content)

		fmt.Fprintf(cmd.OutOrStdout(), "Checking %s against %s...\n\n", sourceDescription, agentsFile)

		_, err = ag.SendStream(ctx, prompt, func(chunk string) {
			fmt.Fprint(cmd.OutOrStdout(), chunk)
		})
		fmt.Fprintln(cmd.OutOrStdout(), "")

		return err
	},
}

func init() {
	rootCmd.AddCommand(knowledgeCmd)
	knowledgeCmd.AddCommand(knowledgeAddCmd)
	knowledgeCmd.AddCommand(knowledgeListCmd)
	knowledgeCmd.AddCommand(knowledgeCheckCmd)
}

func ensureAgentsFile() error {
	if _, err := os.Stat(agentsFile); os.IsNotExist(err) {
		return os.WriteFile(agentsFile, []byte("# AGENTS.md - Project Rules\n\n"), 0644)
	}
	return nil
}

func appendRule(rule string, cmd *cobra.Command) error {
	if err := ensureAgentsFile(); err != nil {
		return err
	}

	f, err := os.OpenFile(agentsFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Format as list item if not already
	toWrite := rule
	if !strings.HasPrefix(rule, "-") && !strings.HasPrefix(rule, "*") && !strings.HasPrefix(rule, "#") {
		toWrite = "- " + rule
	}

	if _, err := f.WriteString(toWrite + "\n"); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added rule: %s\n", rule)
	return nil
}

func listRules(cmd *cobra.Command) error {
	content, err := os.ReadFile(agentsFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s does not exist.\n", agentsFile)
			return nil
		}
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Contents of %s:\n\n", agentsFile)
	fmt.Fprint(cmd.OutOrStdout(), string(content))
	return nil
}
