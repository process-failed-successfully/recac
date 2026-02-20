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

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the system architecture",
	Long:  "Generates a Mermaid graph and optional HTML visualization of the architecture defined in architecture.yaml.",
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate an HTML file with embedded Mermaid diagram")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	html, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("error reading architecture.yaml: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("error parsing architecture.yaml: %w", err)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if html {
		htmlContent, err := generateHTML(mermaid)
		if err != nil {
			return fmt.Errorf("error generating HTML: %w", err)
		}

		outPath := filepath.Join(dir, "architecture.html")
		if err := os.WriteFile(outPath, []byte(htmlContent), 0644); err != nil {
			return fmt.Errorf("error writing HTML file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Visualization generated at %s\n", outPath)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	}
	return nil
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort components for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	// Define nodes
	for _, c := range arch.Components {
		id := sanitizeArchID(c.ID)
		label := escapeMermaidLabel(c.ID)
		shapeStart, shapeEnd := "[", "]"

		switch strings.ToLower(c.Type) {
		case "database", "db":
			shapeStart, shapeEnd = "[(", ")]"
		case "worker":
			shapeStart, shapeEnd = "{{", "}}"
		case "queue":
			shapeStart, shapeEnd = ">", "]"
		case "frontend", "ui":
			shapeStart, shapeEnd = "(", ")"
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", id, shapeStart, label, shapeEnd))
		sb.WriteString(fmt.Sprintf("    class %s %s\n", id, sanitizeArchID(c.Type)))
	}

	// Define edges
	for _, c := range arch.Components {
		cID := sanitizeArchID(c.ID)

		// Consumes: Source -> Component
		for _, input := range c.Consumes {
			if input.Source == "" {
				continue
			}
			srcID := sanitizeArchID(input.Source)
			label := escapeMermaidLabel(input.Type)
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", srcID, label, cID))
		}

		// Produces: Component -> Target
		for _, output := range c.Produces {
			if output.Target == "" {
				continue
			}
			targetID := sanitizeArchID(output.Target)
			label := output.Event
			if label == "" {
				label = output.Type
			}
			label = escapeMermaidLabel(label)
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", cID, label, targetID))
		}
	}

	// Styling
	sb.WriteString("\n    classDef service fill:#f9f,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef database fill:#dfd,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef db fill:#dfd,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef worker fill:#ddf,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef queue fill:#fdd,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef frontend fill:#ffd,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef ui fill:#ffd,stroke:#333,stroke-width:2px;\n")

	return sb.String()
}

func sanitizeArchID(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, s)
}

func escapeMermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "#quot;")
	s = strings.ReplaceAll(s, "<", "#lt;")
	s = strings.ReplaceAll(s, ">", "#gt;")
	return s
}

func generateHTML(mermaid string) (string, error) {
	const tpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>System Architecture Visualization</title>
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
</html>
`
	t, err := template.New("webpage").Parse(tpl)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if err := t.Execute(&sb, template.HTML(mermaid)); err != nil {
		return "", err
	}

	return sb.String(), nil
}
