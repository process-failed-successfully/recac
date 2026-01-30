package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Policy struct {
	Name string `yaml:"name"`
}

type PolicyList struct {
	Policies []Policy `yaml:"policies"`
}

type Violation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Policy  string `json:"policy"`
	Message string `json:"message"`
}

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage and enforce codebase policies using AI",
	Long:  `Define semantic policies (e.g. "No hardcoded passwords") and enforce them using the AI agent.`,
}

var policyAddCmd = &cobra.Command{
	Use:   "add [policy description]",
	Short: "Add a new policy",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		policyName := args[0]
		// Join remaining args if any
		if len(args) > 1 {
			for _, arg := range args[1:] {
				policyName += " " + arg
			}
		}

		if err := addPolicy(policyName); err != nil {
			return err
		}
		fmt.Printf("✅ Policy added: %s\n", policyName)
		return nil
	},
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List defined policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		policies, err := loadPolicies()
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No policies defined.")
				return nil
			}
			return err
		}

		if len(policies) == 0 {
			fmt.Println("No policies defined.")
			return nil
		}

		fmt.Println("Current Policies:")
		for i, p := range policies {
			fmt.Printf("%d. %s\n", i+1, p.Name)
		}
		return nil
	},
}

var policyCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check codebase for policy violations",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Load Policies
		policies, err := loadPolicies()
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no policies defined. Use 'recac policy add' first")
			}
			return err
		}
		if len(policies) == 0 {
			return fmt.Errorf("no policies defined. Use 'recac policy add' first")
		}

		// 2. Generate Context
		fmt.Println("🔍 Analyzing codebase...")
		ctxOpts := ContextOptions{
			Roots:   []string{"."},
			MaxSize: 100 * 1024, // 100KB per file
			// Default ignores are handled inside GenerateCodebaseContext
		}
		codebase, err := GenerateCodebaseContext(ctxOpts)
		if err != nil {
			return fmt.Errorf("failed to generate codebase context: %w", err)
		}

		// 3. Prepare Prompt
		var policyListBuilder strings.Builder
		for _, p := range policies {
			policyListBuilder.WriteString(fmt.Sprintf("- %s\n", p.Name))
		}

		prompt := fmt.Sprintf(`You are a strict Code Policy Enforcer.
Analyze the provided Codebase against the defined Policies.

<policies>
%s
</policies>

<codebase>
%s
</codebase>

Return a JSON array of violations found.
Format: [{"file": "path/to/file", "line": 123, "policy": "Policy Name", "message": "Description of violation"}]
If no violations are found, return exactly: []
Do not include any explanation outside the JSON.
`, policyListBuilder.String(), codebase)

		// 4. Call Agent
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		cwd, _ := os.Getwd()
		provider := viper.GetString("provider")
		model := viper.GetString("model")

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-policy")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		fmt.Println("🤖 Consulting AI agent...")
		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		// 5. Parse Response
		jsonStr := utils.CleanCodeBlock(resp)
		var violations []Violation
		if err := json.Unmarshal([]byte(jsonStr), &violations); err != nil {
			// Try to handle potential wrapping or text
			return fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, resp)
		}

		// 6. Report
		if len(violations) == 0 {
			fmt.Println("✅ All policies passed!")
			return nil
		}

		fmt.Printf("\n❌ Found %d violations:\n\n", len(violations))
		for _, v := range violations {
			fmt.Printf("• %s\n", v.Policy)
			fmt.Printf("  File: %s:%d\n", v.File, v.Line)
			fmt.Printf("  Message: %s\n\n", v.Message)
		}

		// Exit with error code
		return fmt.Errorf("policy check failed")
	},
}

func init() {
	rootCmd.AddCommand(policyCmd)
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyCheckCmd)
}

func getPolicyFile() (string, error) {
	// Use .recac/policies.yaml in current dir
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	recacDir := filepath.Join(cwd, ".recac")
	if _, err := os.Stat(recacDir); os.IsNotExist(err) {
		if err := os.Mkdir(recacDir, 0755); err != nil {
			return "", err
		}
	}
	return filepath.Join(recacDir, "policies.yaml"), nil
}

func loadPolicies() ([]Policy, error) {
	file, err := getPolicyFile()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var list PolicyList
	if err := yaml.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list.Policies, nil
}

func addPolicy(name string) error {
	policies, err := loadPolicies()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check duplicates
	for _, p := range policies {
		if p.Name == name {
			return fmt.Errorf("policy already exists")
		}
	}

	policies = append(policies, Policy{Name: name})
	list := PolicyList{Policies: policies}

	data, err := yaml.Marshal(&list)
	if err != nil {
		return err
	}

	file, err := getPolicyFile()
	if err != nil {
		return err
	}

	return os.WriteFile(file, data, 0644)
}
