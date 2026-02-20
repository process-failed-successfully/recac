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

var visualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the system architecture",
	Long:  "Generates a Mermaid diagram from the architecture.yaml file.",
	Run:   runVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	visualizeCmd.Flags().String("out", "", "Output file path (default: stdout or architecture.html if --html)")
	visualizeCmd.Flags().Bool("html", false, "Generate HTML output with embedded Mermaid diagram")
}

func runVisualizeCmd(cmd *cobra.Command, args []string) {
	dir, _ := cmd.Flags().GetString("dir")
	out, _ := cmd.Flags().GetString("out")
	html, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	archData, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(archData, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	outputContent := mermaid
	if html {
		outputContent = generateHTML(mermaid)
		if out == "" {
			out = "architecture.html"
		}
	} else {
		if out == "" {
			// Write to stdout if no file specified
			fmt.Println(outputContent)
			return
		}
	}

	if err := os.WriteFile(out, []byte(outputContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote visualization to %s\n", out)
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Map to keep track of created nodes to avoid duplicates if referenced multiple times (though components should be unique)
	// Also to map original ID to sanitized ID
	idMap := make(map[string]string)

	for _, comp := range arch.Components {
		sanitizedID := sanitizeArchID(comp.ID)
		idMap[comp.ID] = sanitizedID

		// Determine shape based on type
		shapeStart := "["
		shapeEnd := "]"

		switch strings.ToLower(comp.Type) {
		case "database", "db":
			shapeStart = "[("
			shapeEnd = ")]"
		case "worker":
			shapeStart = "{{"
			shapeEnd = "}}"
		case "queue", "topic":
			shapeStart = ">"
			shapeEnd = "]"
		case "frontend", "ui":
			shapeStart = "("
			shapeEnd = ")"
		}

		// Label: ID + Type + Description (if short)
		// We use escapeMermaidLabel to handle special characters for Mermaid
		label := fmt.Sprintf("%s<br/>(%s)", escapeMermaidLabel(comp.ID), escapeMermaidLabel(comp.Type))
		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", sanitizedID, shapeStart, label, shapeEnd))
	}

	// Edges
	for _, comp := range arch.Components {
		sanitizedID := idMap[comp.ID]

		// Consumes
		for _, input := range comp.Consumes {
			sourceID, ok := idMap[input.Source]
			if !ok {
				// If source is external or not defined in components, create a node for it?
				// For now, let's create a sanitized ID for it on the fly, assuming it's an external system
				sourceID = sanitizeArchID(input.Source)
				sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", sourceID, escapeMermaidLabel(input.Source)))
				idMap[input.Source] = sourceID
			}
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, escapeMermaidLabel(input.Type), sanitizedID))
		}

		// Produces (if explicit target)
		for _, output := range comp.Produces {
			if output.Target != "" {
				targetID, ok := idMap[output.Target]
				if !ok {
					targetID = sanitizeArchID(output.Target)
					sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", targetID, escapeMermaidLabel(output.Target)))
					idMap[output.Target] = targetID
				}
				label := output.Event
				if label == "" {
					label = output.Type
				}
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sanitizedID, escapeMermaidLabel(label), targetID))
			}
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	// Replace non-alphanumeric characters with underscores
	reg := regexp.MustCompile("[^a-zA-Z0-9_]")
	return reg.ReplaceAllString(id, "_")
}

// escapeMermaidLabel escapes special characters for Mermaid diagram labels.
func escapeMermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "#quot;")
	s = strings.ReplaceAll(s, "<", "#lt;")
	s = strings.ReplaceAll(s, ">", "#gt;")
	// Mermaid treats | as a separator in some contexts, safer to escape it
	// although inside quotes it's usually fine, but let's be safe
	// #124; is the decimal entity for |
	s = strings.ReplaceAll(s, "|", "#124;")
	return s
}

func generateHTML(mermaid string) string {
	const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>System Architecture</title>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</head>
<body>
    <div class="mermaid">
{{.Mermaid}}
    </div>
</body>
</html>`

	t, err := template.New("webpage").Parse(htmlTemplate)
	if err != nil {
		return fmt.Sprintf("Error creating template: %v", err)
	}

	data := struct {
		Mermaid string
	}{
		Mermaid: mermaid,
	}

	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return fmt.Sprintf("Error executing template: %v", err)
	}

	return sb.String()
}
