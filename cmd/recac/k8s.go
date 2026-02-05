package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	k8sOutputDir string
	k8sPort      string
	k8sReplicas  int
	k8sHelm      bool
	k8sNamespace string
)

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Generate Kubernetes manifests or Helm charts",
	Long: `Analyzes the project and uses AI to generate production-ready Kubernetes manifests
(Deployment, Service, Ingress) or a Helm Chart.

It automatically detects Docker configuration if a Dockerfile exists.`,
	RunE: runK8s,
}

func init() {
	rootCmd.AddCommand(k8sCmd)
	k8sCmd.Flags().StringVarP(&k8sOutputDir, "output", "o", "k8s", "Output directory for generated files")
	k8sCmd.Flags().StringVar(&k8sPort, "port", "", "Container port (default: auto-detected)")
	k8sCmd.Flags().IntVar(&k8sReplicas, "replicas", 2, "Number of replicas")
	k8sCmd.Flags().BoolVar(&k8sHelm, "helm", false, "Generate a Helm chart instead of raw manifests")
	k8sCmd.Flags().StringVarP(&k8sNamespace, "namespace", "n", "default", "Target namespace")
}

func runK8s(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather Context
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing project for K8s generation...")

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Project Structure:\n")

	// Check for Dockerfile
	dockerfileContent := ""
	if content, err := os.ReadFile("Dockerfile"); err == nil {
		dockerfileContent = string(content)
		contextBuilder.WriteString("- Found Dockerfile\n")
		fmt.Fprintln(cmd.OutOrStdout(), "- Found Dockerfile")
		// Simple heuristic for port
		if k8sPort == "" {
			re := regexp.MustCompile(`EXPOSE\s+(\d+)`)
			match := re.FindStringSubmatch(dockerfileContent)
			if len(match) > 1 {
				k8sPort = match[1]
				fmt.Fprintf(cmd.OutOrStdout(), "   Detected port: %s (from Dockerfile)\n", k8sPort)
			}
		}
	} else {
		contextBuilder.WriteString("- No Dockerfile found (Agent will assume standard setup)\n")
	}

	// 2. Prepare Prompt
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-k8s")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	projectName := filepath.Base(cwd)

	prompt := fmt.Sprintf(`You are a Kubernetes and DevOps expert.
Generate configuration files to deploy the project "%s" to Kubernetes.
`, projectName)

	if k8sHelm {
		prompt += "Generate a complete Helm Chart structure.\n"
		prompt += "Include: Chart.yaml, values.yaml, templates/deployment.yaml, templates/service.yaml, templates/ingress.yaml\n"
	} else {
		prompt += "Generate raw Kubernetes manifests.\n"
		prompt += "Include: deployment.yaml, service.yaml, ingress.yaml\n"
	}

	prompt += "\nConfiguration:\n"
	prompt += fmt.Sprintf("- Replicas: %d\n", k8sReplicas)
	prompt += fmt.Sprintf("- Namespace: %s\n", k8sNamespace)
	if k8sPort != "" {
		prompt += fmt.Sprintf("- Container Port: %s\n", k8sPort)
	}

	prompt += "\n" + contextBuilder.String() + "\n"

	if dockerfileContent != "" {
		prompt += "\nDockerfile Content (for reference):\n" + dockerfileContent + "\n"
	}

	prompt += `
IMPORTANT: Output the content of each file wrapped in XML tags like this:
<file path="deployment.yaml">
... content ...
</file>
<file path="values.yaml">
... content ...
</file>

For Helm, use relative paths like "templates/deployment.yaml".
Do not use markdown code blocks inside the XML tags.
Output ONLY the XML structure.
`

	// 3. Send to Agent
	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating K8s assets...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Parse Response
	files := parseK8sXMLFiles(resp)
	if len(files) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "❌ Could not parse files from agent response.")
		// Log raw response for debugging?
		return fmt.Errorf("failed to parse agent response")
	}

	// 5. Write Files
	if err := os.MkdirAll(k8sOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for path, content := range files {
		// Join with output dir
		fullPath := filepath.Join(k8sOutputDir, path)

		// Security check: Prevent path traversal
		absOut, err := filepath.Abs(k8sOutputDir)
		if err != nil {
			return fmt.Errorf("failed to resolve output dir: %w", err)
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		}
		if !strings.HasPrefix(absPath, absOut) {
			return fmt.Errorf("illegal file path (outside output dir): %s", path)
		}

		// Create subdir if needed
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create dir for %s: %w", path, err)
		}

		// Clean content
		content = strings.TrimSpace(content)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fullPath, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Created %s\n", fullPath)
	}

	return nil
}

func parseK8sXMLFiles(text string) map[string]string {
	result := make(map[string]string)
	// Regex to match <file path="...">...</file>
	// (?s) enables dot matching newline
	re := regexp.MustCompile(`(?s)<file\s+path="([^"]+)">\s*(.*?)\s*</file>`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) == 3 {
			path := match[1]
			content := match[2]
			result[path] = content
		}
	}
	return result
}
