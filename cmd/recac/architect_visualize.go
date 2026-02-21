package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Global flags (can be accessed by run function)
var (
	visualizeDir  string
	visualizeHTML bool
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the generated system architecture as a Mermaid diagram",
	Long: `Reads architecture.yaml from the specified directory and generates a Mermaid diagram.
Can output as a standalone HTML file for easy viewing.

Example:
  recac architect visualize --dir .recac/architecture --html
`,
	Run: runArchitectVisualize,
}

func init() {
	// Add to parent command
	if architectCmd != nil {
		architectCmd.AddCommand(architectVisualizeCmd)
	}

	// Flags
	architectVisualizeCmd.Flags().StringVar(&visualizeDir, "dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().BoolVar(&visualizeHTML, "html", false, "Generate HTML output")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) {
	// 1. Read architecture.yaml
	archPath := filepath.Join(visualizeDir, "architecture.yaml")
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

	// 2. Generate Mermaid
	mermaid := generateMermaidSystemArchitecture(&arch)

	// 3. Output
	if visualizeHTML {
		html := generateHTML(mermaid)
		outPath := filepath.Join(visualizeDir, "architecture.html")
		if err := os.WriteFile(outPath, []byte(html), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML to %s: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Printf("✅ Generated architecture diagram: %s\n", outPath)
	} else {
		outPath := filepath.Join(visualizeDir, "architecture.mmd")
		if err := os.WriteFile(outPath, []byte(mermaid), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing Mermaid to %s: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Printf("✅ Generated Mermaid diagram: %s\n", outPath)
		// Also print to stdout if it's small? No, keep it clean.
	}
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes
	for _, comp := range arch.Components {
		id := sanitizeArchID(comp.ID)
		shapeStart, shapeEnd := getShape(comp.Type)

		// Truncate description for label if too long
		desc := comp.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		label := escapeMermaidLabel(comp.ID)
		if desc != "" {
			label += "<br/>" + escapeMermaidLabel(desc)
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", id, shapeStart, label, shapeEnd))
	}

	// Edges
	for _, comp := range arch.Components {
		targetID := sanitizeArchID(comp.ID)

		// Consumes (Input)
		for _, input := range comp.Consumes {
			sourceID := sanitizeArchID(input.Source)
			if sourceID != "" {
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, escapeMermaidLabel(input.Type), targetID))
			}
		}

		// Produces (Output)
		for _, output := range comp.Produces {
			if output.Target != "" {
				destID := sanitizeArchID(output.Target)
				eventName := output.Event
				if eventName == "" {
					eventName = output.Type
				}
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", targetID, escapeMermaidLabel(eventName), destID))
			}
		}
	}

	return sb.String()
}

func getShape(compType string) (string, string) {
	switch strings.ToLower(compType) {
	case "database", "db":
		return "[(", ")]"
	case "worker":
		return "{{", "}}"
	case "queue":
		return ">", "]"
	case "frontend", "ui":
		return "(", ")"
	default:
		return "[", "]" // Service/Default
	}
}

var idSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeArchID(id string) string {
	// Replace non-alphanumeric chars with underscore
	return idSanitizer.ReplaceAllString(id, "_")
}

func escapeMermaidLabel(label string) string {
	label = strings.ReplaceAll(label, "\"", "#quot;")
	label = strings.ReplaceAll(label, "<", "#lt;")
	label = strings.ReplaceAll(label, ">", "#gt;")
	return label
}

func generateHTML(mermaid string) string {
	const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>System Architecture</title>
</head>
<body>
    <div class="mermaid">
%s
    </div>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>`
	return fmt.Sprintf(htmlTemplate, mermaid)
}
