package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"recac/internal/utils"
	"recac/internal/vuln"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	radarOutput string
	radarOpen   bool
)

var radarCmd = &cobra.Command{
	Use:   "radar",
	Short: "Generate a Technology Radar from project dependencies",
	Long: `Analyzes the project's dependencies (from go.mod, package.json) and uses AI to generate a Tech Radar.
The radar classifies technologies into Quadrants (Languages & Frameworks, Tools, Platforms, Techniques)
and Rings (Adopt, Trial, Assess, Hold) to visualize the technology stack's maturity.`,
	RunE: runRadar,
}

func init() {
	rootCmd.AddCommand(radarCmd)
	radarCmd.Flags().StringVarP(&radarOutput, "output", "o", "tech-radar.html", "Output HTML file path")
	radarCmd.Flags().BoolVar(&radarOpen, "open", false, "Open the report in browser after generation")
}

// RadarEntry represents a single blip on the radar
type RadarEntry struct {
	Name        string `json:"name"`
	Quadrant    string `json:"quadrant"` // Languages & Frameworks, Tools, Platforms, Techniques
	Ring        string `json:"ring"`     // Adopt, Trial, Assess, Hold
	Description string `json:"description"`
}

func runRadar(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning dependencies...")

	// 1. Scan Dependencies
	pkgs, err := scanDependencies(cwd)
	if err != nil {
		return err
	}

	if len(pkgs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No dependencies found (checked go.mod, package.json).")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d dependencies. Analyzing with AI...\n", len(pkgs))

	// 2. AI Classification
	entries, err := classifyDependencies(cmd.Context(), cwd, pkgs)
	if err != nil {
		return err
	}

	// 3. Generate HTML
	if err := generateRadarHTML(entries, radarOutput); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Tech Radar generated: %s\n", radarOutput)

	if radarOpen {
		openBrowser(radarOutput) // reused from report.go (assumed accessible in package main)
	}

	return nil
}

func scanDependencies(root string) ([]vuln.Package, error) {
	var allPkgs []vuln.Package

	// Check go.mod
	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		parser := &vuln.GoModParser{}
		pkgs, err := parser.Parse(goModPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse go.mod: %w", err)
		}
		allPkgs = append(allPkgs, pkgs...)
	}

	// Check package.json
	pkgJsonPath := filepath.Join(root, "package.json")
	if _, err := os.Stat(pkgJsonPath); err == nil {
		parser := &vuln.PackageJsonParser{}
		pkgs, err := parser.Parse(pkgJsonPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse package.json: %w", err)
		}
		allPkgs = append(allPkgs, pkgs...)
	}

	return allPkgs, nil
}

func classifyDependencies(ctx context.Context, cwd string, pkgs []vuln.Package) ([]RadarEntry, error) {
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-radar")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Summarize packages for prompt to save tokens
	var pkgList []string
	for _, p := range pkgs {
		pkgList = append(pkgList, fmt.Sprintf("%s (%s)", p.Name, p.Ecosystem))
	}
	// Deduplicate
	pkgList = uniqueStrings(pkgList)

	// Chunking logic might be needed if too many dependencies, but let's assume < 200 for now.
	// If more, we might need multiple calls or just pick top ones.
	// For MVP, we send all.

	prompt := fmt.Sprintf(`You are a Chief Architect.
Classify the following libraries/technologies into a ThoughtWorks Tech Radar format.

Dependencies:
%s

For each item, determine:
1. Quadrant: "Languages & Frameworks", "Tools", "Platforms", or "Techniques" (use "Techniques" for libraries that implement specific patterns).
   - Most libraries fall under "Languages & Frameworks" or "Tools".
2. Ring: "Adopt" (proven, low risk), "Trial" (worth pursuing), "Assess" (worth exploring), "Hold" (avoid/deprecated).
   - Use your knowledge of the technology's maturity and popularity.
3. Description: A very short explanation (max 1 sentence) of what it is.

Return a JSON list:
[
  { "name": "library_name", "quadrant": "...", "ring": "...", "description": "..." }
]

Output ONLY valid JSON.
`, strings.Join(pkgList, "\n"))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	jsonStr := utils.CleanCodeBlock(resp)
	// Additional cleanup: sometimes agents add extra text outside blocks
	jsonStr = utils.CleanJSONBlock(jsonStr)

	var entries []RadarEntry
	if err := json.Unmarshal([]byte(jsonStr), &entries); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, resp)
	}

	return entries, nil
}

func generateRadarHTML(entries []RadarEntry, output string) error {
	tmpl := `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Tech Radar</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
<style>
  body { font-family: sans-serif; padding: 20px; background: #f4f4f9; }
  .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); }
  h1 { text-align: center; color: #333; }
  .chart-container { position: relative; height: 80vh; width: 100%; }
  table { width: 100%; margin-top: 20px; border-collapse: collapse; }
  th, td { padding: 10px; border-bottom: 1px solid #ddd; text-align: left; }
  th { background: #eee; }
</style>
</head>
<body>
<div class="container">
  <h1>Technology Radar</h1>
  <div class="chart-container">
    <canvas id="radarChart"></canvas>
  </div>

  <h2>Details</h2>
  <table>
    <thead>
      <tr><th>Name</th><th>Quadrant</th><th>Ring</th><th>Description</th></tr>
    </thead>
    <tbody>
      {{range .Entries}}
      <tr>
        <td>{{.Name}}</td>
        <td>{{.Quadrant}}</td>
        <td>{{.Ring}}</td>
        <td>{{.Description}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
</div>

<script>
  const data = {{.JSON}};

  // Map Rings to numeric values for Bubble Chart (radius/distance)
  // Adopt (Inner) -> 1, Trial -> 2, Assess -> 3, Hold (Outer) -> 4
  const ringMap = { "Adopt": 1, "Trial": 2, "Assess": 3, "Hold": 4 };

  // Map Quadrants to angles/coordinates roughly
  // We will use a Bubble Chart (Scatter)

  const quadrantMap = {
    "Languages & Frameworks": 0,
    "Tools": 1,
    "Platforms": 2,
    "Techniques": 3
  };

  function getRandom(min, max) {
    return Math.random() * (max - min) + min;
  }

  const datasets = [{
    label: 'Technologies',
    data: data.map(d => {
      const qIndex = quadrantMap[d.Quadrant] || 0;

      // Calculate random angle within quadrant
      const angleStart = qIndex * 90;
      const angleEnd = angleStart + 90;
      const angle = getRandom(angleStart + 10, angleEnd - 10) * (Math.PI / 180);

      // Calculate radius based on Ring (plus some jitter)
      // Adopt: 0-25, Trial: 25-50, Assess: 50-75, Hold: 75-100
      // Normalized: Adopt: 0.1-0.3, Trial: 0.35-0.55, Assess: 0.6-0.8, Hold: 0.85-1.0
      let rMin = 0, rMax = 0;
      if (d.Ring === "Adopt") { rMin = 0.1; rMax = 0.3; }
      else if (d.Ring === "Trial") { rMin = 0.35; rMax = 0.55; }
      else if (d.Ring === "Assess") { rMin = 0.6; rMax = 0.8; }
      else { rMin = 0.85; rMax = 1.0; }

      const r = getRandom(rMin, rMax);

      return {
        x: r * Math.cos(angle),
        y: r * Math.sin(angle),
        name: d.Name,
        ring: d.Ring,
        quadrant: d.Quadrant
      };
    }),
    backgroundColor: data.map(d => {
        if(d.Ring === "Adopt") return 'rgba(75, 192, 192, 0.6)';
        if(d.Ring === "Trial") return 'rgba(54, 162, 235, 0.6)';
        if(d.Ring === "Assess") return 'rgba(255, 206, 86, 0.6)';
        return 'rgba(255, 99, 132, 0.6)';
    })
  }];

  const ctx = document.getElementById('radarChart').getContext('2d');
  new Chart(ctx, {
    type: 'scatter',
    data: { datasets: datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        x: { display: false, min: -1.2, max: 1.2 },
        y: { display: false, min: -1.2, max: 1.2 }
      },
      plugins: {
        tooltip: {
          callbacks: {
            label: function(context) {
              const p = context.raw;
              return p.name + " (" + p.ring + ")";
            }
          }
        },
        legend: { display: false }
      }
    }
  });
</script>
</body>
</html>`

	// Prepare data for template
	jsonData, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	data := struct {
		JSON    template.JS
		Entries []RadarEntry
	}{
		JSON:    template.JS(jsonData),
		Entries: entries,
	}

	t, err := template.New("radar").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}
