package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Interactive dependency upgrade assistant",
	Long: `Detects outdated dependencies (Go modules or NPM packages),
allows you to select which ones to upgrade, and then attempts to fix any resulting build or test failures using AI.`,
	RunE: runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

type UpgradeCandidate struct {
	Name    string
	Current string
	Latest  string
	Type    string // "go" or "npm"
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	candidates, err := detectUpdates()
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "All dependencies are up to date! 🎉")
		return nil
	}

	selected, err := selectUpdates(candidates)
	if err != nil {
		return err // User cancelled or error
	}

	if len(selected) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No updates selected.")
		return nil
	}

	// Group by type to batch updates if possible (though for safety, one by one is better for AI fixing)
	// But usually we want to update all selected, run tests once, if fail, try to fix.
	// If fix fails, maybe rollback? Rollback is hard without git.
	// Let's check for clean git state first?
	// For now, let's just proceed.

	fmt.Fprintln(cmd.OutOrStdout(), "\nApplying updates...")
	for _, c := range selected {
		if err := applyUpdate(c); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to update %s: %v\n", c.Name, err)
			// Continue with others? or stop? Stop is safer.
			return fmt.Errorf("update failed for %s", c.Name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Updated %s to %s\n", c.Name, c.Latest)
	}

	// Run Verification
	fmt.Fprintln(cmd.OutOrStdout(), "\nVerifying changes (running tests)...")
	if err := verifyAndFix(cmd); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n✨ Upgrade complete and verified!")
	return nil
}

func detectUpdates() ([]UpgradeCandidate, error) {
	var candidates []UpgradeCandidate

	// Check Go
	if _, err := os.Stat("go.mod"); err == nil {
		fmt.Println("Checking Go modules...")
		goUpdates, err := checkUpdatesGo()
		if err != nil {
			fmt.Printf("Warning: failed to check go modules: %v\n", err)
		} else {
			candidates = append(candidates, goUpdates...)
		}
	}

	// Check NPM
	if _, err := os.Stat("package.json"); err == nil {
		fmt.Println("Checking NPM packages...")
		npmUpdates, err := checkUpdatesNpm()
		if err != nil {
			fmt.Printf("Warning: failed to check npm packages: %v\n", err)
		} else {
			candidates = append(candidates, npmUpdates...)
		}
	}

	return candidates, nil
}

type GoListOutput struct {
	Path    string
	Version string
	Update  *struct {
		Version string
	}
}

func checkUpdatesGo() ([]UpgradeCandidate, error) {
	// go list -u -m -json all
	cmd := execCommand("go", "list", "-u", "-m", "-json", "all")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var candidates []UpgradeCandidate
	decoder := json.NewDecoder(strings.NewReader(string(out)))
	for decoder.More() {
		var mod GoListOutput
		if err := decoder.Decode(&mod); err != nil {
			// Skip malformed entries
			continue
		}
		if mod.Update != nil && mod.Path != "" {
			candidates = append(candidates, UpgradeCandidate{
				Name:    mod.Path,
				Current: mod.Version,
				Latest:  mod.Update.Version,
				Type:    "go",
			})
		}
	}
	return candidates, nil
}

type NpmOutdatedOutput map[string]struct {
	Current string
	Latest  string
}

func checkUpdatesNpm() ([]UpgradeCandidate, error) {
	// npm outdated --json
	cmd := execCommand("npm", "outdated", "--json")
	out, err := cmd.Output()
	if err != nil {
		// npm outdated returns exit code 1 if there are outdated packages, which is annoying.
		// We need to check if output is valid JSON.
		// If it's exit code 1 but we have output, it's fine.
		if len(out) == 0 {
			return nil, err
		}
	}

	if len(out) == 0 {
		return nil, nil
	}

	var data NpmOutdatedOutput
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("failed to parse npm output: %w", err)
	}

	var candidates []UpgradeCandidate
	for pkg, info := range data {
		candidates = append(candidates, UpgradeCandidate{
			Name:    pkg,
			Current: info.Current,
			Latest:  info.Latest,
			Type:    "npm",
		})
	}
	return candidates, nil
}

func selectUpdates(candidates []UpgradeCandidate) ([]UpgradeCandidate, error) {
	var options []string
	candidateMap := make(map[string]UpgradeCandidate)

	for _, c := range candidates {
		label := fmt.Sprintf("[%s] %s (%s -> %s)", strings.ToUpper(c.Type), c.Name, c.Current, c.Latest)
		options = append(options, label)
		candidateMap[label] = c
	}

	var selectedLabels []string
	prompt := &survey.MultiSelect{
		Message: "Select dependencies to upgrade:",
		Options: options,
	}

	if err := askOneFunc(prompt, &selectedLabels); err != nil {
		return nil, err
	}

	var selected []UpgradeCandidate
	for _, label := range selectedLabels {
		selected = append(selected, candidateMap[label])
	}
	return selected, nil
}

func applyUpdate(c UpgradeCandidate) error {
	switch c.Type {
	case "go":
		// go get -u [package]
		// Actually, to update to specific latest version found:
		// go get [package]@[version]
		target := fmt.Sprintf("%s@%s", c.Name, c.Latest)
		cmd := execCommand("go", "get", target)
		return cmd.Run()
	case "npm":
		// npm install [package]@[version]
		target := fmt.Sprintf("%s@%s", c.Name, c.Latest)
		cmd := execCommand("npm", "install", target)
		return cmd.Run()
	}
	return fmt.Errorf("unknown type: %s", c.Type)
}

func verifyAndFix(cmd *cobra.Command) error {
	// 1. Determine test command
	testCmd := ""
	if _, err := os.Stat("go.mod"); err == nil {
		testCmd = "go test ./..."
	} else if _, err := os.Stat("package.json"); err == nil {
		testCmd = "npm test"
	} else {
		return fmt.Errorf("could not determine test command")
	}

	maxRetries := 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		fmt.Fprintf(cmd.OutOrStdout(), "Running tests (Attempt %d/%d)...\n", attempt+1, maxRetries+1)
		output, err := executeShellCommand(testCmd)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Tests passed!")
			return nil
		}

		if attempt == maxRetries {
			return fmt.Errorf("tests failed after %d attempts. Manual intervention required.\nOutput:\n%s", maxRetries, output)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "❌ Tests failed. Asking AI to fix...\n")

		// Fix with AI
		if err := runAgentFix(cmd, output); err != nil {
			return fmt.Errorf("agent failed to fix code: %w", err)
		}
	}
	return nil
}

func runAgentFix(cmd *cobra.Command, errorOutput string) error {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-upgrade-fix")
	if err != nil {
		return err
	}

	// We need context. Maybe git diff?
	// Get changed files since last commit? Or just unstaged changes?
	// The update modified go.mod/sum. The fix might need to modify code.
	// We assume the update is staged or unstaged.
	// Let's get "git diff" output to see what changed (deps) and if we already modified code.

	diffOutput, _ := executeShellCommand("git diff")

	// Also maybe we need to find which files are causing errors.
	// Similar to 'debug', extract files.
	fileContext, _ := extractFileContexts(errorOutput)

	prompt := fmt.Sprintf(`I updated dependencies and tests are failing.
Please fix the code to be compatible with the new dependencies.

Error Output:
'''
%s
'''

Git Diff (Recent Changes):
'''
%s
'''

Relevant Files:
%s

Return the fixed code for the relevant files.
Wrap each file in XML tags like:
<file path="path/to/file.go">
... code ...
</file>
`, errorOutput, diffOutput, fileContext)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	modifiedFiles := utils.ParseFileBlocks(resp)
	if len(modifiedFiles) == 0 {
		return fmt.Errorf("agent suggested no changes")
	}

	for path, content := range modifiedFiles {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Fixed %s\n", path)
	}

	return nil
}
