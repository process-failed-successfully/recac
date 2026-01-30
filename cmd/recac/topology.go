package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var topologyCmd = &cobra.Command{
	Use:   "topology [file]",
	Short: "Visualize infrastructure topology from docker-compose.yml",
	Long:  `Parses a docker-compose.yml file and generates a Mermaid graph showing services, dependencies, and ports.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTopology,
}

func init() {
	rootCmd.AddCommand(topologyCmd)
}

func runTopology(cmd *cobra.Command, args []string) error {
	filename := "docker-compose.yml"
	if len(args) > 0 {
		filename = args[0]
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	var compose DockerCompose
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	graph := generateTopologyGraph(compose)
	fmt.Fprintln(cmd.OutOrStdout(), graph)
	return nil
}

type DockerCompose struct {
	Version  string             `yaml:"version"`
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Image       string      `yaml:"image"`
	Ports       []string    `yaml:"ports"`
	DependsOn   interface{} `yaml:"depends_on"`
	Links       []string    `yaml:"links"`
	Networks    interface{} `yaml:"networks"`
}

func generateTopologyGraph(compose DockerCompose) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Collect service names for deterministic order
	var serviceNames []string
	for name := range compose.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	for _, name := range serviceNames {
		svc := compose.Services[name]
		safeName := sanitizeMermaidIDTopology(name)

		// Label with ports if available
		label := name
		if len(svc.Ports) > 0 {
			// Clean up ports for display (e.g. "8080:80" -> "8080:80")
			label += fmt.Sprintf("<br/>(%s)", strings.Join(svc.Ports, ", "))
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeName, label))

		// Dependencies
		deps := parseDependsOn(svc.DependsOn)
		// Add links as deps
		for _, link := range svc.Links {
			// links can be "service" or "service:alias"
			parts := strings.Split(link, ":")
			deps = append(deps, parts[0])
		}

		// Deduplicate and sort deps
		uniqueDeps := make(map[string]bool)
		var sortedDeps []string
		for _, d := range deps {
			if !uniqueDeps[d] {
				uniqueDeps[d] = true
				sortedDeps = append(sortedDeps, d)
			}
		}
		sort.Strings(sortedDeps)

		for _, dep := range sortedDeps {
			safeDep := sanitizeMermaidIDTopology(dep)
			// Draw edge: Dependency -> Service (meaning Service depends on Dependency)
			// Wait, in Mermaid "A --> B" means arrow from A to B.
			// "Service depends on DB" usually means DB must start first.
			// Visualization: usually flow is "start order".
			// So DB --> Service.
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDep, safeName))
		}
	}

	return sb.String()
}

func parseDependsOn(dependsOn interface{}) []string {
	var deps []string
	if dependsOn == nil {
		return deps
	}

	// Try list of strings (simple form)
	if list, ok := dependsOn.([]interface{}); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps
	}

	// Try map (long form)
	if m, ok := dependsOn.(map[string]interface{}); ok {
		for k := range m {
			deps = append(deps, k)
		}
		return deps
	}

	return deps
}

func sanitizeMermaidIDTopology(id string) string {
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}
