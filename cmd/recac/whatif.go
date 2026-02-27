package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	whatIfFocus string
)

var whatIfCmd = &cobra.Command{
	Use:   "whatif [change description]",
	Short: "Analyze the impact of a hypothetical change using AI",
	Long: `Simulates a hypothetical change to the codebase and predicts its impact using AI.
It identifies affected components, potential breaking changes, and suggests migration steps.

Example:
  recac whatif "Change User.ID from int to uuid.UUID"
  recac whatif "Refactor the logging interface to support structured logging" --focus pkg/logger`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWhatIf,
}

func init() {
	rootCmd.AddCommand(whatIfCmd)
	whatIfCmd.Flags().StringVarP(&whatIfFocus, "focus", "f", ".", "Limit analysis to a specific path")
}

func runWhatIf(cmd *cobra.Command, args []string) error {
	changeDescription := strings.Join(args, " ")
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather Context
	// We use the same logic as 'ask' or 'suggest' to generate a codebase dump.
	// But we might want to be smarter and filter by keywords in the description?
	// For now, let's use the provided focus path or root.

	roots := []string{whatIfFocus}
	if whatIfFocus == "." {
		roots = []string{"."}
	} else {
		if _, err := os.Stat(whatIfFocus); err != nil {
			return fmt.Errorf("focus path does not exist: %w", err)
		}
	}

	opts := ContextOptions{
		Roots:     roots,
		MaxSize:   100 * 1024, // 100KB limit
		Tree:      true,
		NoContent: false,
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Analyzing codebase context (focus: %s)...\n", whatIfFocus)
	codebaseContext, err := generateContextFunc(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-whatif")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are a Senior Software Architect.
Perform an impact analysis for the following HYPOTHETICAL change:
"%s"

Analyze the provided codebase context and identify:
1. **Affected Components**: Which files, structs, or functions will need modification?
2. **Breaking Changes**: Will this break APIs, database schemas, or downstream dependencies?
3. **Migration Steps**: A high-level plan to execute this change safely.
4. **Estimated Effort**: Low/Medium/High complexity estimate.

Codebase Context:
%s

Output the report in Markdown format.
`, changeDescription, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Simulating change impact...")

	// 4. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to analyze: %w", err)
	}

	// 5. Output
	fmt.Fprintln(cmd.OutOrStdout(), "\n"+resp)

	return nil
}
