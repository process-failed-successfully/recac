package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"recac/internal/architecture"
	"recac/internal/cmdutils"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub integration commands",
	Long:  "Commands for interacting with GitHub API",
}

var githubGenerateFromSpecCmd = &cobra.Command{
	Use:   "generate-from-spec",
	Short: "Generate GitHub issues from app_spec.txt",
	Long:  "Reads app_spec.txt, uses an LLM to decompose it into issues, and creates them in GitHub.",
	Run:   runGithubGenerateFromSpec,
}

func runGithubGenerateFromSpec(cmd *cobra.Command, args []string) {
	specPath, _ := cmd.Flags().GetString("spec")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read spec file %s: %v\n", specPath, err)
		exit(1)
	}

	ctx := context.Background()
	ghClient, err := cmdutils.GetGitHubClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, ".", "recac-github-gen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize agent: %v\n", err)
		exit(1)
	}

	repoURL, _ := cmd.Flags().GetString("repo-url")
	if repoURL == "" {
		// Try to construct from client config
		repoURL = fmt.Sprintf("https://github.com/%s/%s", ghClient.Owner, ghClient.Repo)
	}

	userLabels, _ := cmd.Flags().GetStringSlice("label")
	runLabel := fmt.Sprintf("recac-gen-%s", time.Now().Format("20060102-150405"))
	allLabels := append([]string{runLabel}, userLabels...)

	createdTickets, err := generateTickets(ctx, string(specContent), ghClient.Repo, repoURL, allLabels, ghClient, ag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	outputPath, _ := cmd.Flags().GetString("output-json")
	if outputPath != "" {
		data, _ := json.MarshalIndent(createdTickets, "", "  ")
		os.WriteFile(outputPath, data, 0644)
	}
}

var githubGenerateFromArchCmd = &cobra.Command{
	Use:   "generate-from-arch",
	Short: "Generate GitHub issues from architecture.yaml",
	Long:  "Reads architecture.yaml, and deterministically creates issues for components.",
	Run:   runGithubGenerateFromArch,
}

func runGithubGenerateFromArch(cmd *cobra.Command, args []string) {
	archPath, _ := cmd.Flags().GetString("arch")
	ctx := context.Background()

	archData, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read arch file %s: %v\n", archPath, err)
		exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(archData, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to parse architecture: %v\n", err)
		exit(1)
	}

	ghClient, err := cmdutils.GetGitHubClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	repoURL, _ := cmd.Flags().GetString("repo-url")
	if repoURL == "" {
		repoURL = fmt.Sprintf("https://github.com/%s/%s", ghClient.Owner, ghClient.Repo)
	}

	specPath, _ := cmd.Flags().GetString("spec")
	var specContent string
	if specPath != "" {
		content, err := os.ReadFile(specPath)
		if err == nil {
			specContent = string(content)
		}
	}

	rootDesc := fmt.Sprintf("Implementation of %s system.\nRepo: %s", arch.SystemName, repoURL)
	if specContent != "" {
		rootDesc += "\n\n# Application Specification\n\n" + specContent
	}

	rootEpic := ticketNode{
		Title:       fmt.Sprintf("ID:[SYSTEM] %s Architecture", arch.SystemName),
		Description: rootDesc,
		Type:        "Epic",
		Children:    []ticketNode{},
	}

	for _, comp := range arch.Components {
		compStory := ticketNode{
			Title:       fmt.Sprintf("ID:[%s] [Service] %s", comp.ID, comp.ID),
			Description: fmt.Sprintf("%s\n\nType: %s\nRepo: %s", comp.Description, comp.Type, repoURL),
			Type:        "Story",
			Children:    []ticketNode{},
		}

		for i, step := range comp.ImplementationSteps {
			compStory.Children = append(compStory.Children, ticketNode{
				Title:       fmt.Sprintf("ID:[%s-STEP-%d] %s", comp.ID, i+1, truncateString(step, 50)),
				Description: fmt.Sprintf("Task: %s\nRepo: %s", step, repoURL),
				Type:        "Subtask",
			})
		}

		for _, fn := range comp.Functions {
			desc := fmt.Sprintf("Implement Function: %s\n", fn.Name)
			desc += fmt.Sprintf("Signature: (%s) -> (%s)\n", fn.Args, fn.Return)
			desc += fmt.Sprintf("Description: %s\n", fn.Description)
			desc += fmt.Sprintf("Repo: %s\n", repoURL)

			criteria := []string{
				fmt.Sprintf("Function %s matches signature (%s) -> (%s)", fn.Name, fn.Args, fn.Return),
			}
			criteria = append(criteria, fn.Requirements...)

			compStory.Children = append(compStory.Children, ticketNode{
				Title:              fmt.Sprintf("ID:[%s-FUNC-%s] Func %s", comp.ID, fn.Name, fn.Name),
				Description:        desc,
				Type:               "Subtask",
				AcceptanceCriteria: criteria,
			})
		}

		for _, in := range comp.Consumes {
			compStory.Children = append(compStory.Children, ticketNode{
				Title:       fmt.Sprintf("ID:[%s-IN-%s] Implement Input %s", comp.ID, in.Type, in.Type),
				Description: fmt.Sprintf("Implement consumption of %s from %s.\nSchema: %s\nRepo: %s", in.Type, in.Source, in.Schema, repoURL),
				Type:        "Subtask",
				AcceptanceCriteria: []string{
					fmt.Sprintf("Component %s successfully parses %s", comp.ID, in.Type),
				},
			})
		}

		for _, out := range comp.Produces {
			typeName := out.Type
			if typeName == "" {
				typeName = out.Event
			}
			compStory.Children = append(compStory.Children, ticketNode{
				Title:       fmt.Sprintf("ID:[%s-OUT-%s] Implement Output %s", comp.ID, typeName, typeName),
				Description: fmt.Sprintf("Implement production of %s.\nSchema: %s\nRepo: %s", typeName, out.Schema, repoURL),
				Type:        "Subtask",
				AcceptanceCriteria: []string{
					fmt.Sprintf("Component %s successfully emits valid %s", comp.ID, typeName),
				},
			})
		}
		rootEpic.Children = append(rootEpic.Children, compStory)
	}

	tickets := []ticketNode{rootEpic}

	userLabels, _ := cmd.Flags().GetStringSlice("label")
	runLabel := fmt.Sprintf("recac-gen-%s", time.Now().Format("20060102-150405"))
	allLabels := append([]string{runLabel}, userLabels...)

	createdTickets, err := createTicketsFromNodes(ctx, tickets, ghClient.Repo, repoURL, allLabels, ghClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating tickets: %v\n", err)
		exit(1)
	}

	outputPath, _ := cmd.Flags().GetString("output-json")
	if outputPath != "" {
		data, _ := json.MarshalIndent(createdTickets, "", "  ")
		os.WriteFile(outputPath, data, 0644)
	}
}

func init() {
	rootCmd.AddCommand(githubCmd)

	githubGenerateFromSpecCmd.Flags().String("spec", "app_spec.txt", "Path to application specification file")
	githubGenerateFromSpecCmd.Flags().StringSliceP("label", "l", []string{}, "Custom labels")
	githubGenerateFromSpecCmd.Flags().String("output-json", "", "Path to write output JSON")
	githubGenerateFromSpecCmd.Flags().String("repo-url", "", "Repository URL")
	githubCmd.AddCommand(githubGenerateFromSpecCmd)

	githubGenerateFromArchCmd.Flags().String("arch", ".recac/architecture/architecture.yaml", "Path to architecture.yaml")
	githubGenerateFromArchCmd.Flags().String("spec", "", "Path to original app_spec.txt")
	githubGenerateFromArchCmd.Flags().String("repo-url", "", "Repository URL")
	githubGenerateFromArchCmd.Flags().StringSliceP("label", "l", []string{}, "Labels")
	githubGenerateFromArchCmd.Flags().String("output-json", "", "Output JSON path")
	githubCmd.AddCommand(githubGenerateFromArchCmd)
}
