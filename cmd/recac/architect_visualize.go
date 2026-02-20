package main

import (
	"fmt"
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
	Short: "Visualize the generated system architecture",
	Long:  `Generates a Mermaid diagram from the architecture.yaml file created by 'recac architect'.`,
	Run:   runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("out", ".recac/architecture", "Directory containing architecture.yaml")
	architectVisualizeCmd.Flags().Bool("html", false, "Output a standalone HTML file instead of raw Mermaid code")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) {
	outDir, _ := cmd.Flags().GetString("out")
	html, _ := cmd.Flags().GetBool("html")

	archPath := filepath.Join(outDir, "architecture.yaml")
	data, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	mermaid := generateMermaidSystemArchitecture(&arch)

	if html {
		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Architecture Visualization</title>
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
</html>`, mermaid)
		fmt.Println(htmlContent)
	} else {
		fmt.Println(mermaid)
	}
}

func generateMermaidSystemArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	sanitizeArchID := func(id string) string {
		re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
		safe := re.ReplaceAllString(id, "_")
		if safe == "" {
			return "unknown"
		}
		if safe[0] >= '0' && safe[0] <= '9' {
			return "n" + safe
		}
		return safe
	}

	// 1. Define Nodes
	// Sort components by ID to ensure deterministic output
	// Copy to avoid modifying original order if that matters (it doesn't usually)
	sortedComps := make([]architecture.Component, len(arch.Components))
	copy(sortedComps, arch.Components)
	sort.Slice(sortedComps, func(i, j int) bool {
		return sortedComps[i].ID < sortedComps[j].ID
	})

	for _, comp := range sortedComps {
		safeID := sanitizeArchID(comp.ID)
		label := comp.ID
		// Basic shape logic
		shapeOpen := "["
		shapeClose := "]"

		switch strings.ToLower(comp.Type) {
		case "database", "db", "store", "redis", "postgres", "mysql":
			shapeOpen = "[("
			shapeClose = ")]"
		case "worker", "cron", "job":
			shapeOpen = "{{"
			shapeClose = "}}"
		case "queue", "topic", "kafka", "rabbitmq":
			shapeOpen = ">"
			shapeClose = "]"
		case "frontend", "ui", "web":
			shapeOpen = "("
			shapeClose = ")"
		}

		// Escape label if needed
		label = strings.ReplaceAll(label, "\"", "'")
		sb.WriteString(fmt.Sprintf("    %s%s\"%s<br/>(%s)\"%s\n", safeID, shapeOpen, label, comp.Type, shapeClose))
	}

	// 2. Collect Edges
	// Map "Source->Target" to list of labels
	edgeLabels := make(map[string][]string)

	for _, comp := range sortedComps {
		safeID := sanitizeArchID(comp.ID)

		// Consumes: Source -> Component
		for _, input := range comp.Consumes {
			if input.Source != "" {
				srcSafe := sanitizeArchID(input.Source)
				key := fmt.Sprintf("%s->%s", srcSafe, safeID)

				label := input.Type
				if label == "" {
					label = "uses"
				}
				edgeLabels[key] = append(edgeLabels[key], label)
			}
		}

		// Produces: Component -> Target
		for _, output := range comp.Produces {
			if output.Target != "" {
				tgtSafe := sanitizeArchID(output.Target)
				key := fmt.Sprintf("%s->%s", safeID, tgtSafe)

				label := output.Type
				if output.Event != "" {
					label = "Event: " + output.Event
				}
				if label == "" {
					label = "produces"
				}
				edgeLabels[key] = append(edgeLabels[key], label)
			}
		}
	}

	// 3. Write Edges
	var edgeKeys []string
	for k := range edgeLabels {
		edgeKeys = append(edgeKeys, k)
	}
	sort.Strings(edgeKeys)

	for _, k := range edgeKeys {
		parts := strings.Split(k, "->")
		src, tgt := parts[0], parts[1]

		// Dedup labels
		labels := edgeLabels[k]
		uniqueLabels := make([]string, 0, len(labels))
		seen := make(map[string]bool)
		for _, l := range labels {
			if !seen[l] {
				seen[l] = true
				uniqueLabels = append(uniqueLabels, l)
			}
		}
		sort.Strings(uniqueLabels)

		fullLabel := strings.Join(uniqueLabels, ", ")
		sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", src, fullLabel, tgt))
	}

	return sb.String()
}
