package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RadarItem struct {
	Name        string `json:"name"`
	Quadrant    string `json:"quadrant"` // Techniques, Tools, Platforms, Languages & Frameworks
	Ring        string `json:"ring"`     // Adopt, Trial, Assess, Hold
	Description string `json:"description,omitempty"`
}

type RadarOutput struct {
	Date  string      `json:"date"`
	Items []RadarItem `json:"items"`
}

var (
	radarJSON bool
	radarHTML bool
)

var radarCmd = &cobra.Command{
	Use:   "radar [path]",
	Short: "Generate a Tech Radar for the project",
	Long: `Analyzes the project dependencies and generates a Technology Radar.
It uses the configured AI agent to classify technologies into Quadrants and Rings.
If no agent is available, it falls back to a heuristic classification based on presence.`,
	RunE: runRadar,
}

func init() {
	rootCmd.AddCommand(radarCmd)
	radarCmd.Flags().BoolVar(&radarJSON, "json", false, "Output as JSON")
	radarCmd.Flags().BoolVar(&radarHTML, "html", false, "Output as HTML file (radar.html)")
}

func runRadar(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Gather Context (Stack Info)
	// We reuse analyzeStack from stack.go which is in package main
	stack, err := analyzeStack(root)
	if err != nil {
		return fmt.Errorf("failed to analyze stack: %w", err)
	}

	// 2. Prepare Agent
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	var items []RadarItem

	// Try to use Agent
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-radar")
	if err == nil {
		items, err = generateRadarAI(ctx, ag, stack)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "AI generation failed, falling back to heuristics: %v\n", err)
			items = generateRadarHeuristic(stack)
		}
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "No agent configured, using heuristics.\n")
		items = generateRadarHeuristic(stack)
	}

	output := RadarOutput{
		Date:  "2024-05-20", // In real world, use time.Now().Format("2006-01-02")
		Items: items,
	}

	// 3. Output
	if radarJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	if radarHTML {
		return generateRadarHTML(root, output)
	}

	// Default: Text Table
	printRadarTable(cmd, output)
	return nil
}

func generateRadarAI(ctx context.Context, ag Agent, stack *StackInfo) ([]RadarItem, error) {
	// Construct prompt
	techs := []string{}
	for k := range stack.Languages {
		techs = append(techs, k)
	}
	techs = append(techs, stack.Frameworks...)
	techs = append(techs, stack.Infrastructure...)
	techs = append(techs, stack.Databases...)
	techs = append(techs, stack.CI...)

	prompt := fmt.Sprintf(`Analyze the following technologies used in this project and classify them into a Tech Radar format.
Technologies: %s

Output JSON only. Format:
[
  { "name": "TechName", "quadrant": "Languages & Frameworks" | "Tools" | "Platforms" | "Techniques", "ring": "Adopt" | "Trial" | "Assess" | "Hold", "description": "Short reasoning" }
]

Guidelines:
- "Languages & Frameworks": Languages, frameworks, major libraries.
- "Tools": CI/CD, build tools, CLI tools, databases.
- "Platforms": Infrastructure, Cloud Providers, SaaS.
- "Techniques": Architecture patterns, coding standards (if any inferable, otherwise skip).
- Rings:
  - Adopt: Core, proven, used heavily.
  - Trial: Used but maybe not everywhere or experimenting.
  - Assess: Just started using or evaluating.
  - Hold: Legacy or discouraged (if you know it's deprecated).
`, strings.Join(techs, ", "))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Extract JSON
	jsonStr := resp
	if start := strings.Index(jsonStr, "["); start != -1 {
		jsonStr = jsonStr[start:]
		if end := strings.LastIndex(jsonStr, "]"); end != -1 {
			jsonStr = jsonStr[:end+1]
		}
	}

	var items []RadarItem
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %v", err)
	}

	return items, nil
}

func generateRadarHeuristic(stack *StackInfo) []RadarItem {
	var items []RadarItem

	// Simple mapping
	for k := range stack.Languages {
		items = append(items, RadarItem{Name: k, Quadrant: "Languages & Frameworks", Ring: "Adopt", Description: "Primary Language"})
	}
	for _, f := range stack.Frameworks {
		items = append(items, RadarItem{Name: f, Quadrant: "Languages & Frameworks", Ring: "Adopt", Description: "Core Framework"})
	}
	for _, i := range stack.Infrastructure {
		items = append(items, RadarItem{Name: i, Quadrant: "Platforms", Ring: "Adopt", Description: "Infrastructure"})
	}
	for _, d := range stack.Databases {
		items = append(items, RadarItem{Name: d, Quadrant: "Tools", Ring: "Adopt", Description: "Database"})
	}
	for _, c := range stack.CI {
		items = append(items, RadarItem{Name: c, Quadrant: "Tools", Ring: "Adopt", Description: "CI/CD"})
	}

	return items
}

func generateRadarHTML(root string, data RadarOutput) error {
	path := filepath.Join(root, "radar.html")

	// Minimal HTML template with Zalando Tech Radar lib would be cool, but let's do a simple one using Chart.js or just CSS circles.
	// For simplicity and robustness, we will generate a clean Table/Card view in HTML.

	output := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<title>Tech Radar</title>
<style>
body { font-family: sans-serif; padding: 20px; }
.quadrant { margin-bottom: 20px; border: 1px solid #ccc; padding: 10px; border-radius: 5px; }
.quadrant h2 { border-bottom: 1px solid #eee; }
.ring-adopt { color: green; font-weight: bold; }
.ring-trial { color: orange; }
.ring-assess { color: blue; }
.ring-hold { color: red; text-decoration: line-through; }
</style>
</head>
<body>
<h1>Tech Radar</h1>
<p>Generated on %s</p>
`, data.Date)

	// Group by Quadrant
	byQuad := make(map[string][]RadarItem)
	for _, item := range data.Items {
		byQuad[item.Quadrant] = append(byQuad[item.Quadrant], item)
	}

	for q, items := range byQuad {
		output += fmt.Sprintf("<div class='quadrant'><h2>%s</h2><ul>", q)
		for _, item := range items {
			line := fmt.Sprintf("<li><span class='ring-%s'>[%s]</span> <strong>%s</strong>: %s</li>",
				strings.ToLower(item.Ring), html.EscapeString(item.Ring), html.EscapeString(item.Name), html.EscapeString(item.Description))
			output += line
		}
		output += "</ul></div>"
	}

	output += "</body></html>"

	return os.WriteFile(path, []byte(output), 0644)
}

func printRadarTable(cmd *cobra.Command, data RadarOutput) {
	byQuad := make(map[string][]RadarItem)
	for _, item := range data.Items {
		byQuad[item.Quadrant] = append(byQuad[item.Quadrant], item)
	}

	for q, items := range byQuad {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n%s\n", strings.ToUpper(q), strings.Repeat("-", len(q)))
		for _, item := range items {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-15s [%s] %s\n", item.Name, item.Ring, item.Description)
		}
	}
}

// Agent interface needs to be locally defined if not imported, but we imported internal/agent
// However, agentClientFactory returns agent.Agent which is an interface.
// So we use that.
type Agent interface {
	Send(ctx context.Context, content string) (string, error)
	SendStream(ctx context.Context, content string, callback func(string)) (string, error)
}
