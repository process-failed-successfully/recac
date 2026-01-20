package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/architecture"
	"recac/internal/cmdutils"
	"recac/internal/jira"

	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// jiraCmd represents the jira command
var jiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Jira integration commands",
	Long:  "Commands for interacting with Jira API",
}

// jiraTestAuthCmd represents the jira test-auth command
var jiraTestAuthCmd = &cobra.Command{
	Use:   "test-auth",
	Short: "Test Jira authentication",
	Long: `Test Jira authentication using credentials from environment variables or config.
	
Environment variables:
  JIRA_URL       - Jira instance URL (e.g., https://yourcompany.atlassian.net)
  JIRA_USERNAME  - Jira username or email
  JIRA_API_TOKEN - Jira API token
  
Or configure in config.yaml:
  jira:
    url: https://yourcompany.atlassian.net
    username: user@example.com
    api_token: your-api-token`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create Jira client using factory helper
		ctx := context.Background()
		client, err := cmdutils.GetJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		// Test authentication
		if err := client.Authenticate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Authentication failed: %v\n", err)
			exit(1)
		}

		fmt.Println("Success: Jira authentication successful!")
	},
}

// jiraGetCmd represents the jira get command
var jiraGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a Jira ticket by ID",
	Long: `Fetch and display a Jira ticket by its ID (e.g., PROJ-123).

Environment variables:
  JIRA_URL       - Jira instance URL (e.g., https://yourcompany.atlassian.net)
  JIRA_USERNAME  - Jira username or email
  JIRA_API_TOKEN - Jira API token
  
Or configure in config.yaml:
  jira:
    url: https://yourcompany.atlassian.net
    username: user@example.com
    api_token: your-api-token`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get ticket ID from flag
		ticketID, _ := cmd.Flags().GetString("id")
		if ticketID == "" {
			fmt.Fprintf(os.Stderr, "Error: --id flag is required\n")
			fmt.Fprintf(os.Stderr, "Usage: %s jira get --id PROJ-123\n", os.Args[0])
			exit(1)
		}

		// Create Jira client using factory helper
		ctx := context.Background()
		client, err := cmdutils.GetJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		// Fetch ticket
		ticket, err := client.GetTicket(ctx, ticketID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to fetch ticket %s: %v\n", ticketID, err)
			exit(1)
		}

		// Extract and display ticket details
		key, _ := ticket["key"].(string)
		fields, ok := ticket["fields"].(map[string]interface{})
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: Invalid ticket response format\n")
			exit(1)
		}

		summary, _ := fields["summary"].(string)
		description := client.ParseDescription(ticket)

		// Display ticket information
		fmt.Printf("Ticket: %s\n", key)
		fmt.Printf("Title: %s\n", summary)
		if description != "" {
			fmt.Printf("Description:\n%s\n", description)
		} else {
			fmt.Println("Description: (empty)")
		}
	},
}

// jiraTransitionCmd represents the jira transition command
var jiraTransitionCmd = &cobra.Command{
	Use:   "transition",
	Short: "Transition a Jira ticket to a new status",
	Long: `Transition a Jira ticket to a new status (e.g., "In Progress").

Environment variables:
  JIRA_URL       - Jira instance URL (e.g., https://yourcompany.atlassian.net)
  JIRA_USERNAME  - Jira username or email
  JIRA_API_TOKEN - Jira API token
  
Or configure in config.yaml:
  jira:
    url: https://yourcompany.atlassian.net
    username: user@example.com
    api_token: your-api-token`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get ticket ID from flag
		ticketID, _ := cmd.Flags().GetString("id")
		if ticketID == "" {
			fmt.Fprintf(os.Stderr, "Error: --id flag is required\n")
			fmt.Fprintf(os.Stderr, "Usage: %s jira transition --id PROJ-123 --transition-id 31\n", os.Args[0])
			exit(1)
		}

		// Get transition Name or ID from flag (defaults to "In Progress")
		transition, _ := cmd.Flags().GetString("transition")
		if transition == "" {
			transition = "In Progress"
		}

		// Create Jira client using factory helper
		ctx := context.Background()
		client, err := cmdutils.GetJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		// Transition ticket
		if err := client.SmartTransition(ctx, ticketID, transition); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to transition ticket %s: %v\n", ticketID, err)
			exit(1)
		}

		fmt.Printf("Success: Ticket %s transitioned to '%s' successfully\n", ticketID, transition)
	},
}

type ticketNode struct {
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	Type               string       `json:"type"`
	BlockedBy          []string     `json:"blocked_by"`
	AcceptanceCriteria []string     `json:"acceptance_criteria"`
	Children           []ticketNode `json:"children"`
}

// jiraGenerateFromSpecCmd represents the jira generate-from-spec command
var jiraGenerateFromSpecCmd = &cobra.Command{
	Use:   "generate-from-spec",
	Short: "Generate Jira tickets from app_spec.txt",
	Long:  "Reads app_spec.txt, uses an LLM to decompose it into Epics and Stories, and creates them in Jira.",
	Run:   runGenerateTicketsCmd,
}

func runGenerateTicketsCmd(cmd *cobra.Command, args []string) {
	// 1. Read app_spec.txt
	specPath, _ := cmd.Flags().GetString("spec")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read spec file %s: %v\n", specPath, err)
		exit(1)
	}

	// 2. Setup Jira Client
	ctx := context.Background()
	jiraClient, err := cmdutils.GetJiraClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	projectKey, _ := cmd.Flags().GetString("project")
	if projectKey == "" {
		projectKey = os.Getenv("JIRA_PROJECT_KEY")
	}
	if projectKey == "" {
		projectKey = viper.GetString("jira.project_key")
	}
	if projectKey == "" {
		fmt.Fprintf(os.Stderr, "Error: JIRA_PROJECT_KEY is required. Use --project flag, JIRA_PROJECT_KEY env var, or jira.project_key in config.\n")
		exit(1)
	}

	// 3. Setup Agent
	provider, _ := cmd.Flags().GetString("provider")
	if provider == "" {
		provider = viper.GetString("provider")
	}
	model, _ := cmd.Flags().GetString("model")
	if model == "" {
		model = viper.GetString("model")
	}

	ag, err := agentClientFactory(ctx, provider, model, ".", "recac-jira-gen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize agent: %v\n", err)
		exit(1)
	}

	// 4. Labels
	runLabel := fmt.Sprintf("recac-gen-%s", time.Now().Format("20060102-150405"))
	userLabels, _ := cmd.Flags().GetStringSlice("label")
	allLabels := append([]string{runLabel}, userLabels...)
	fmt.Printf("Using labels for all tickets: %v\n", allLabels)

	repoURL, _ := cmd.Flags().GetString("repo-url")

	createdTickets, err := generateTickets(ctx, string(specContent), projectKey, repoURL, allLabels, jiraClient, ag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	// 5. Output JSON if requested
	outputPath, _ := cmd.Flags().GetString("output-json")
	if outputPath != "" {
		data, err := json.MarshalIndent(createdTickets, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to marshal output JSON: %v\n", err)
			exit(1)
		}
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to write output JSON to %s: %v\n", outputPath, err)
			exit(1)
		}
		fmt.Printf("Created ticket mapping written to %s\n", outputPath)
	}
}

// generateTickets contains the core logic for ticket generation, decoupled from flags for testing.
func generateTickets(ctx context.Context, specContent, projectKey, repoURL string, allLabels []string, jiraClient jira.ClientInterface, ag agent.Agent) (map[string]string, error) {
	// 5. Generate Tickets JSON
	prompt, err := prompts.GetPrompt(prompts.TPMAgent, map[string]string{"spec": specContent})
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt: %w", err)
	}

	fmt.Println("Analyzing spec and generating ticket plan...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed to generate response: %w", err)
	}

	// Strip markdown code blocks if present
	jsonStr := resp
	if strings.Contains(jsonStr, "```json") {
		parts := strings.Split(jsonStr, "```json")
		if len(parts) > 1 {
			jsonStr = parts[1]
		}
		parts = strings.Split(jsonStr, "```")
		jsonStr = parts[0]
	} else if strings.Contains(jsonStr, "```") {
		// Generic code block
		parts := strings.Split(jsonStr, "```")
		if len(parts) > 1 {
			jsonStr = parts[1]
		}
		parts = strings.Split(jsonStr, "```")
		jsonStr = parts[0]
	}
	jsonStr = strings.TrimSpace(jsonStr)

	var tickets []ticketNode
	if err := json.Unmarshal([]byte(jsonStr), &tickets); err != nil {
		return nil, fmt.Errorf("failed to parse agent response as JSON: %w\nResponse was:\n%s", err, resp)
	}

	return createTicketsFromNodes(ctx, tickets, projectKey, repoURL, allLabels, jiraClient)
}

func createTicketsFromNodes(ctx context.Context, tickets []ticketNode, projectKey, repoURL string, allLabels []string, jiraClient jira.ClientInterface) (map[string]string, error) {
	fmt.Printf("Found %d top-level items. Creating tickets...\n", len(tickets))

	// Validate repository in descriptions
	repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
	// Helper for recursive validation
	var validate func([]ticketNode) error
	validate = func(nodes []ticketNode) error {
		for _, node := range nodes {
			// If repoURL is provided via flag, we don't strictly enforce it in description during validation
			// because we will inject it. But if NOT provided via flag, we enforce it.
			if repoURL == "" && !repoRegex.MatchString(node.Description) {
				return fmt.Errorf("Item '%s' description missing repository URL (Repo: https://...)", node.Title)
			}
			if err := validate(node.Children); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(tickets); err != nil {
		return nil, err
	}

	// Keep track of titles to keys for linking
	titleToKey := make(map[string]string)

	for _, node := range tickets {
		if err := createTicketRecursively(ctx, node, "", projectKey, repoURL, allLabels, jiraClient, titleToKey); err != nil {
			return nil, err
		}
	}

	// Create Links for Blockers
	fmt.Println("Creating issue links for blockers...")
	// Flatten all nodes to process links easily? Or just recurse again.
	// Let's recurse.
	var linkBlockers func([]ticketNode)
	linkBlockers = func(nodes []ticketNode) {
		for _, node := range nodes {
			nodeKey := titleToKey[node.Title]
			if nodeKey != "" {
				for _, blockerTitle := range node.BlockedBy {
					if blockerKey, ok := titleToKey[blockerTitle]; ok {
						fmt.Printf("Linking %s as blocked by %s\n", nodeKey, blockerKey)
						if err := jiraClient.AddIssueLink(ctx, blockerKey, nodeKey, "Blocks"); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: Failed to link %s as blocked by %s: %v\n", nodeKey, blockerKey, err)
						}
					}
				}
			}
			linkBlockers(node.Children)
		}
	}
	linkBlockers(tickets)

	// 4. Map logical IDs back from titles
	idToKey := make(map[string]string)
	idRegex := regexp.MustCompile(`(?i)ID:\[?([\w-]+)\]?`) // Match ID:[SQL] or ID:SQL-1

	for title, key := range titleToKey {
		matches := idRegex.FindStringSubmatch(title)
		if len(matches) > 1 {
			idToKey[matches[1]] = key
			fmt.Printf("Mapped ID %s -> %s\n", matches[1], key)
		}
	}

	fmt.Println("Done.")
	return idToKey, nil
}

func createTicketRecursively(ctx context.Context, node ticketNode, parentKey, projectKey, repoURL string, allLabels []string, jiraClient jira.ClientInterface, titleToKey map[string]string) error {
	issueType := node.Type
	if issueType == "" {
		// Inference fallback
		if parentKey == "" {
			issueType = "Epic"
		} else {
			issueType = "Story" // Default child
		}
	}

	indent := ""
	if parentKey != "" {
		indent = "  "
	}

	fmt.Printf("%sCreating %s: %s\n", indent, issueType, node.Title)

	// Combine Description and Acceptance Criteria
	fullDescription := node.Description
	if len(node.AcceptanceCriteria) > 0 {
		fullDescription += "\n\nAcceptance Criteria:\n"
		for _, ac := range node.AcceptanceCriteria {
			fullDescription += fmt.Sprintf("- %s\n", ac)
		}
	}

	// Inject Repo URL if provided and missing
	if repoURL != "" && !strings.Contains(strings.ToLower(fullDescription), "repo: http") {
		fullDescription += fmt.Sprintf("\n\nRepo: %s", repoURL)
	}

	var key string
	var err error

	if parentKey == "" {
		// Top level
		key, err = jiraClient.CreateTicket(ctx, projectKey, node.Title, fullDescription, issueType, allLabels)
	} else {
		// Child
		key, err = jiraClient.CreateChildTicket(ctx, projectKey, node.Title, fullDescription, issueType, parentKey, allLabels)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create %s '%s': %v. Trying 'Task'...\n", issueType, node.Title, err)
		fallbackType := "Task"
		// If it's a subtask level, maybe we need "Subtask"? But CreateChildTicket might handle that mapping if the provider needs it.
		// For now assuming "Task" is a safe fallback for Stories, but Sub-tasks are special in Jira.
		// If explicit "Subtask" failed, falling back to "Task" might fail if parent is an issue that can't have "Task" as subtask?
		// Actually typical Jira hierarchy: Epic -> Story/Task/Bug -> Subtask.
		// If we are at level 3 (Subtask), "Task" might not be valid sub-issue type.
		// But let's assume the user config/Jira setup handles standard types.

		if parentKey == "" {
			key, err = jiraClient.CreateTicket(ctx, projectKey, node.Title, fullDescription, fallbackType, allLabels)
		} else {
			key, err = jiraClient.CreateChildTicket(ctx, projectKey, node.Title, fullDescription, fallbackType, parentKey, allLabels)
		}

		if err != nil {
			return fmt.Errorf("failed to create ticket '%s': %w", node.Title, err)
		}
		issueType = fallbackType // update for log
	}

	fmt.Printf("%s-> Created %s %s\n", indent, issueType, key)
	titleToKey[node.Title] = key

	for _, child := range node.Children {
		if err := createTicketRecursively(ctx, child, key, projectKey, repoURL, allLabels, jiraClient, titleToKey); err != nil {
			return err
		}
	}
	return nil
}

// jiraGenerateFromArchCmd represents the jira generate-from-arch command
var jiraGenerateFromArchCmd = &cobra.Command{
	Use:   "generate-from-arch",
	Short: "Generate Jira tickets from architecture.yaml",
	Long:  "Reads architecture.yaml, and deterministically creates Epics for components and Stories for their inputs/outputs.",
	Run:   runGenerateFromArchCmd,
}

func runGenerateFromArchCmd(cmd *cobra.Command, args []string) {
	archPath, _ := cmd.Flags().GetString("arch")
	repoUrl, _ := cmd.Flags().GetString("repo-url")
	ctx := context.Background()

	// 1. Read Arch
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

	// 1b. Read Spec (Optional but recommended)
	specPath, _ := cmd.Flags().GetString("spec")
	var specContent string
	if specPath != "" {
		content, err := os.ReadFile(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to read spec file %s: %v\n", specPath, err)
		} else {
			specContent = string(content)
		}
	}

	// 2. Build Ticket Tree
	// Level 1: System Epic
	rootDesc := fmt.Sprintf("Implementation of %s system.\nRepo: %s", arch.SystemName, repoUrl)
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
		// Level 2: Component Story
		compStory := ticketNode{
			Title:       fmt.Sprintf("ID:[%s] [Service] %s", comp.ID, comp.ID),
			Description: fmt.Sprintf("%s\n\nType: %s\nRepo: %s", comp.Description, comp.Type, repoUrl),
			Type:        "Story", // Was Epic, now Story
			Children:    []ticketNode{},
		}

		// Level 3: Implementation Steps (Subtasks)
		for i, step := range comp.ImplementationSteps {
			compStory.Children = append(compStory.Children, ticketNode{
				Title:       fmt.Sprintf("ID:[%s-STEP-%d] %s", comp.ID, i+1, truncate(step, 50)),
				Description: fmt.Sprintf("Task: %s\nRepo: %s", step, repoUrl),
				Type:        "Subtask",
			})
		}

		// Level 3: Functions (Subtasks)
		for _, fn := range comp.Functions {
			desc := fmt.Sprintf("Implement Function: %s\n", fn.Name)
			desc += fmt.Sprintf("Signature: (%s) -> (%s)\n", fn.Args, fn.Return)
			desc += fmt.Sprintf("Description: %s\n", fn.Description)
			desc += fmt.Sprintf("Repo: %s\n", repoUrl)

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

		// Level 3: Inputs (Subtasks)
		for _, in := range comp.Consumes {
			compStory.Children = append(compStory.Children, ticketNode{
				Title:       fmt.Sprintf("ID:[%s-IN-%s] Implement Input %s", comp.ID, in.Type, in.Type),
				Description: fmt.Sprintf("Implement consumption of %s from %s.\nSchema: %s\nRepo: %s", in.Type, in.Source, in.Schema, repoUrl),
				Type:        "Subtask", // Was Story, now Subtask
				AcceptanceCriteria: []string{
					fmt.Sprintf("Component %s successfully parses %s", comp.ID, in.Type),
				},
			})
		}

		// Level 3: Outputs (Subtasks)
		for _, out := range comp.Produces {
			typeName := out.Type
			if typeName == "" {
				typeName = out.Event
			}
			compStory.Children = append(compStory.Children, ticketNode{
				Title:       fmt.Sprintf("ID:[%s-OUT-%s] Implement Output %s", comp.ID, typeName, typeName),
				Description: fmt.Sprintf("Implement production of %s.\nSchema: %s\nRepo: %s", typeName, out.Schema, repoUrl),
				Type:        "Subtask", // Was Story, now Subtask
				AcceptanceCriteria: []string{
					fmt.Sprintf("Component %s successfully emits valid %s", comp.ID, typeName),
				},
			})
		}
		rootEpic.Children = append(rootEpic.Children, compStory)
	}

	tickets := []ticketNode{rootEpic}

	// 3. Setup Jira Client
	jiraClient, err := getJiraClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	projectKey, _ := cmd.Flags().GetString("project")
	if projectKey == "" {
		projectKey = os.Getenv("JIRA_PROJECT_KEY")
	}
	if projectKey == "" {
		projectKey = viper.GetString("jira.project_key")
	}

	// 4. Labels
	runLabel := fmt.Sprintf("recac-gen-%s", time.Now().Format("20060102-150405"))
	userLabels, _ := cmd.Flags().GetStringSlice("label")
	allLabels := append([]string{runLabel}, userLabels...)

	// 5. Create tickets using existing helper
	// We need to slightly refactor createTickets to be reusable or just call it.
	// For now, I'll copy-paste the creation loop from generateTickets or move it to a helper.

	// Actually, let's just use the logic from generateTickets but pass the pre-built nodes.
	createdTickets, err := createTicketsFromNodes(ctx, tickets, projectKey, repoUrl, allLabels, jiraClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating tickets: %v\n", err)
		exit(1)
	}

	// 6. Output JSON
	outputPath, _ := cmd.Flags().GetString("output-json")
	if outputPath != "" {
		data, _ := json.MarshalIndent(createdTickets, "", "  ")
		os.WriteFile(outputPath, data, 0644)
	}
}

func init() {
	rootCmd.AddCommand(jiraCmd)
	jiraCmd.AddCommand(jiraTestAuthCmd)
	jiraGetCmd.Flags().String("id", "", "Jira ticket ID (e.g., PROJ-123)")
	jiraGetCmd.MarkFlagRequired("id")
	jiraCmd.AddCommand(jiraGetCmd)
	jiraTransitionCmd.Flags().String("id", "", "Jira ticket ID (e.g., PROJ-123)")
	jiraTransitionCmd.Flags().String("transition", "", "Transition Name or ID (defaults to 'In Progress')")
	jiraTransitionCmd.MarkFlagRequired("id")
	jiraCmd.AddCommand(jiraTransitionCmd)

	jiraGenerateFromSpecCmd.Flags().String("spec", "app_spec.txt", "Path to application specification file")
	jiraGenerateFromSpecCmd.Flags().String("project", "", "Jira project key (overrides JIRA_PROJECT_KEY env var and config)")
	jiraGenerateFromSpecCmd.Flags().StringSliceP("label", "l", []string{}, "Custom labels to add to generated tickets")
	jiraGenerateFromSpecCmd.Flags().String("output-json", "", "Path to write the created ticket mapping (Title -> Key) in JSON format")
	jiraGenerateFromSpecCmd.Flags().String("repo-url", "", "Repository URL to include in ticket descriptions")
	jiraCmd.AddCommand(jiraGenerateFromSpecCmd)

	jiraGenerateFromArchCmd.Flags().String("arch", ".recac/architecture/architecture.yaml", "Path to architecture.yaml")
	jiraGenerateFromArchCmd.Flags().String("spec", "", "Path to original app_spec.txt to include in root ticket")
	jiraGenerateFromArchCmd.Flags().String("project", "", "Jira project key")
	jiraGenerateFromArchCmd.Flags().String("repo-url", "", "Repository URL to include in descriptions")
	jiraGenerateFromArchCmd.Flags().StringSliceP("label", "l", []string{}, "Labels")
	jiraGenerateFromArchCmd.Flags().String("output-json", "", "Output JSON path")
	viper.BindPFlag("repo_url", jiraGenerateFromArchCmd.Flags().Lookup("repo-url"))
	jiraCmd.AddCommand(jiraGenerateFromArchCmd)
}

// jiraCleanupCmd represents the jira cleanup command
var jiraCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Delete Jira tickets by label",
	Long:  "Deletes all Jira tickets that match the specified label. Use with caution.",
	Run: func(cmd *cobra.Command, args []string) {
		label, _ := cmd.Flags().GetString("label")
		if label == "" {
			fmt.Fprintf(os.Stderr, "Error: --label is required\n")
			exit(1)
		}

		ctx := context.Background()
		client, err := getJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		fmt.Printf("Searching for tickets with label '%s'...\n", label)
		issues, err := client.LoadLabelIssues(ctx, label)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to load issues: %v\n", err)
			exit(1)
		}

		if len(issues) == 0 {
			fmt.Println("No tickets found to delete.")
			return
		}

		fmt.Printf("Found %d tickets. Deleting...\n", len(issues))
		count := 0
		for _, issue := range issues {
			key, _ := issue["key"].(string)
			fmt.Printf("Deleting %s... ", key)
			if err := client.DeleteIssue(ctx, key); err != nil {
				fmt.Printf("Failed: %v\n", err)
			} else {
				fmt.Printf("Success\n")
				count++
			}
		}
		fmt.Printf("Done. Deleted %d tickets.\n", count)
	},
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
