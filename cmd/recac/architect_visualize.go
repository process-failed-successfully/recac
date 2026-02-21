package main

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/architecture"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the architecture as a Mermaid diagram",
	RunE:  runArchitectVisualizeCmd,
}

var htmlFlag bool

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().BoolVar(&htmlFlag, "html", false, "Generate an HTML file with the diagram")
}

func runArchitectVisualizeCmd(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	archPath := filepath.Join(dir, "architecture.yaml")

	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", archPath, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("error parsing architecture.yaml: %w", err)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if htmlFlag {
		html := generateHTML(mermaid)
		outPath := filepath.Join(dir, "architecture.html")
		if err := os.WriteFile(outPath, []byte(html), 0644); err != nil {
			return fmt.Errorf("error writing HTML: %w", err)
		}
		fmt.Printf("Generated HTML: %s\n", outPath)
	} else {
		fmt.Println(mermaid)
	}

	return nil
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Components
	for _, comp := range arch.Components {
		// Shapes based on type
		shapeStart, shapeEnd := "[", "]"
		if comp.Type == "database" || comp.Type == "db" {
			shapeStart, shapeEnd = "[(", ")]"
		} else if comp.Type == "worker" {
			shapeStart, shapeEnd = "{{", "}}"
		} else if comp.Type == "queue" {
			shapeStart, shapeEnd = ">", "]"
		} else if comp.Type == "frontend" || comp.Type == "ui" {
			shapeStart, shapeEnd = "(", ")"
		}

		safeID := sanitizeArchID(comp.ID)
		// Escape quotes in description/label if necessary, but here we use ID and type
		escapedID := escapeMermaidLabel(comp.ID)
		escapedType := escapeMermaidLabel(comp.Type)
		label := fmt.Sprintf("%s<br/>(%s)", escapedID, escapedType)

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", safeID, shapeStart, label, shapeEnd))
	}

	// Relationships
	for _, comp := range arch.Components {
		safeCompID := sanitizeArchID(comp.ID)
		for _, consumes := range comp.Consumes {
			if consumes.Source != "" {
				safeSourceID := sanitizeArchID(consumes.Source)
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", safeSourceID, escapeMermaidLabel(consumes.Type), safeCompID))
			}
		}
		for _, produces := range comp.Produces {
			if produces.Target != "" {
				safeTargetID := sanitizeArchID(produces.Target)
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", safeCompID, escapeMermaidLabel(produces.Type), safeTargetID))
			}
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, " ", "_")
	return id
}

func escapeMermaidLabel(label string) string {
	label = strings.ReplaceAll(label, "\"", "#quot;")
	label = strings.ReplaceAll(label, "<", "#lt;")
	label = strings.ReplaceAll(label, ">", "#gt;")
	return label
}

func generateHTML(mermaid string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>System Architecture</title>
</head>
<body>
    <h1>System Architecture</h1>
    <div class="mermaid">
%s
    </div>
    <script type="module">
      import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
      mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>`, mermaid)
}
