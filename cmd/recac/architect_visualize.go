package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the system architecture",
	Long:  `Generates a Mermaid diagram from the architecture.yaml file.`,
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate an HTML file with the diagram")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	htmlFlag, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture.yaml: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if htmlFlag {
		htmlPath := filepath.Join(dir, "architecture.html")
		if err := generateHTML(htmlPath, mermaid); err != nil {
			return fmt.Errorf("failed to generate HTML: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Generated architecture visualization at %s\n", htmlPath)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	}

	return nil
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Helper to sanitize IDs
	sanitizeArchID := func(id string) string {
		// Replace common separators with underscore first
		id = strings.ReplaceAll(id, " ", "_")
		id = strings.ReplaceAll(id, "-", "_")
		id = strings.ReplaceAll(id, ".", "_")
		id = strings.ReplaceAll(id, "/", "_")

		// Remove any remaining invalid characters for Mermaid IDs
		re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
		return re.ReplaceAllString(id, "")
	}

	escapeMermaidLabel := func(s string) string {
		return strings.ReplaceAll(s, "\"", "'")
	}

	// 1. Nodes
	for _, comp := range arch.Components {
		id := sanitizeArchID(comp.ID)
		shapeStart, shapeEnd := "[", "]" // Default rectangle

		switch strings.ToLower(comp.Type) {
		case "database", "db":
			shapeStart, shapeEnd = "[(", ")]"
		case "worker":
			shapeStart, shapeEnd = "{{", "}}"
		case "queue", "topic":
			shapeStart, shapeEnd = ">", "]"
		case "frontend", "ui", "web":
			shapeStart, shapeEnd = "(", ")"
		}

		// Escape label just in case
		label := escapeMermaidLabel(comp.ID)
		typeLabel := escapeMermaidLabel(comp.Type)

		sb.WriteString(fmt.Sprintf("    %s%s\"%s<br/><small>%s</small>\"%s\n", id, shapeStart, label, typeLabel, shapeEnd))
	}

	// 2. Edges
	for _, comp := range arch.Components {
		targetID := sanitizeArchID(comp.ID)

		// Inputs (Source -> Target)
		for _, input := range comp.Consumes {
			sourceID := sanitizeArchID(input.Source)
			label := input.Type
			if label == "" {
				label = "consumes"
			}
			label = escapeMermaidLabel(label)
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", sourceID, label, targetID))
		}

		// Outputs (Source -> Target)
		for _, output := range comp.Produces {
			if output.Target != "" {
				destID := sanitizeArchID(output.Target)
				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "produces"
				}
				label = escapeMermaidLabel(label)
				sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", targetID, label, destID))
			}
		}
	}

	return sb.String()
}

func generateHTML(path string, mermaidContent string) error {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>System Architecture</title>
</head>
<body>
    <div class="mermaid">
    {{.}}
    </div>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>`

	t, err := template.New("arch").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use template.HTML to prevent escaping of arrow syntax
	return t.Execute(f, template.HTML(mermaidContent))
}
