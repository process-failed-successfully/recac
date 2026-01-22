package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	dataSchema string
	dataDesc   string
	dataCount  int
	dataFormat string
	dataOut    string
)

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Generate realistic mock data",
	Long: `Generate realistic mock data for testing, seeding, or demos using AI.
You can specify a structured schema or a natural language description.

Examples:
  recac data --schema "user(id:uuid, name:string, email:email)" --count 10 --format json
  recac data --desc "E-commerce transactions with items and totals" --count 5 --format csv
  recac data --desc "SQL insert statements for a 'products' table" --format sql
`,
	RunE: runData,
}

func init() {
	rootCmd.AddCommand(dataCmd)

	dataCmd.Flags().StringVarP(&dataSchema, "schema", "s", "", "Structure definition (e.g. 'field:type, field2:type')")
	dataCmd.Flags().StringVarP(&dataDesc, "desc", "d", "", "Natural language description of the data")
	dataCmd.Flags().IntVarP(&dataCount, "count", "c", 5, "Number of items to generate")
	dataCmd.Flags().StringVarP(&dataFormat, "format", "f", "json", "Output format (json, csv, sql, xml, yaml)")
	dataCmd.Flags().StringVarP(&dataOut, "out", "o", "", "Output file path (default: stdout)")
}

func runData(cmd *cobra.Command, args []string) error {
	// validation
	if dataSchema == "" && dataDesc == "" {
		// Try to read from args if flags not set
		if len(args) > 0 {
			dataDesc = strings.Join(args, " ")
		} else {
			return fmt.Errorf("please provide a --schema or --desc (or argument)")
		}
	}

	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-data")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Construct Prompt
	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("Generate %d items of realistic mock data.\n", dataCount))
	promptBuilder.WriteString(fmt.Sprintf("Format: %s\n", dataFormat))

	if dataSchema != "" {
		promptBuilder.WriteString(fmt.Sprintf("Schema: %s\n", dataSchema))
	}
	if dataDesc != "" {
		promptBuilder.WriteString(fmt.Sprintf("Description: %s\n", dataDesc))
	}

	promptBuilder.WriteString("\nRequirements:\n")
	promptBuilder.WriteString("- The data must be realistic (e.g., valid emails, consistent dates).\n")
	promptBuilder.WriteString("- Return ONLY the raw data. Do not include markdown code blocks or explanations.\n")

	if dataFormat == "json" {
		promptBuilder.WriteString("- The output must be a valid JSON array.\n")
	} else if dataFormat == "csv" {
		promptBuilder.WriteString("- Include a header row.\n")
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Generating data...")

	resp, err := ag.Send(ctx, promptBuilder.String())
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Clean Output
	output := utils.CleanCodeBlock(resp)

	// Basic Validation for JSON
	if strings.ToLower(dataFormat) == "json" {
		var js interface{}
		if err := json.Unmarshal([]byte(output), &js); err != nil {
			// Try to fix it? Or just warn?
			// Sometimes agents wrap in ```json ... ``` which CleanCodeBlock handles.
			// If it still fails, it might be partial.
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Output might not be valid JSON: %v\n", err)
		}
	}

	// Output
	if dataOut != "" {
		if err := os.WriteFile(dataOut, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Data written to %s\n", dataOut)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	return nil
}
