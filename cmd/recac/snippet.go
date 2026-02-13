package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/cmdutils"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var snippetCmd = &cobra.Command{
	Use:   "snippet",
	Short: "Manage and apply code snippets",
	Long:  `Store, list, and apply code snippets with optional AI-powered placeholder filling.`,
}

var snippetAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new snippet",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnippetAdd,
}

var snippetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available snippets",
	RunE:  runSnippetList,
}

var snippetApplyCmd = &cobra.Command{
	Use:   "apply <name>",
	Short: "Apply a snippet",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnippetApply,
}

var snippetDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a snippet",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnippetDelete,
}

var snippetContent string
var snippetUseAI bool

func initSnippetCmd(rootCmd *cobra.Command) {
	snippetAddCmd.Flags().StringVarP(&snippetContent, "content", "c", "", "Content of the snippet (required)")
	snippetAddCmd.MarkFlagRequired("content")

	snippetApplyCmd.Flags().BoolVar(&snippetUseAI, "ai", false, "Use AI to fill placeholders")

	snippetCmd.AddCommand(snippetAddCmd)
	snippetCmd.AddCommand(snippetListCmd)
	snippetCmd.AddCommand(snippetApplyCmd)
	snippetCmd.AddCommand(snippetDeleteCmd)

	rootCmd.AddCommand(snippetCmd)
}

var getSnippetDirs = func() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(".recac", "snippets"),
		filepath.Join(home, ".recac", "snippets"),
	}
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func runSnippetAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	// Validate name
	if strings.Contains(name, "..") || strings.Contains(name, string(os.PathSeparator)) {
		return fmt.Errorf("invalid snippet name: %s", name)
	}

	dirs := getSnippetDirs()
	localDir := dirs[0] // Prefer local .recac

	if err := ensureDir(localDir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", localDir, err)
	}

	path := filepath.Join(localDir, name)
	if err := os.WriteFile(path, []byte(snippetContent), 0644); err != nil {
		return fmt.Errorf("failed to write snippet: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Snippet '%s' added to %s\n", name, localDir)
	return nil
}

func runSnippetList(cmd *cobra.Command, args []string) error {
	dirs := getSnippetDirs()
	snippets := make(map[string]string) // name -> location

	// Iterate in reverse order (global then local) so local overwrites global
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		files, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				// Just log warning if directory cannot be read, but continue
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read %s: %v\n", dir, err)
			}
			continue
		}
		for _, f := range files {
			if !f.IsDir() {
				snippets[f.Name()] = dir
			}
		}
	}

	if len(snippets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No snippets found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLOCATION")
	for name, loc := range snippets {
		fmt.Fprintf(w, "%s\t%s\n", name, loc)
	}
	w.Flush()
	return nil
}

func findSnippet(name string) (string, error) {
	if strings.Contains(name, "..") || strings.Contains(name, string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid snippet name")
	}

	dirs := getSnippetDirs()
	for _, dir := range dirs {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("snippet '%s' not found", name)
}

func runSnippetApply(cmd *cobra.Command, args []string) error {
	name := args[0]
	path, err := findSnippet(name)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read snippet: %w", err)
	}
	text := string(content)

	if snippetUseAI {
		return applyAI(cmd, text)
	}

	fmt.Fprint(cmd.OutOrStdout(), text)
	return nil
}

func applyAI(cmd *cobra.Command, text string) error {
	ctx := context.Background()
	// Pass empty project path/name as snippet application is context-free
	client, err := cmdutils.GetAgentClient(ctx, "", "", "", "")
	if err != nil {
		return fmt.Errorf("failed to initialize AI agent: %w", err)
	}

	prompt := fmt.Sprintf("Please fill in the placeholders ({{...}}) in the following code snippet and return only the completed code:\n\n```\n%s\n```\nDo not include any explanation, just the code.", text)

	resp, err := client.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("AI agent failed: %w", err)
	}

	// Extract code block
	code := extractSnippetCode(resp)
	fmt.Fprint(cmd.OutOrStdout(), code)
	return nil
}

func extractSnippetCode(text string) string {
	// regex to find content between ``` and ```
	// Handle optional language identifier
	re := regexp.MustCompile("(?s)```(?:\\w+\\n)?(.*?)```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	// If no code block found, return original text but trim whitespace
	return strings.TrimSpace(text)
}

func runSnippetDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	path, err := findSnippet(name)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete snippet: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Snippet '%s' deleted from %s\n", name, filepath.Dir(path))
	return nil
}
