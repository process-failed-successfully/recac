package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	radarJSON bool
	radarHTML bool
	radarOut  string
)

var radarCmd = &cobra.Command{
	Use:   "radar [path]",
	Short: "Generate a Technology Radar",
	Long: `Scans the project for technologies and generates a ThoughtWorks-style Tech Radar.
It uses AI to evaluate each technology's industry status (Adopt, Trial, Assess, Hold)
and categorizes them into Quadrants (Languages & Frameworks, Tools, Platforms, Techniques).`,
	RunE: runRadar,
}

func init() {
	rootCmd.AddCommand(radarCmd)
	radarCmd.Flags().BoolVar(&radarJSON, "json", false, "Output as JSON")
	radarCmd.Flags().BoolVar(&radarHTML, "html", false, "Output as an HTML report")
	radarCmd.Flags().StringVarP(&radarOut, "out", "o", "", "Output file path")
}

type RadarItem struct {
	Name        string `json:"name"`
	Quadrant    string `json:"quadrant"` // "Languages & Frameworks", "Tools", "Platforms", "Techniques"
	Ring        string `json:"ring"`     // "Adopt", "Trial", "Assess", "Hold"
	Description string `json:"description"`
}

func runRadar(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Collect Technologies
	info, err := analyzeStack(root)
	if err != nil {
		return fmt.Errorf("failed to analyze stack: %w", err)
	}

	techs := flattenStack(info)
	if len(techs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No technologies detected.")
		return nil
	}

	// 2. Ask AI to Classify
	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-radar")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze the following list of technologies used in this project.
Classify each into a ThoughtWorks Technology Radar.

Quadrants:
- "Languages & Frameworks"
- "Tools"
- "Platforms" (Infrastructure, DBs)
- "Techniques" (Patterns, Approaches - infer if possible, otherwise skip)

Rings:
- "Adopt": Proven, mature, use when appropriate.
- "Trial": Worth pursuing, use on low-risk projects.
- "Assess": Worth exploring, understanding how it affects you.
- "Hold": Process with caution, legacy, or avoid.

Technologies:
%s

Return a JSON list of objects:
[
  { "name": "TechName", "quadrant": "QuadrantName", "ring": "RingName", "description": "Brief reasoning" }
]

Only return the JSON.`, strings.Join(techs, ", "))

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Analyzing technologies and trends...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 3. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var items []RadarItem
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to parse agent response: %v\nResponse: %s\n", err, resp)
		return err
	}

	// 4. Output
	if radarJSON {
		return outputJSON(cmd, items)
	}
	if radarHTML {
		return outputHTML(cmd, items)
	}
	return outputText(cmd, items)
}

func flattenStack(info *StackInfo) []string {
	var techs []string
	for k := range info.Languages {
		techs = append(techs, k)
	}
	techs = append(techs, info.Frameworks...)
	techs = append(techs, info.Infrastructure...)
	techs = append(techs, info.Databases...)
	techs = append(techs, info.CI...)
	return techs
}

func outputJSON(cmd *cobra.Command, items []RadarItem) error {
	out := cmd.OutOrStdout()
	if radarOut != "" {
		f, err := os.Create(radarOut)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		return err
	}
	if radarOut != "" {
		fmt.Printf("JSON written to %s\n", radarOut)
	}
	return nil
}

func outputText(cmd *cobra.Command, items []RadarItem) error {
	// Group by Quadrant
	byQuad := make(map[string][]RadarItem)
	for _, item := range items {
		byQuad[item.Quadrant] = append(byQuad[item.Quadrant], item)
	}

	out := cmd.OutOrStdout()
	if radarOut != "" {
		f, err := os.Create(radarOut)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	fmt.Fprintln(out, "📡 TECHNOLOGY RADAR")
	fmt.Fprintln(out, "===================")

	quadrants := []string{"Languages & Frameworks", "Tools", "Platforms", "Techniques"}
	for _, q := range quadrants {
		list := byQuad[q]
		if len(list) == 0 {
			continue
		}
		fmt.Fprintf(out, "\n%s:\n", strings.ToUpper(q))
		for _, item := range list {
			icon := "⚪"
			switch item.Ring {
			case "Adopt":
				icon = "🟢"
			case "Trial":
				icon = "🔵"
			case "Assess":
				icon = "🟡"
			case "Hold":
				icon = "🔴"
			}
			fmt.Fprintf(out, "  %s %-15s [%s] %s\n", icon, item.Name, item.Ring, item.Description)
		}
	}
	return nil
}

func outputHTML(cmd *cobra.Command, items []RadarItem) error {
	tmpl := `<!DOCTYPE html>
<html>
<head>
<title>Tech Radar</title>
<style>
body { font-family: sans-serif; padding: 20px; background: #f4f4f4; }
.container { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; max-width: 1200px; margin: auto; }
.quadrant { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
h2 { border-bottom: 2px solid #ddd; padding-bottom: 10px; color: #333; }
.item { margin-bottom: 10px; padding: 10px; background: #fafafa; border-left: 5px solid #ccc; }
.ring-Adopt { border-left-color: #2ecc71; }
.ring-Trial { border-left-color: #3498db; }
.ring-Assess { border-left-color: #f1c40f; }
.ring-Hold { border-left-color: #e74c3c; }
.badge { font-size: 0.8em; padding: 2px 6px; border-radius: 4px; color: white; float: right; }
.bg-Adopt { background-color: #2ecc71; }
.bg-Trial { background-color: #3498db; }
.bg-Assess { background-color: #f1c40f; }
.bg-Hold { background-color: #e74c3c; }
</style>
</head>
<body>
<h1 style="text-align:center">Technology Radar</h1>
<div class="container">
{{ range $q, $items := . }}
    <div class="quadrant">
        <h2>{{ $q }}</h2>
        {{ range $items }}
        <div class="item ring-{{ .Ring }}">
            <strong>{{ .Name }}</strong>
            <span class="badge bg-{{ .Ring }}">{{ .Ring }}</span>
            <p style="margin: 5px 0 0; font-size: 0.9em; color: #666;">{{ .Description }}</p>
        </div>
        {{ end }}
    </div>
{{ end }}
</div>
</body>
</html>`

	// Group by Quadrant
	byQuad := make(map[string][]RadarItem)
	for _, item := range items {
		byQuad[item.Quadrant] = append(byQuad[item.Quadrant], item)
	}

	outPath := "radar.html"
	if radarOut != "" {
		outPath = radarOut
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	t, err := template.New("radar").Parse(tmpl)
	if err != nil {
		return err
	}

	if err := t.Execute(f, byQuad); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Radar HTML generated at %s\n", outPath)
	return nil
}
