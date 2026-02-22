package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the architecture as a Mermaid diagram",
	Long:  `Generates a Mermaid diagram from the architecture.yaml file. Can output raw Mermaid text or a standalone HTML file.`,
	Run:   runArchitectVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("arch-file", ".recac/architecture/architecture.yaml", "Path to architecture.yaml file")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate an HTML file with embedded Mermaid")
	architectVisualizeCmd.Flags().String("out", "", "Output file path (default: stdout for text, architecture.html for HTML)")
}

func runArchitectVisualizeCmd(cmd *cobra.Command, args []string) {
	archFile, _ := cmd.Flags().GetString("arch-file")
	htmlMode, _ := cmd.Flags().GetBool("html")
	outFile, _ := cmd.Flags().GetString("out")

	// Read architecture file
	data, err := os.ReadFile(archFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading architecture file: %v\n", err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture YAML: %v\n", err)
		os.Exit(1)
	}

	// Generate Mermaid
	mermaid := generateMermaidFromArch(&arch)

	// Determine output content
	var outputContent string
	if htmlMode {
		var err error
		outputContent, err = generateHTMLFromMermaid(mermaid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating HTML: %v\n", err)
			os.Exit(1)
		}
		if outFile == "" {
			outFile = "architecture.html"
		}
	} else {
		outputContent = mermaid
	}

	// Write output
	if outFile != "" {
		if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(outFile, []byte(outputContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Visualization written to %s\n", outFile)
	} else {
		// Print to stdout
		fmt.Println(outputContent)
	}
}

// sanitizeArchitectureID creates a safe Mermaid ID from a string
func sanitizeArchitectureID(id string) string {
	// If ID contains special characters, hash it to be safe and unique-ish
	// But first try to just replace invalid chars with underscores to keep it readable
	reg, _ := regexp.Compile("[^a-zA-Z0-9_]")
	safe := reg.ReplaceAllString(id, "_")

	// If the ID started with a number, prefix it
	if len(safe) > 0 && safe[0] >= '0' && safe[0] <= '9' {
		safe = "id_" + safe
	}

	// Check for collisions or excessively long/messy IDs?
	// For now, let's use a hash if it's too weird, but usually simple replacement is enough.
	// Actually, let's append a short hash to ensure uniqueness if we mapped multiple things to same ID (unlikely with this regex)
	// But for visualization, readability is key.

	return safe
}

func generateMermaidFromArch(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Nodes (Components)
	// Sort components by ID for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	for _, comp := range arch.Components {
		id := sanitizeArchitectureID(comp.ID)

		// Use shapes based on type
		shapeStart := "["
		shapeEnd := "]"
		if comp.Type == "database" {
			shapeStart = "[("
			shapeEnd = ")]"
		} else if comp.Type == "queue" || comp.Type == "topic" || comp.Type == "event-bus" {
			shapeStart = "(["
			shapeEnd = "])"
		} else if comp.Type == "frontend" || comp.Type == "ui" {
			shapeStart = "{{"
			shapeEnd = "}}"
		}

		// Sanitize description for label (replace newlines with <br/>)
		// Also escape double quotes
		label := fmt.Sprintf("%s<br/>(%s)", comp.ID, comp.Type)
		label = strings.ReplaceAll(label, "\"", "'")

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", id, shapeStart, label, shapeEnd))
	}

	// 2. Edges
	// We need to track edges to avoid duplicates if possible

	for _, comp := range arch.Components {
		targetID := sanitizeArchitectureID(comp.ID)

		// Inputs (Consumes): Source -> comp.ID
		for _, input := range comp.Consumes {
			if input.Source != "" {
				sourceID := sanitizeArchitectureID(input.Source)

				// Sanitize label
				label := input.Type
				if label == "" {
					label = "consumes"
				}
				label = strings.ReplaceAll(label, "\"", "'")

				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, label, targetID))
			}
		}

		// Outputs (Produces): comp.ID -> Target
		for _, output := range comp.Produces {
			if output.Target != "" {
				destID := sanitizeArchitectureID(output.Target)

				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "produces"
				}
				label = strings.ReplaceAll(label, "\"", "'")

				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", targetID, label, destID))
			}
		}
	}

	return sb.String()
}

func generateHTMLFromMermaid(mermaid string) (string, error) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Architecture Visualization</title>
</head>
<body>
    <h1>System Architecture</h1>
    <div class="mermaid">
{{ .Mermaid }}
    </div>
    <script type="module">
      import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
      mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>`

	data := struct {
		Mermaid template.HTML
	}{
		Mermaid: template.HTML(mermaid),
	}

	t, err := template.New("webpage").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", err
	}

	return sb.String(), nil
}
