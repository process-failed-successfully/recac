package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the generated system architecture",
	Long:  "Generates a Mermaid graph from the architecture.yaml file.",
	Run:   runArchitectVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	visualizeCmd.Flags().Bool("html", false, "Output as HTML file with embedded Mermaid diagram")
}

func runArchitectVisualizeCmd(cmd *cobra.Command, args []string) {
	dir, _ := cmd.Flags().GetString("dir")
	html, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if html {
		htmlContent := generateHTML(mermaid)
		outputPath := filepath.Join(dir, "architecture.html")
		if err := os.WriteFile(outputPath, []byte(htmlContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Visualization saved to %s\n", outputPath)
	} else {
		fmt.Println(mermaid)
	}
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Nodes
	for _, comp := range arch.Components {
		id := sanitizeArchID(comp.ID)
		shapeStart, shapeEnd := "[", "]"

		switch strings.ToLower(comp.Type) {
		case "database", "db":
			shapeStart, shapeEnd = "[(", ")]"
		case "queue", "topic":
			shapeStart, shapeEnd = ">", "]"
		case "worker":
			shapeStart, shapeEnd = "{{", "}}"
		case "frontend", "ui":
			shapeStart, shapeEnd = "(", ")"
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s<br/>(%s)\"%s\n", id, shapeStart, comp.ID, comp.Type, shapeEnd))
	}

	// 2. Edges (Consumes)
	for _, comp := range arch.Components {
		targetID := sanitizeArchID(comp.ID)
		for _, input := range comp.Consumes {
			sourceID := sanitizeArchID(input.Source)
			if sourceID != "" {
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, input.Type, targetID))
			}
		}
	}

	// 3. Edges (Produces - explicit target)
	for _, comp := range arch.Components {
		sourceID := sanitizeArchID(comp.ID)
		for _, output := range comp.Produces {
			targetID := sanitizeArchID(output.Target)
			if targetID != "" {
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, output.Type, targetID))
			}
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, id)
}

func generateHTML(mermaid string) string {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Architecture Visualization</title>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script>mermaid.initialize({startOnLoad:true});</script>
</head>
<body>
    <div class="mermaid">
{{ .Mermaid }}
    </div>
</body>
</html>`

	t := template.Must(template.New("html").Parse(tmpl))
	var sb strings.Builder
	t.Execute(&sb, struct{ Mermaid string }{mermaid})
	return sb.String()
}
