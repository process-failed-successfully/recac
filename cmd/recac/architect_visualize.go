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
	Short: "Visualize the generated system architecture",
	Long: `Generates a Mermaid diagram from the architecture.yaml file.
Can output raw Mermaid code or a standalone HTML file.`,
	RunE: runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().String("out", "", "Output file path (default: stdout for mermaid, architecture.html for html)")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate an HTML file with embedded Mermaid diagram")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	outFile, _ := cmd.Flags().GetString("out")
	htmlMode, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file at %s: %w", archPath, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if htmlMode {
		if outFile == "" {
			outFile = filepath.Join(dir, "architecture.html")
		}
		if err := generateHTML(outFile, mermaid); err != nil {
			return fmt.Errorf("failed to generate HTML: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "HTML diagram generated at %s\n", outFile)
	} else {
		if outFile != "" {
			if err := os.WriteFile(outFile, []byte(mermaid), 0644); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mermaid diagram saved to %s\n", outFile)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), mermaid)
		}
	}

	return nil
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Styles
	sb.WriteString("    classDef service fill:#f9f,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef database fill:#ccf,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef frontend fill:#ff9,stroke:#333,stroke-width:2px;\n")
	sb.WriteString("    classDef worker fill:#9f9,stroke:#333,stroke-width:2px;\n")

	// Nodes
	// Sort components by ID for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	for _, c := range arch.Components {
		// Sanitize ID
		safeID := sanitizeArchID(c.ID)
		label := fmt.Sprintf("%s<br/>(%s)", c.ID, c.Type)
		// Escape label for Mermaid
		label = escapeMermaidLabel(label)

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, label))

		// Apply style
		switch strings.ToLower(c.Type) {
		case "service", "api":
			sb.WriteString(fmt.Sprintf("    class %s service;\n", safeID))
		case "database", "store", "cache":
			sb.WriteString(fmt.Sprintf("    class %s database;\n", safeID))
		case "frontend", "ui", "web":
			sb.WriteString(fmt.Sprintf("    class %s frontend;\n", safeID))
		case "worker", "job":
			sb.WriteString(fmt.Sprintf("    class %s worker;\n", safeID))
		}
	}

	// Edges
	// We iterate components to find relationships.
	// Consumes: Source -> Component
	for _, c := range arch.Components {
		safeID := sanitizeArchID(c.ID)
		for _, input := range c.Consumes {
			if input.Source == "" {
				continue
			}
			safeSource := sanitizeArchID(input.Source)
			label := input.Type
			if label == "" {
				label = "uses"
			}
			label = escapeMermaidLabel(label)
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", safeSource, label, safeID))
		}
	}

	// Produces: Component -> Target (if explicit)
	for _, c := range arch.Components {
		safeID := sanitizeArchID(c.ID)
		for _, output := range c.Produces {
			if output.Target == "" {
				continue
			}
			safeTarget := sanitizeArchID(output.Target)
			label := output.Type
			if label == "" {
				label = output.Event
			}
			if label == "" {
				label = "produces"
			}
			label = escapeMermaidLabel(label)
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", safeID, label, safeTarget))
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	// Replace non-alphanumeric with underscore
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, id)
}

func escapeMermaidLabel(label string) string {
	return strings.ReplaceAll(label, "\"", "'")
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>System Architecture</title>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script>mermaid.initialize({startOnLoad:true});</script>
</head>
<body>
    <h1>System Architecture</h1>
    <div class="mermaid">
        {{.}}
    </div>
</body>
</html>`

func generateHTML(path string, mermaidContent string) error {
	tmpl, err := template.New("arch").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// We need to prevent HTML escaping of the mermaid content (like arrows -->)
	return tmpl.Execute(f, template.HTML(mermaidContent))
}
