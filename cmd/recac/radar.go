package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/analysis"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	radarOut  string
	radarJSON bool
)

var radarCmd = &cobra.Command{
	Use:   "radar",
	Short: "Generate a Technology Radar",
	Long: `Scans the project for dependencies and configuration files, then uses AI to
categorize technologies into a "Technology Radar" format (Adopt, Trial, Assess, Hold).
Generates an interactive HTML report.`,
	RunE: runRadar,
}

func init() {
	rootCmd.AddCommand(radarCmd)
	radarCmd.Flags().StringVarP(&radarOut, "out", "o", "tech-radar.html", "Output file path")
	radarCmd.Flags().BoolVar(&radarJSON, "json", false, "Output results as JSON instead of HTML")
}

func runRadar(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Scan for Files
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning for dependency files...")
	files, err := analysis.ScanForDependencyFiles(cwd)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(files) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No dependency files found (e.g., go.mod, package.json).")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Found %d files: %v\n", len(files), files)

	// 2. Read Content
	var contentBuilder strings.Builder
	totalSize := 0
	const maxSize = 50 * 1024 // 50KB limit to avoid token overflow

	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.Size() > 100*1024 {
			// Skip huge files
			fmt.Fprintf(cmd.ErrOrStderr(), "Skipping large file %s\n", f)
			continue
		}

		b, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to read %s: %v\n", f, err)
			continue
		}

		// Truncate if individual file is too big?
		// Better to truncate globally.
		if totalSize+len(b) > maxSize {
			fmt.Fprintf(cmd.ErrOrStderr(), "Context limit reached, stopping file read at %s\n", f)
			break
		}

		contentBuilder.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n", f))
		contentBuilder.Write(b)
		totalSize += len(b)
	}

	// 3. Ask Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-radar")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze the following dependency files and configuration.
Identify the key technologies (Languages, Frameworks, Tools, Platforms, Libraries) used in this project.
Categorize them into a ThoughtWorks Technology Radar format.

Quadrants:
- "Languages & Frameworks"
- "Tools"
- "Platforms" (Infrastructure, DBs, Cloud)
- "Techniques" (Architectural patterns, if inferable)

Rings (based on maturity and standard usage):
- "Adopt": Proven, standard, widely used in this project.
- "Trial": Used but maybe new or experimental.
- "Assess": Worth exploring or used in a limited scope.
- "Hold": Legacy, deprecated, or problematic.

Return ONLY a JSON array of objects with the following keys:
- "name": string
- "quadrant": string
- "ring": string
- "description": string (short summary of what it is and why it's in this ring)

Files Content:
%s`, contentBuilder.String())

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Analyzing technologies with AI...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Parse JSON
	cleanedJSON := utils.CleanCodeBlock(resp)
	var items []analysis.RadarItem
	if err := json.Unmarshal([]byte(cleanedJSON), &items); err != nil {
		return fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, resp)
	}

	// 5. Output
	if radarJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	html, err := analysis.GenerateRadarHTML(items)
	if err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}

	if err := os.WriteFile(radarOut, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Tech Radar generated at %s\n", radarOut)
	return nil
}
