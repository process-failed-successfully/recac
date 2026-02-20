package main

import (
	"fmt"
	"os"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize system architecture as a Mermaid diagram",
	Long: `Reads the generated architecture.yaml file and generates a Mermaid diagram
visualizing components and their interactions (Consumes/Produces).`,
	Run: runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("file", ".recac/architecture/architecture.yaml", "Path to architecture.yaml file")
	architectVisualizeCmd.Flags().String("out", "", "Output file path (default: stdout)")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate an HTML file with embedded Mermaid viewer")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) {
	file, _ := cmd.Flags().GetString("file")
	out, _ := cmd.Flags().GetString("out")
	html, _ := cmd.Flags().GetBool("html")

	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", file, err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture YAML: %v\n", err)
		os.Exit(1)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	var output string
	if html {
		output = generateHTML(mermaid)
	} else {
		output = mermaid
	}

	if out != "" {
		if err := os.WriteFile(out, []byte(output), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", out, err)
			os.Exit(1)
		}
		fmt.Printf("Diagram written to %s\n", out)
	} else {
		fmt.Println(output)
	}
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes
	for _, comp := range arch.Components {
		// Sanitize ID
		id := sanitizeArchID(comp.ID)

		shapeOpen := "["
		shapeClose := "]"
		compType := strings.ToLower(comp.Type)

		if strings.Contains(compType, "database") || strings.Contains(compType, "store") {
			shapeOpen = "[("
			shapeClose = ")]"
		} else if strings.Contains(compType, "worker") || strings.Contains(compType, "job") {
			shapeOpen = "("
			shapeClose = ")"
		} else if strings.Contains(compType, "front") || strings.Contains(compType, "ui") || strings.Contains(compType, "web") {
			shapeOpen = "{{"
			shapeClose = "}}"
		}

		// Escape quotes in ID and Type just in case
		displayID := strings.ReplaceAll(comp.ID, "\"", "'")
		displayType := strings.ReplaceAll(comp.Type, "\"", "'")

		sb.WriteString(fmt.Sprintf("    %s%s\"%s<br/>(%s)\"%s\n", id, shapeOpen, displayID, displayType, shapeClose))
	}

	// Edges
	for _, comp := range arch.Components {
		consumerID := sanitizeArchID(comp.ID)

		// Consumes (Input)
		for _, in := range comp.Consumes {
			sourceID := sanitizeArchID(in.Source)
			if sourceID == "" {
				sourceID = "External"
			}

			label := in.Type
			if label == "" {
				label = "consumes"
			}

			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", sourceID, label, consumerID))
		}

		// Produces (Output)
		for _, out := range comp.Produces {
			targetID := sanitizeArchID(out.Target)
			if targetID == "" {
				continue
			}

			label := out.Type
			if label == "" {
				label = out.Event
			}
			if label == "" {
				label = "produces"
			}

			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", consumerID, label, targetID))
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	if id == "" {
		return ""
	}
	// Replace invalid characters for Mermaid IDs
	safe := strings.ReplaceAll(id, "-", "_")
	safe = strings.ReplaceAll(safe, " ", "_")
	safe = strings.ReplaceAll(safe, ".", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	safe = strings.ReplaceAll(safe, "/", "_")
	return safe
}

func generateHTML(mermaid string) string {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>System Architecture</title>
</head>
<body>
    <pre class="mermaid">
%s
    </pre>
    <script type="module">
      import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
      mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>`

	return fmt.Sprintf(tmpl, mermaid)
}
