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
	Use:   "topology",
	Short: "Visualize infrastructure topology from docker-compose",
	Long:  `Parses a docker-compose file and generates a Mermaid diagram showing services and their dependencies.`,
	RunE:  runTopology,
}

var topologyFile string

func init() {
	rootCmd.AddCommand(topologyCmd)
	topologyCmd.Flags().StringVarP(&topologyFile, "file", "f", "docker-compose.yml", "Path to docker-compose file")
}

// Minimal struct to capture what we need
type ComposeFile struct {
	Version  string             `yaml:"version"`
	Services map[string]Service `yaml:"services"`
	Networks map[string]interface{} `yaml:"networks"`
}

type Service struct {
	Image       string                 `yaml:"image"`
	DependsOn   interface{}            `yaml:"depends_on"` // Can be []string or map[string]Condition
	Links       []string               `yaml:"links"`
	Environment interface{}            `yaml:"environment"` // Can be []string or map[string]string
	Networks    interface{}            `yaml:"networks"`    // Can be []string or map[string]interface{}
}

func runTopology(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(topologyFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", topologyFile, err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(content, &compose); err != nil {
		return fmt.Errorf("failed to parse yaml: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), generateMermaidTopology(compose))
	return nil
}

func generateMermaidTopology(compose ComposeFile) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Collect service names and sort for determinism
	var services []string
	for name := range compose.Services {
		services = append(services, name)
	}
	sort.Strings(services)

	for _, name := range services {
		svc := compose.Services[name]
		// Determine dependencies
		deps := getDependencies(svc)

		// Sort dependencies
		sort.Strings(deps)

		// If no dependencies, still output the node? Yes.
		// Use sanitized ID
		safeName := sanitizeMermaidID(name)

		// Label with image if available? Maybe too cluttered.
		// Just service name is fine.

		// Check if we have deps
		if len(deps) == 0 {
			// Just verify it exists in graph by referencing itself?
			// Mermaid implicitly creates nodes if they are mentioned.
			// But if it has no connections, we should list it explicitly?
			// "A" is valid mermaid.
			sb.WriteString(fmt.Sprintf("    %s\n", safeName))
		}

		for _, dep := range deps {
			// Ensure dep exists in services? Usually yes, but might be external.
			safeDep := sanitizeMermaidID(dep)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeName, safeDep))
		}
	}

	return sb.String()
}

func getDependencies(svc Service) []string {
	depsMap := make(map[string]bool)

	// 1. depends_on
	if svc.DependsOn != nil {
		// Try list
		if list, ok := svc.DependsOn.([]interface{}); ok {
			for _, item := range list {
				if s, ok := item.(string); ok {
					depsMap[s] = true
				}
			}
		} else if m, ok := svc.DependsOn.(map[string]interface{}); ok {
			for dep := range m {
				depsMap[dep] = true
			}
		}
	}

	// 2. links
	for _, link := range svc.Links {
		// Link might be "service:alias"
		parts := strings.Split(link, ":")
		depsMap[parts[0]] = true
	}

	// 3. Environment inference (Heuristic)
	// Check for env vars that match other service names
	// This requires knowing all service names, but here we process per service.
	// We can check value strings.

	// Simplify: Just return what we found in explicit deps.
	// Inference is risky without access to the full list of services here.

	var deps []string
	for d := range depsMap {
		deps = append(deps, d)
	}
	return deps
}
