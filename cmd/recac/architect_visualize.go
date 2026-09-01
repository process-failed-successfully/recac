package main

import (
	"bytes"
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
	Short: "Visualize the system architecture",
	Long:  "Generates a Mermaid diagram from the architecture.yaml file.",
	RunE:  runArchitectVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().String("out", "", "Output file (default: stdout)")
	architectVisualizeCmd.Flags().Bool("html", false, "Generate standalone HTML file")
}

func runArchitectVisualizeCmd(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	out, _ := cmd.Flags().GetString("out")
	html, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(dir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture.yaml: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if html {
		tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>System Architecture - {{.SystemName}}</title>
</head>
<body>
    <div class="mermaid">
{{.Mermaid}}
    </div>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</body>
</html>`

		t, err := template.New("viz").Parse(tmpl)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		var buf bytes.Buffer
		data := struct {
			SystemName string
			Mermaid    template.HTML
		}{
			SystemName: arch.SystemName,
			Mermaid:    template.HTML(mermaid),
		}

		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}

		if out == "" {
			out = filepath.Join(dir, "architecture.html")
		}
		if err := os.WriteFile(out, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write HTML file: %w", err)
		}
		fmt.Printf("Generated HTML visualization at %s\n", out)
	} else {
		if out != "" {
			if err := os.WriteFile(out, []byte(mermaid), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Printf("Generated Mermaid diagram at %s\n", out)
		} else {
			fmt.Println(mermaid)
		}
	}

	return nil
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes
	for _, c := range arch.Components {
		id := sanitizeArchID(c.ID)
		label := c.ID
		// Escape label for Mermaid
		label = escapeMermaidLabel(label)

		switch strings.ToLower(c.Type) {
		case "database", "db":
			sb.WriteString(fmt.Sprintf("    %s[(\"%s\")]\n", id, label))
		case "worker":
			sb.WriteString(fmt.Sprintf("    %s{{ \"%s\" }}\n", id, label))
		case "queue":
			sb.WriteString(fmt.Sprintf("    %s> \"%s\" ]\n", id, label))
		case "frontend", "ui":
			sb.WriteString(fmt.Sprintf("    %s([\"%s\"])\n", id, label))
		default:
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}
	}

	// Edges
	for _, c := range arch.Components {
		cID := sanitizeArchID(c.ID)

		// Consumes
		for _, in := range c.Consumes {
			if in.Source != "" {
				srcID := sanitizeArchID(in.Source)
				desc := escapeMermaidLabel(in.Type)
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", srcID, desc, cID))
			}
		}

		// Produces
		for _, out := range c.Produces {
			if out.Target != "" {
				tgtID := sanitizeArchID(out.Target)
				desc := escapeMermaidLabel(out.Type)
				if out.Event != "" {
					desc = escapeMermaidLabel(out.Event)
				}
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", cID, desc, tgtID))
			}
		}
	}

	return sb.String()
}

func sanitizeArchID(id string) string {
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}

func escapeMermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "#quot;")
	s = strings.ReplaceAll(s, "<", "#lt;")
	s = strings.ReplaceAll(s, ">", "#gt;")
	return s
}
