package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	reportFormat string
	reportOutput string
	reportOpen   bool
)

var reportCmd = &cobra.Command{
	Use:   "report [path]",
	Short: "Generate a comprehensive code quality report",
	Long: `Generates a detailed report aggregating:
- Code Complexity
- Code Smells
- Copy-Paste Detection (CPD)
- TODO/FIXME scan
- Security Vulnerabilities (placeholder for now)
- Test Coverage (optional, if available)

The report is generated as an HTML file by default.`,
	RunE: runReport,
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().StringVarP(&reportFormat, "format", "f", "html", "Report format (html, json)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "recac_report.html", "Output file path")
	reportCmd.Flags().BoolVar(&reportOpen, "open", false, "Open the report in browser after generation")
}

// ReportData holds all the data for the report
type ReportData struct {
	GeneratedAt  time.Time            `json:"generated_at"`
	ProjectName  string               `json:"project_name"`
	Complexity   []FunctionComplexity `json:"complexity"`
	Smells       []SmellFinding       `json:"smells"`
	Duplications []Duplication        `json:"duplications"`
	Todos        []TodoItem           `json:"todos"`
	Stats        ProjectStats         `json:"stats"`
	TotalIssues  int                  `json:"total_issues"`
	HealthScore  int                  `json:"health_score"` // 0-100
}

type ProjectStats struct {
	TotalFiles       int `json:"total_files"`
	TotalGoFiles     int `json:"total_go_files"`
	TotalLines       int `json:"total_lines"`
	ComplexityIssues int `json:"complexity_issues"`
	SmellIssues      int `json:"smell_issues"`
	DuplicationCount int `json:"duplication_count"`
	TodoCount        int `json:"todo_count"`
}

func runReport(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Starting comprehensive code analysis...")

	data := ReportData{
		GeneratedAt: time.Now(),
		ProjectName: getProjectName(path),
	}

	// 1. Complexity
	fmt.Fprintln(cmd.OutOrStdout(), "  - Analyzing complexity...")
	compResults, err := runComplexityAnalysis(path)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: Complexity analysis failed: %v\n", err)
	}
	// Filter significant complexity
	for _, c := range compResults {
		if c.Complexity >= 10 { // Default threshold
			data.Complexity = append(data.Complexity, c)
		}
	}

	// 2. Smells
	fmt.Fprintln(cmd.OutOrStdout(), "  - Detecting code smells...")
	smellResults, err := analyzeSmells(path)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: Smell analysis failed: %v\n", err)
	}
	data.Smells = smellResults

	// 3. CPD
	fmt.Fprintln(cmd.OutOrStdout(), "  - Checking for duplicates...")
	cpdResults, err := runCPD(path, 10, []string{}) // Default 10 lines
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: CPD analysis failed: %v\n", err)
	}
	data.Duplications = cpdResults

	// 4. TODOs
	fmt.Fprintln(cmd.OutOrStdout(), "  - Scanning TODOs...")
	todos, err := ScanForTodos(path)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: TODO scan failed: %v\n", err)
	}
	data.Todos = todos

	// 5. Calculate Stats
	data.Stats.ComplexityIssues = len(data.Complexity)
	data.Stats.SmellIssues = len(data.Smells)
	data.Stats.DuplicationCount = len(data.Duplications)
	data.Stats.TodoCount = len(data.Todos)
	data.TotalIssues = data.Stats.ComplexityIssues + data.Stats.SmellIssues + data.Stats.DuplicationCount + data.Stats.TodoCount

	// Calculate simple files stats
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			data.Stats.TotalFiles++
			if strings.HasSuffix(p, ".go") {
				data.Stats.TotalGoFiles++
			}
			// Estimate lines?
		}
		return nil
	})

	// Calculate Health Score (Basic heuristic)
	// Start at 100, deduct points for issues
	score := 100
	score -= data.Stats.ComplexityIssues * 2
	score -= data.Stats.SmellIssues * 1
	score -= data.Stats.DuplicationCount * 5
	// score -= data.Stats.TodoCount * 0.1 // Todos are fine
	if score < 0 {
		score = 0
	}
	data.HealthScore = score

	// 6. Generate Output
	if reportFormat == "json" {
		f, err := os.Create(reportOutput)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ JSON Report generated: %s\n", reportOutput)
	} else {
		if reportOutput == "recac_report.html" && !strings.HasSuffix(reportOutput, ".html") {
			reportOutput += ".html"
		}
		if err := generateHTMLReport(data, reportOutput); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ HTML Report generated: %s\n", reportOutput)
	}

	if reportOpen {
		openBrowser(reportOutput)
	}

	return nil
}

func getProjectName(path string) string {
	abs, _ := filepath.Abs(path)
	return filepath.Base(abs)
}

func openBrowser(url string) {
	var err error
	switch os.Getenv("GOOS") {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		fmt.Printf("Error opening browser: %v\n", err)
	}
}

func generateHTMLReport(data ReportData, filename string) error {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>RECAC Report: {{.ProjectName}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #333; max-width: 1200px; margin: 0 auto; padding: 20px; background: #f4f4f9; }
        h1, h2, h3 { color: #2c3e50; }
        .header { background: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 20px; display: flex; justify-content: space-between; align-items: center; }
        .score { font-size: 2.5em; font-weight: bold; color: {{if ge .HealthScore 80}}#27ae60{{else if ge .HealthScore 50}}#f39c12{{else}}#c0392b{{end}}; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .stat-card { background: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.05); text-align: center; }
        .stat-value { font-size: 2em; font-weight: bold; color: #3498db; }
        .section { background: #fff; padding: 25px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 30px; }
        table { width: 100%; border-collapse: collapse; margin-top: 15px; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f9fa; }
        tr:hover { background-color: #f1f1f1; }
        .tag { display: inline-block; padding: 3px 8px; border-radius: 4px; font-size: 0.85em; font-weight: bold; }
        .tag-high { background: #ffebee; color: #c62828; }
        .tag-medium { background: #fff3e0; color: #ef6c00; }
        .tag-low { background: #e8f5e9; color: #2e7d32; }
        code { background: #f4f4f4; padding: 2px 5px; border-radius: 3px; font-family: monospace; }
    </style>
</head>
<body>
    <div class="header">
        <div>
            <h1>📊 Code Quality Report</h1>
            <p>Project: <strong>{{.ProjectName}}</strong> | Generated: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
        </div>
        <div style="text-align: right;">
            <div>Health Score</div>
            <div class="score">{{.HealthScore}}</div>
        </div>
    </div>

    <div class="stats-grid">
        <div class="stat-card">
            <div class="stat-value">{{.Stats.TotalFiles}}</div>
            <div>Files Scanned</div>
        </div>
        <div class="stat-card">
            <div class="stat-value" style="color: #e74c3c;">{{.Stats.ComplexityIssues}}</div>
            <div>Complexity Hotspots</div>
        </div>
        <div class="stat-card">
            <div class="stat-value" style="color: #f39c12;">{{.Stats.SmellIssues}}</div>
            <div>Code Smells</div>
        </div>
        <div class="stat-card">
            <div class="stat-value" style="color: #9b59b6;">{{.Stats.DuplicationCount}}</div>
            <div>Duplications</div>
        </div>
    </div>

    {{if .Complexity}}
    <div class="section">
        <h2>🔥 Complexity Hotspots</h2>
        <p>Functions with Cyclomatic Complexity >= 10</p>
        <table>
            <thead>
                <tr>
                    <th>Complexity</th>
                    <th>Function</th>
                    <th>Location</th>
                </tr>
            </thead>
            <tbody>
                {{range .Complexity}}
                <tr>
                    <td><strong>{{.Complexity}}</strong></td>
                    <td><code>{{.Function}}</code></td>
                    <td>{{.File}}:{{.Line}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    {{end}}

    {{if .Smells}}
    <div class="section">
        <h2>👃 Code Smells</h2>
        <p>Issues detected by static analysis</p>
        <table>
            <thead>
                <tr>
                    <th>Type</th>
                    <th>Function</th>
                    <th>Value</th>
                    <th>Location</th>
                </tr>
            </thead>
            <tbody>
                {{range .Smells}}
                <tr>
                    <td><span class="tag tag-medium">{{.Type}}</span></td>
                    <td><code>{{.Function}}</code></td>
                    <td>{{.Value}} (max {{.Threshold}})</td>
                    <td>{{.File}}:{{.Line}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    {{end}}

    {{if .Duplications}}
    <div class="section">
        <h2>👯 Duplicated Code</h2>
        <p>Identical blocks of code (copy-paste)</p>
        <table>
            <thead>
                <tr>
                    <th>Lines</th>
                    <th>Location A</th>
                    <th>Location B</th>
                </tr>
            </thead>
            <tbody>
                {{range .Duplications}}
                <tr>
                    <td>{{.LineCount}}</td>
                    <td>{{(index .Locations 0).File}}:{{(index .Locations 0).StartLine}}-{{(index .Locations 0).EndLine}}</td>
                    <td>{{(index .Locations 1).File}}:{{(index .Locations 1).StartLine}}-{{(index .Locations 1).EndLine}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    {{end}}

    <div class="section">
        <h2>📝 TODOs & FIXMEs</h2>
        <p>Pending tasks found in comments</p>
        <table>
            <thead>
                <tr>
                    <th>Type</th>
                    <th>Task</th>
                    <th>Location</th>
                </tr>
            </thead>
            <tbody>
                {{range .Todos}}
                <tr>
                    <td><span class="tag {{if eq .Keyword "FIXME"}}tag-high{{else}}tag-low{{end}}">{{.Keyword}}</span></td>
                    <td>{{.Content}}</td>
                    <td>{{.File}}:{{.Line}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>

</body>
</html>`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, data)
}
