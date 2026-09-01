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
	Long:  "Generates a Mermaid diagram from architecture.yaml and optionally creates an HTML viewer.",
	Run:   runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().Bool("html", false, "Generate an HTML file with the diagram")
	// "dir" flag is not needed if we inherit from architectCmd, but architectCmd doesn't persist flags?
	// architectCmd flags: spec, out.
	// We want to read from "out". So let's add a "dir" flag defaulting to .recac/architecture
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) {
	dir, _ := cmd.Flags().GetString("dir")
	generateHTML, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", archPath, err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if generateHTML {
		htmlPath := filepath.Join(dir, "architecture.html")
		if err := writeHTML(htmlPath, arch.SystemName, mermaid); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Visualization generated at: %s\n", htmlPath)
	} else {
		fmt.Println(mermaid)
	}
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Nodes
	// Sort components for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	// Track known nodes to identify external systems
	knownNodes := make(map[string]bool)
	for _, c := range arch.Components {
		knownNodes[c.ID] = true
	}

	for _, c := range arch.Components {
		// Default shape: rectangle [ ]
		shapeStart, shapeEnd := "[", "]"

		switch strings.ToLower(c.Type) {
		case "database", "db":
			shapeStart, shapeEnd = "[(", ")]" // Cylinder
		case "worker":
			shapeStart, shapeEnd = "{{", "}}" // Hexagon
		case "queue", "topic":
			shapeStart, shapeEnd = ">", "]"   // Asymmetric
		case "frontend", "ui":
			shapeStart, shapeEnd = "(", ")"   // Rounded rect? No, ( ) is rounded rect
		}

		safeID := sanitizeArchID(c.ID)
		label := escapeMermaidLabel(c.ID)
		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", safeID, shapeStart, label, shapeEnd))
	}

	// 2. Edges and External Nodes
	externalNodes := make(map[string]bool)
	var edges []string

	for _, c := range arch.Components {
		cSafeID := sanitizeArchID(c.ID)

		// Consumes: Source -> Component
		for _, inp := range c.Consumes {
			src := inp.Source
			if src == "" {
				continue
			}

			// If source is not a known component, it's external
			if !knownNodes[src] {
				externalNodes[src] = true
			}

			srcSafeID := sanitizeArchID(src)
			label := inp.Type
			if label == "" {
				label = "uses"
			}
			edges = append(edges, fmt.Sprintf("    %s -->|%s| %s", srcSafeID, escapeMermaidLabel(label), cSafeID))
		}

		// Produces: Component -> Target
		for _, out := range c.Produces {
			target := out.Target
			if target == "" {
				continue
			}

			// If target is not a known component, it's external
			if !knownNodes[target] {
				externalNodes[target] = true
			}

			targetSafeID := sanitizeArchID(target)
			label := out.Event
			if label == "" {
				label = out.Type
			}
			if label == "" {
				label = "produces"
			}
			edges = append(edges, fmt.Sprintf("    %s -->|%s| %s", cSafeID, escapeMermaidLabel(label), targetSafeID))
		}
	}

	// Add external nodes
	var sortedExternals []string
	for ext := range externalNodes {
		sortedExternals = append(sortedExternals, ext)
	}
	sort.Strings(sortedExternals)

	if len(sortedExternals) > 0 {
		sb.WriteString("\n    %% External Systems\n")
		for _, ext := range sortedExternals {
			safeID := sanitizeArchID(ext)
			// External systems style
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, escapeMermaidLabel(ext)))
			sb.WriteString(fmt.Sprintf("    style %s fill:#f9f,stroke:#333,stroke-width:2px,stroke-dasharray: 5 5\n", safeID))
		}
	}

	// Add edges
	sort.Strings(edges)
	if len(edges) > 0 {
		sb.WriteString("\n")
		for _, e := range edges {
			sb.WriteString(e + "\n")
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	// Replace invalid chars with underscore
	// Mermaid IDs must be alphanumeric or underscore
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

func writeHTML(path, title, mermaidContent string) error {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Architecture</title>
</head>
<body>
    <h1>{{.Title}}</h1>
    <div class="mermaid">
{{.Mermaid}}
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

	data := struct {
		Title   string
		Mermaid template.HTML
	}{
		Title:   title,
		Mermaid: template.HTML(mermaidContent),
	}

	return t.Execute(f, data)
}
