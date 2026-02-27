package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize [path-to-architecture.yaml]",
	Short: "Visualize the system architecture in a browser",
	Long:  `Generates an interactive Mermaid.js diagram from the architecture.yaml file and opens it in your default browser.`,
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
}

// listenAndServeFunc mocks http.ListenAndServe
var listenAndServeFunc = http.ListenAndServe

// openBrowserFunc mocks opening the browser
var openBrowserFunc = openBrowserForVis

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	path := ".recac/architecture/architecture.yaml"
	if len(args) > 0 {
		path = args[0]
	}

	// 1. Read Architecture File
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read architecture file at %s: %w", path, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture YAML: %w", err)
	}

	// 2. Generate Mermaid Diagram
	mermaidGraph := generateArchMermaid(&arch)

	// 3. Serve HTML
	mux := setupVisualizeServer(mermaidGraph)

	port := ":8080"
	url := "http://localhost" + port
	fmt.Fprintf(cmd.OutOrStdout(), "Serving architecture visualization at %s\n", url)

	// Start server in a goroutine so we can open the browser
	go func() {
		if err := openBrowserFunc(url); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to open browser: %v\n", err)
		}
	}()

	return listenAndServeFunc(port, mux)
}

func setupVisualizeServer(mermaidGraph string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("index").Parse(htmlTemplate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.Execute(w, map[string]string{"Graph": mermaidGraph}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	return mux
}

func generateArchMermaid(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")

	// Add Components as nodes
	for _, c := range arch.Components {
		// Clean ID for Mermaid compatibility
		safeID := cleanArchID(c.ID)
		desc := c.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}

		// Use different shapes based on type
		shapeStart, shapeEnd := "[", "]"
		switch c.Type {
		case "database":
			shapeStart, shapeEnd = "[(", ")]"
		case "queue", "topic":
			shapeStart, shapeEnd = "([", "])"
		case "service":
			shapeStart, shapeEnd = "(", ")"
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s<br/><small>%s</small>\"%s\n", safeID, shapeStart, c.ID, desc, shapeEnd))
	}

	// Add Edges based on Consumes (Input) -> Component
	// and Component -> Produces (Output, if target specified)

	// Track added edges to avoid duplicates if defined on both ends
	edges := make(map[string]bool)

	for _, c := range arch.Components {
		targetID := cleanArchID(c.ID)

		// Inputs: Source -> Current
		for _, input := range c.Consumes {
			if input.Source != "" {
				sourceID := cleanArchID(input.Source)
				edgeKey := fmt.Sprintf("%s->%s:%s", sourceID, targetID, input.Type)
				if !edges[edgeKey] {
					sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, input.Type, targetID))
					edges[edgeKey] = true
				}
			}
		}

		// Outputs: Current -> Target (if explicit)
		for _, output := range c.Produces {
			if output.Target != "" {
				destID := cleanArchID(output.Target)
				// Type or Event
				label := output.Type
				if label == "" {
					label = output.Event
				}

				edgeKey := fmt.Sprintf("%s->%s:%s", targetID, destID, label)
				if !edges[edgeKey] {
					sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", targetID, label, destID))
					edges[edgeKey] = true
				}
			}
		}
	}

	return sb.String()
}

func cleanArchID(id string) string {
	return strings.ReplaceAll(strings.ReplaceAll(id, "-", "_"), " ", "_")
}

func openBrowserForVis(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return execCommand(cmd, args...).Start()
}

const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>RECAC Architecture Visualization</title>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script>
        mermaid.initialize({ startOnLoad: true, theme: 'default' });
    </script>
    <style>
        body { font-family: sans-serif; margin: 0; padding: 20px; display: flex; flex-direction: column; align-items: center; }
        .mermaid { width: 100%; height: 100%; }
    </style>
</head>
<body>
    <h1>System Architecture</h1>
    <div class="mermaid">
        {{.Graph}}
    </div>
</body>
</html>
`
