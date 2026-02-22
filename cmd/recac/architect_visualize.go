package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the architecture as a Mermaid diagram",
	Long:  "Generates a Mermaid flowchart from the architecture.yaml file. Can output raw Mermaid syntax or an HTML file.",
	RunE:  runVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().String("file", ".recac/architecture/architecture.yaml", "Path to architecture.yaml file")
	visualizeCmd.Flags().Bool("html", false, "Generate an HTML file with embedded Mermaid diagram")
}

func runVisualizeCmd(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	html, _ := cmd.Flags().GetBool("html")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("error parsing YAML: %w", err)
	}

	mermaid := generateArchitectureMermaid(&arch)

	if html {
		htmlContent := generateHTML(mermaid)
		// Output to same directory as input file, but with .html extension
		outFile := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".html"
		if err := os.WriteFile(outFile, []byte(htmlContent), 0644); err != nil {
			return fmt.Errorf("error writing HTML file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Architecture visualization saved to %s\n", outFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	}
	return nil
}

func generateArchitectureMermaid(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Nodes
	for _, c := range arch.Components {
		safeID := cleanArchID(c.ID)
		shapeStart, shapeEnd := "[", "]"

		switch strings.ToLower(c.Type) {
		case "database":
			shapeStart, shapeEnd = "[(", ")]"
		case "worker":
			shapeStart, shapeEnd = "([", "])"
		case "queue":
			shapeStart, shapeEnd = "{{", "}}"
		case "frontend":
			shapeStart, shapeEnd = "((", "))"
		case "service":
			shapeStart, shapeEnd = "[", "]"
		}

		// Use ID as label
		sb.WriteString(fmt.Sprintf("    %s%s%s%s\n", safeID, shapeStart, c.ID, shapeEnd))
	}

	sb.WriteString("\n")

	// 2. Edges
	// Consumes: Source -> Component
	for _, c := range arch.Components {
		safeID := cleanArchID(c.ID)
		for _, input := range c.Consumes {
			if input.Source != "" {
				sourceID := cleanArchID(input.Source)
				label := input.Type
				if label == "" {
					label = "consumes"
				}
				sb.WriteString(fmt.Sprintf("    %s -- %s --> %s\n", sourceID, label, safeID))
			}
		}
	}

	// Produces: Component -> Target
	for _, c := range arch.Components {
		safeID := cleanArchID(c.ID)
		for _, output := range c.Produces {
			if output.Target != "" {
				targetID := cleanArchID(output.Target)
				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "produces"
				}
				sb.WriteString(fmt.Sprintf("    %s -- %s --> %s\n", safeID, label, targetID))
			}
		}
	}

	return sb.String()
}

func cleanArchID(id string) string {
	// Replace characters that might break Mermaid ID syntax
	r := strings.NewReplacer("-", "_", " ", "_", ".", "_", "/", "_", ":", "_")
	return r.Replace(id)
}

func generateHTML(mermaid string) string {
	const htmlTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Architecture Visualization</title>
</head>
<body>
    <div class="mermaid">
MERMAID_CONTENT
    </div>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>`
	return strings.Replace(htmlTmpl, "MERMAID_CONTENT", mermaid, 1)
}
