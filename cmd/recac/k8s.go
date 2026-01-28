package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	k8sLabelSelector string
	k8sLogFollow     bool
	k8sOutputDir     string
	k8sForce         bool
)

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Manage Kubernetes resources",
	Long:  `Utilities for interacting with the Kubernetes cluster, viewing agents, and logs.`,
}

var k8sInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show cluster information",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := k8sClientFactory()
		if err != nil {
			return fmt.Errorf("failed to connect to kubernetes: %w", err)
		}

		version, err := client.GetServerVersion()
		if err != nil {
			return fmt.Errorf("failed to get server version: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Kubernetes Version: %s\n", version.String())
		fmt.Fprintf(cmd.OutOrStdout(), "Namespace: %s\n", client.GetNamespace())
		return nil
	},
}

var k8sAgentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List agent pods",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := k8sClientFactory()
		if err != nil {
			return fmt.Errorf("failed to connect to kubernetes: %w", err)
		}

		pods, err := client.ListPods(cmd.Context(), k8sLabelSelector)
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}

		if len(pods) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No agent pods found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Found %d agents:\n", len(pods))
		for _, pod := range pods {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s (%s) [%s]\n", pod.Name, pod.Status.Phase, pod.Status.PodIP)
		}
		return nil
	},
}

var k8sLogsCmd = &cobra.Command{
	Use:   "logs [pod_name]",
	Short: "Stream logs from a pod",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := k8sClientFactory()
		if err != nil {
			return fmt.Errorf("failed to connect to kubernetes: %w", err)
		}

		podName := args[0]
		stream, err := client.GetPodLogs(cmd.Context(), podName, k8sLogFollow)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}
		defer stream.Close()

		_, err = io.Copy(cmd.OutOrStdout(), stream)
		return err
	},
}

var k8sGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Kubernetes manifests",
	Long:  `Analyzes the project structure and uses AI to generate Deployment, Service, and Ingress manifests.`,
	RunE:  runK8sGenerate,
}

func init() {
	rootCmd.AddCommand(k8sCmd)
	k8sCmd.AddCommand(k8sInfoCmd)
	k8sCmd.AddCommand(k8sAgentsCmd)
	k8sCmd.AddCommand(k8sLogsCmd)
	k8sCmd.AddCommand(k8sGenerateCmd)

	k8sAgentsCmd.Flags().StringVarP(&k8sLabelSelector, "selector", "l", "app=recac-agent", "Label selector to filter pods")
	k8sLogsCmd.Flags().BoolVarP(&k8sLogFollow, "follow", "f", false, "Follow log stream")

	k8sGenerateCmd.Flags().StringVarP(&k8sOutputDir, "output-dir", "o", "deploy/k8s", "Directory to write generated files")
	k8sGenerateCmd.Flags().BoolVarP(&k8sForce, "force", "f", false, "Overwrite existing files")
}

func runK8sGenerate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather Context
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing project structure...")

	var contextBuilder strings.Builder
	contextBuilder.WriteString("File Tree:\n")
	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(cwd, path)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		contextBuilder.WriteString("- " + rel + "\n")
		return nil
	})

	// Include Dockerfile or docker-compose.yml if they exist
	for _, f := range []string{"Dockerfile", "docker-compose.yml", "go.mod", "package.json"} {
		if content, err := os.ReadFile(f); err == nil {
			contextBuilder.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", f, string(content)))
		}
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-k8s-generate")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a Kubernetes and DevOps expert.
Generate the following Kubernetes manifests for this project:
1. deployment.yaml
2. service.yaml
3. ingress.yaml (optional, if relevant)

Assume the image name is "myapp:latest" (or infer from context).
Use standard best practices (liveness probes, resources requests/limits).

IMPORTANT: Output the content of each file wrapped in XML tags like this:
<file path="deployment.yaml">
... content ...
</file>

Output ONLY the XML structure.

PROJECT CONTEXT:
%s`, contextBuilder.String())

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating Kubernetes manifests...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 3. Parse and Write
	// parseXMLFiles is available from main package (containerize.go)
	files := parseXMLFiles(resp)
	if len(files) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "❌ Could not parse files from agent response.")
		return fmt.Errorf("failed to parse agent response")
	}

	if err := os.MkdirAll(k8sOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for path, content := range files {
		fullPath := filepath.Join(k8sOutputDir, path)
		if _, err := os.Stat(fullPath); err == nil && !k8sForce {
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Skipping %s (exists). Use --force to overwrite.\n", fullPath)
			continue
		}

		content = strings.TrimSpace(content)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fullPath, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Created %s\n", fullPath)
	}

	return nil
}
