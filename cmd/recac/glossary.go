package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	glossaryOutput string
	glossaryLimit  int
	glossaryFocus  string
)

var glossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "Extract a domain glossary from the codebase",
	Long: `Analyzes the codebase to extract and define domain-specific terms.
It generates a glossary including terms, definitions, and where they are found in the code.
This is useful for onboarding and understanding the Ubiquitous Language of the project.`,
	RunE: runGlossary,
}

func init() {
	rootCmd.AddCommand(glossaryCmd)
	glossaryCmd.Flags().StringVarP(&glossaryOutput, "output", "o", "", "File to write the glossary to (Markdown format)")
	glossaryCmd.Flags().IntVarP(&glossaryLimit, "limit", "l", 50, "Maximum number of terms to extract")
	glossaryCmd.Flags().StringVarP(&glossaryFocus, "focus", "f", ".", "Focus analysis on a specific path")
}

type GlossaryTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
	Context    string `json:"context"` // e.g. "Order.go"
}

func runGlossary(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Generate Context
	opts := ContextOptions{
		Roots:     []string{glossaryFocus},
		MaxSize:   100 * 1024,
		Tree:      true,
		NoContent: false,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning codebase for domain terms...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-glossary")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`You are a Domain-Driven Design (DDD) expert.
Your goal is to extract a glossary of the "Ubiquitous Language" from the codebase.
Identify key domain terms (Structs, Interfaces, Constants, Business Concepts) and define them based on their usage.

Focus on:
- Business entities (e.g., "Session", "Agent", "Orchestrator")
- Key processes (e.g., "Replay", "Bisect")
- Specific technical terms used in a unique way.

Limit to the top %d most important terms.

Return the result as a raw JSON list of objects with the following structure:
[
  {
    "term": "Term Name",
    "definition": "A concise definition inferred from the code.",
    "context": "File or package where it is most prominent (e.g. 'internal/runner/session.go')"
  }
]

Do not wrap the JSON in markdown code blocks. Just return the raw JSON string.

CODEBASE CONTEXT:
%s`, glossaryLimit, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Consulting AI agent (this may take a moment)...")

	// 4. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate glossary: %w", err)
	}

	// 5. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var terms []GlossaryTerm
	if err := json.Unmarshal([]byte(jsonStr), &terms); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "Raw response:")
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	if len(terms) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No domain terms found.")
		return nil
	}

	// 6. Format Output
	var sb strings.Builder
	sb.WriteString("# Domain Glossary\n\n")
	sb.WriteString("| Term | Definition | Context |\n")
	sb.WriteString("|---|---|---|\n")

	for _, t := range terms {
		sb.WriteString(fmt.Sprintf("| **%s** | %s | `%s` |\n", t.Term, t.Definition, t.Context))
	}

	output := sb.String()

	// 7. Write to File or Stdout
	if glossaryOutput != "" {
		// Ensure output path is relative to CWD if not absolute
		outPath := glossaryOutput
		if !filepath.IsAbs(outPath) {
			outPath = filepath.Join(cwd, outPath)
		}
		if err := writeFileFunc(outPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Glossary written to %s\n", glossaryOutput)
	} else {
		// Pretty print to stdout using tabwriter for better alignment in terminal
		fmt.Fprintln(cmd.OutOrStdout(), "\nDomain Glossary:")
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "TERM\tDEFINITION\tCONTEXT")
		for _, t := range terms {
			// Truncate definition if too long for terminal
			def := t.Definition
			if len(def) > 60 {
				def = def[:57] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.Term, def, t.Context)
		}
		w.Flush()
	}

	return nil
}
