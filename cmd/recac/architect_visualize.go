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
	Long:  "Generates a Mermaid graph from the architecture.yaml file.",
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate HTML file")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	html, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture file: %w", err)
	}

	mermaidGraph := generateMermaidGraph(&arch)

	if html {
		htmlPath := filepath.Join(dir, "architecture.html")
		if err := generateHTML(mermaidGraph, htmlPath); err != nil {
			return fmt.Errorf("failed to generate HTML: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Generated HTML visualization at %s\n", htmlPath)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaidGraph)
	}

	return nil
}

func generateMermaidGraph(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes
	for _, comp := range arch.Components {
		// Sanitize ID for Mermaid
		safeID := sanitizeArchID(comp.ID)
		sb.WriteString(fmt.Sprintf("    %s[\"%s<br/>(%s)\"]\n", safeID, comp.ID, comp.Type))
	}

	// Edges
	for _, comp := range arch.Components {
		safeID := sanitizeArchID(comp.ID)

		// Consumes (Source -> Comp)
		for _, input := range comp.Consumes {
			if input.Source != "" {
				sourceID := sanitizeArchID(input.Source)
				label := input.Type
				if label == "" {
					label = "Consumes"
				}
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, label, safeID))
			}
		}

		// Produces (Comp -> Target)
		for _, output := range comp.Produces {
			if output.Target != "" {
				targetID := sanitizeArchID(output.Target)
				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "Produces"
				}
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", safeID, label, targetID))
			}
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	// Replace non-alphanumeric chars with underscore
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "/", "_")
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	return re.ReplaceAllString(id, "")
}

func generateHTML(mermaidGraph, outputPath string) error {
	const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Architecture Visualization</title>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
</head>
<body>
    <h1>System Architecture</h1>
    <div class="mermaid">
        {{.}}
    </div>
    <script>
        mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>
`
	tmpl, err := template.New("arch").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Safe HTML template execution
	return tmpl.Execute(f, template.HTML(mermaidGraph))
}
