package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the generated system architecture",
	Long:  "Generates a Mermaid diagram from the architecture.yaml file.",
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate an HTML file with the diagram")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	htmlOutput, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture file: %w", err)
	}

	mermaid := generateArchMermaid(arch)

	if htmlOutput {
		htmlPath := filepath.Join(dir, "architecture.html")
		if err := writeHTML(htmlPath, mermaid); err != nil {
			return fmt.Errorf("failed to write HTML file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Architecture diagram saved to %s\n", htmlPath)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	}

	return nil
}

func generateArchMermaid(arch architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")

	// Helper to get a safe, unique ID for Mermaid nodes
	getSafeID := func(id string) string {
		h := sha256.New()
		h.Write([]byte(id))
		// Prefix with 'N' to ensure it starts with a letter
		return "N" + hex.EncodeToString(h.Sum(nil))[:8]
	}

	// Helper to escape double quotes in labels
	escapeLabel := func(s string) string {
		return strings.ReplaceAll(s, "\"", "'")
	}

	// 1. Define Nodes (Components)
	for _, comp := range arch.Components {
		id := getSafeID(comp.ID)
		label := escapeLabel(comp.ID)
		if comp.Type != "" {
			label += fmt.Sprintf("<br/>(%s)", escapeLabel(comp.Type))
		}
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
	}

	// 2. Define Edges (Consumes/Produces)
	for _, comp := range arch.Components {
		targetId := getSafeID(comp.ID)

		// Consumes: Source -> Component
		for _, input := range comp.Consumes {
			if input.Source == "" {
				continue
			}
			sourceId := getSafeID(input.Source)
			label := escapeLabel(input.Type)
			if label == "" {
				label = "uses"
			}
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceId, label, targetId))
		}

		// Produces: Component -> Target
		for _, output := range comp.Produces {
			if output.Target == "" {
				continue
			}
			destId := getSafeID(output.Target)
			label := output.Event
			if label == "" {
				label = output.Type
			}
			if label == "" {
				label = "notifies"
			}
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", targetId, escapeLabel(label), destId))
		}
	}

	return sb.String()
}

func writeHTML(path, mermaidContent string) error {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Architecture Diagram</title>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</head>
<body>
    <h1>System Architecture</h1>
    <div class="mermaid">
        {{.}}
    </div>
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

	// Use template.HTML to prevent escaping of arrow syntax (-->)
	return t.Execute(f, template.HTML(mermaidContent))
}
