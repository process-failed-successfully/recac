package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	threatJSON   bool
	threatOutput string
	threatFile   string
)

var threatCmd = &cobra.Command{
	Use:   "threat [file]",
	Short: "Perform STRIDE threat modeling using AI",
	Long: `Analyzes the system architecture or specification to identify potential security threats
using the STRIDE model (Spoofing, Tampering, Repudiation, Information Disclosure, Denial of Service, Elevation of Privilege).

It accepts an architecture file (YAML) or a text specification.
If no file is provided, it looks for:
1. .recac/architecture/architecture.yaml
2. app_spec.txt`,
	RunE: runThreat,
}

func init() {
	rootCmd.AddCommand(threatCmd)
	threatCmd.Flags().BoolVar(&threatJSON, "json", false, "Output results as JSON")
	threatCmd.Flags().StringVarP(&threatOutput, "output", "o", "", "Output report to file (Markdown)")
	threatCmd.Flags().StringVarP(&threatFile, "file", "f", "", "Specific architecture/spec file to analyze")
}

type ThreatReport struct {
	SystemDescription string   `json:"system_description"`
	Threats           []Threat `json:"threats"`
}

type Threat struct {
	ID          string       `json:"id"`
	Category    string       `json:"category"`
	Component   string       `json:"component"`
	Description string       `json:"description"`
	Severity    string       `json:"severity"`
	Mitigations []Mitigation `json:"mitigations"`
}

type Mitigation struct {
	Description string `json:"description"`
	Status      string `json:"status"`
}

func runThreat(cmd *cobra.Command, args []string) error {
	inputFile := threatFile

	// 1. Resolve Input File
	if inputFile == "" && len(args) > 0 {
		inputFile = args[0]
	}

	if inputFile == "" {
		// Check defaults
		defaults := []string{
			".recac/architecture/architecture.yaml",
			"app_spec.txt",
		}
		for _, d := range defaults {
			if _, err := os.Stat(d); err == nil {
				inputFile = d
				break
			}
		}
	}

	if inputFile == "" {
		return fmt.Errorf("no input file specified and no default files found (.recac/architecture/architecture.yaml, app_spec.txt)")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Analyzing %s for threats...\n", inputFile)

	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// 2. Prepare Agent
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-threat")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are a Security Architect Expert.
Perform a STRIDE threat analysis on the following system description.

System Description:
'''
%s
'''

Identify potential threats for each category of STRIDE where applicable.
Return ONLY a JSON object matching the following structure:
{
  "system_description": "Brief summary of the system analyzed",
  "threats": [
    {
      "id": "T-1",
      "category": "Spoofing|Tampering|Repudiation|Information Disclosure|Denial of Service|Elevation of Privilege",
      "component": "Component Name",
      "description": "Description of the threat",
      "severity": "Critical|High|Medium|Low",
      "mitigations": [
        { "description": "Proposed mitigation", "status": "Proposed" }
      ]
    }
  ]
}
Do not use markdown formatting.
`, string(content))

	// 4. Call Agent
	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Identifying threats (STRIDE)...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 5. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var report ThreatReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, resp)
	}

	// 6. Output
	if threatJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	if threatOutput != "" {
		return writeThreatReportMarkdown(threatOutput, report)
	}

	printThreatTable(cmd, report)
	return nil
}

func printThreatTable(cmd *cobra.Command, report ThreatReport) {
	fmt.Fprintf(cmd.OutOrStdout(), "\nSystem: %s\n", report.SystemDescription)
	fmt.Fprintf(cmd.OutOrStdout(), "Identified Threats: %d\n\n", len(report.Threats))

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSEVERITY\tCATEGORY\tCOMPONENT\tDESCRIPTION")
	fmt.Fprintln(w, "--\t--------\t--------\t---------\t-----------")

	for _, t := range report.Threats {
		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.ID, t.Severity, t.Category, t.Component, desc)
	}
	w.Flush()
}

func writeThreatReportMarkdown(path string, report ThreatReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# Threat Model Report (STRIDE)\n\n")
	fmt.Fprintf(f, "**System Description:** %s\n\n", report.SystemDescription)

	fmt.Fprintf(f, "## Identified Threats (%d)\n\n", len(report.Threats))

	for _, t := range report.Threats {
		fmt.Fprintf(f, "### [%s] %s (%s)\n", t.ID, t.Description, t.Severity)
		fmt.Fprintf(f, "- **Category:** %s\n", t.Category)
		fmt.Fprintf(f, "- **Component:** %s\n", t.Component)
		fmt.Fprintf(f, "- **Mitigations:**\n")
		for _, m := range t.Mitigations {
			fmt.Fprintf(f, "  - %s (%s)\n", m.Description, m.Status)
		}
		fmt.Fprintln(f, "")
	}

	return nil
}
