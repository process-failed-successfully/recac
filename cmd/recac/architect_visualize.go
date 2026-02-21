package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	archVizIn   string
	archVizOut  string
	archVizHtml bool
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the system architecture",
	Long:  `Generates a Mermaid graph from the architecture.yaml file showing components and data flow.`,
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().StringVar(&archVizIn, "in", ".recac/architecture/architecture.yaml", "Path to architecture.yaml")
	architectVisualizeCmd.Flags().StringVarP(&archVizOut, "out", "o", "", "Output file (default stdout)")
	architectVisualizeCmd.Flags().BoolVar(&archVizHtml, "html", false, "Generate HTML file with embedded Mermaid")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	// 1. Read Architecture
	data, err := os.ReadFile(archVizIn)
	if err != nil {
		return fmt.Errorf("failed to read architecture file: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	// 2. Generate Mermaid
	mermaid := generateMermaidSystemGraph(&arch)

	// 3. Format Output
	var output []byte
	if archVizHtml {
		html, err := generateHTML(mermaid)
		if err != nil {
			return err
		}
		output = []byte(html)
	} else {
		output = []byte(mermaid)
	}

	// 4. Write
	if archVizOut != "" {
		if err := os.MkdirAll(filepath.Dir(archVizOut), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(archVizOut, output, 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		cmd.Printf("Visualization saved to %s\n", archVizOut)
	} else {
		cmd.Println(string(output))
	}

	return nil
}

func generateMermaidSystemGraph(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort components for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	// Nodes
	for _, c := range arch.Components {
		// Clean ID for Mermaid
		safeID := sanitizeArchID(c.ID)
		sb.WriteString(fmt.Sprintf("    %s[\"%s<br/>(%s)\"]\n", safeID, escapeLabel(c.ID), escapeLabel(c.Type)))

		// Style based on type
		switch strings.ToLower(c.Type) {
		case "database", "store":
			sb.WriteString(fmt.Sprintf("    style %s fill:#e1f5fe,stroke:#01579b,stroke-width:2px\n", safeID))
		case "service", "api":
			sb.WriteString(fmt.Sprintf("    style %s fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px\n", safeID))
		case "worker", "consumer":
			sb.WriteString(fmt.Sprintf("    style %s fill:#fff3e0,stroke:#ef6c00,stroke-width:2px\n", safeID))
		case "queue", "topic":
			sb.WriteString(fmt.Sprintf("    style %s fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px\n", safeID))
		}
	}

	// Edges (Consumes)
	for _, c := range arch.Components {
		safeID := sanitizeArchID(c.ID)
		for _, input := range c.Consumes {
			if input.Source != "" {
				sourceID := sanitizeArchID(input.Source)
				label := input.Type
				if label == "" {
					label = "consumes"
				}
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, label, safeID))
			}
		}
	}

	// Edges (Produces - if explicit target)
	for _, c := range arch.Components {
		safeID := sanitizeArchID(c.ID)
		for _, output := range c.Produces {
			if output.Target != "" {
				targetID := sanitizeArchID(output.Target)
				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "produces"
				}
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", safeID, label, targetID))
			}
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	// Replace non-alphanumeric chars with _
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, id)
}

func escapeLabel(s string) string {
	// Mermaid uses HTML labels by default in modern renderers, but let's just escape quotes.
	return strings.ReplaceAll(s, "\"", "#quot;")
}

func generateHTML(mermaidContent string) (string, error) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>System Architecture</title>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</head>
<body>
    <h1>System Architecture</h1>
    <div class="mermaid">
{{ . }}
    </div>
</body>
</html>`

	t, err := template.New("viz").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if err := t.Execute(&sb, template.HTML(mermaidContent)); err != nil {
		return "", err
	}
	return sb.String(), nil
}
