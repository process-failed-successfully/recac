package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the system architecture",
	Long:  "Generates a Mermaid diagram from the architecture.yaml file.",
	Run:   runVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	visualizeCmd.Flags().String("out", "", "Output file (default: stdout, or architecture.html if --html is set)")
	visualizeCmd.Flags().Bool("html", false, "Generate an HTML file with embedded Mermaid diagram")
}

func runVisualizeCmd(cmd *cobra.Command, args []string) {
	dir, _ := cmd.Flags().GetString("dir")
	out, _ := cmd.Flags().GetString("out")
	htmlMode, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	archData, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(archData, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if htmlMode {
		htmlContent := generateHTML(mermaid)
		if out == "" {
			out = "architecture.html"
		}
		if err := os.WriteFile(out, []byte(htmlContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Generated HTML visualization at %s\n", out)
	} else {
		if out != "" {
			if err := os.WriteFile(out, []byte(mermaid), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Generated Mermaid diagram at %s\n", out)
		} else {
			fmt.Println(mermaid)
		}
	}
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes
	for _, comp := range arch.Components {
		id := sanitizeArchID(comp.ID)
		shapeStart := "["
		shapeEnd := "]"

		switch comp.Type {
		case "database", "db":
			shapeStart = "[("
			shapeEnd = ")]"
		case "queue":
			shapeStart = ">"
			shapeEnd = "]"
		case "worker":
			shapeStart = "{{"
			shapeEnd = "}}"
		case "frontend", "ui":
			shapeStart = "("
			shapeEnd = ")"
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s<br/>(%s)\"%s\n", id, shapeStart, comp.ID, comp.Type, shapeEnd))

		// Add click event? Maybe later.
	}

	// Edges
	// We track edges to avoid duplicates if defined in both Consumes and Produces
	edges := make(map[string]bool)

	for _, comp := range arch.Components {
		targetID := sanitizeArchID(comp.ID)

		// Consumes (Source -> Comp)
		for _, input := range comp.Consumes {
			sourceID := sanitizeArchID(input.Source)
			edgeKey := fmt.Sprintf("%s->%s", sourceID, targetID)
			if !edges[edgeKey] {
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, input.Type, targetID))
				edges[edgeKey] = true
			}
		}

		// Produces (Comp -> Target)
		for _, output := range comp.Produces {
			if output.Target != "" {
				destID := sanitizeArchID(output.Target)
				edgeKey := fmt.Sprintf("%s->%s", targetID, destID)
				if !edges[edgeKey] {
					label := output.Event
					if label == "" {
						label = output.Type
					}
					sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", targetID, label, destID))
					edges[edgeKey] = true
				}
			}
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	return strings.ReplaceAll(strings.ReplaceAll(id, "-", "_"), " ", "_")
}

func generateHTML(mermaid string) string {
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

	t, err := template.New("html").Parse(tmpl)
	if err != nil {
		return fmt.Sprintf("Error generating HTML: %v", err)
	}

	var sb strings.Builder
	if err := t.Execute(&sb, template.HTML(mermaid)); err != nil {
		return fmt.Sprintf("Error executing template: %v", err)
	}
	return sb.String()
}
