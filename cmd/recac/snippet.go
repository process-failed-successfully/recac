package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/utils"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	snippetApplyAI bool
)

var snippetCmd = &cobra.Command{
	Use:   "snippet",
	Short: "Manage and apply reusable code snippets",
	Long: `Manage a library of code snippets.
Snippets are stored in ~/.recac/snippets/ (global) or .recac/snippets/ (local).
You can save, list, show, and apply snippets to your code.
The 'apply' command supports AI to intelligently fill in placeholders.`,
}

var snippetAddCmd = &cobra.Command{
	Use:   "add [name] [file]",
	Short: "Add a new snippet",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSnippetAdd,
}

var snippetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all snippets",
	RunE:  runSnippetList,
}

var snippetShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show snippet content",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnippetShow,
}

var snippetApplyCmd = &cobra.Command{
	Use:   "apply [name] [target-file]",
	Short: "Apply a snippet to a file or stdout",
	Long: `Applies the specified snippet.
If a target file is provided, the snippet is appended to it.
If no target file is provided, the snippet is printed to stdout.

Use --ai to let the agent fill in placeholders (e.g. {{variable}}) based on the target file's context.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSnippetApply,
}

var snippetRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a snippet",
	Args:    cobra.ExactArgs(1),
	RunE:    runSnippetRemove,
}

func init() {
	rootCmd.AddCommand(snippetCmd)
	snippetCmd.AddCommand(snippetAddCmd)
	snippetCmd.AddCommand(snippetListCmd)
	snippetCmd.AddCommand(snippetShowCmd)
	snippetCmd.AddCommand(snippetApplyCmd)
	snippetCmd.AddCommand(snippetRemoveCmd)

	snippetApplyCmd.Flags().BoolVar(&snippetApplyAI, "ai", false, "Use AI to fill placeholders")
}

// getSnippetDirs returns the list of snippet directories (local and global).
// Priority: Local > Global.
func getSnippetDirs() ([]string, error) {
	var dirs []string

	// Local
	cwd, err := os.Getwd()
	if err == nil {
		localDir := filepath.Join(cwd, ".recac", "snippets")
		if _, err := os.Stat(localDir); err == nil {
			dirs = append(dirs, localDir)
		}
	}

	// Global
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	globalDir := filepath.Join(home, ".recac", "snippets")
	// Ensure global exists
	if _, err := os.Stat(globalDir); os.IsNotExist(err) {
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			return nil, err
		}
	}
	dirs = append(dirs, globalDir)

	return dirs, nil
}

// resolveSnippet finds a snippet by name in the available directories.
// Returns the full path and the content.
func resolveSnippet(name string) (string, string, error) {
	if strings.Contains(name, "..") || strings.Contains(name, string(os.PathSeparator)) {
		return "", "", fmt.Errorf("invalid snippet name: %s", name)
	}

	dirs, err := getSnippetDirs()
	if err != nil {
		return "", "", err
	}

	for _, dir := range dirs {
		path := filepath.Join(dir, name)
		// Check for extension-less match first
		if content, err := os.ReadFile(path); err == nil {
			return path, string(content), nil
		}
		// Check with common extensions?
		// For simplicity, we assume the name matches the filename exactly for now.
		// Or we can list dir and find prefix match?
		// Let's iterate directory to find match ignoring extension
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.Name() == name || strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())) == name {
				fullPath := filepath.Join(dir, f.Name())
				content, err := os.ReadFile(fullPath)
				if err == nil {
					return fullPath, string(content), nil
				}
			}
		}
	}
	return "", "", fmt.Errorf("snippet '%s' not found", name)
}

func runSnippetAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	if strings.Contains(name, "..") || strings.Contains(name, string(os.PathSeparator)) {
		return fmt.Errorf("invalid snippet name: %s", name)
	}
	var content []byte
	var err error

	if len(args) > 1 {
		// Read from file
		content, err = os.ReadFile(args[1])
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
	} else {
		// Read from stdin
		// Use cmd.InOrStdin() to allow mocking in tests
		in := cmd.InOrStdin()

		// If it's a file (like os.Stdin), check if it's a TTY
		if f, ok := in.(*os.File); ok && f == os.Stdin {
			stat, _ := f.Stat()
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				return fmt.Errorf("please provide a file path or pipe content via stdin")
			}
		}

		content, err = io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	}

	// Determine where to save
	// Default to global unless .recac exists in current dir
	targetDir := ""
	cwd, _ := os.Getwd()
	localRecac := filepath.Join(cwd, ".recac")
	if _, err := os.Stat(localRecac); err == nil {
		targetDir = filepath.Join(localRecac, "snippets")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		targetDir = filepath.Join(home, ".recac", "snippets")
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// If input file had extension, maybe preserve it?
	// But 'name' argument overrides.
	// We just use 'name'. User can provide 'name.go'.
	targetPath := filepath.Join(targetDir, name)

	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return fmt.Errorf("failed to save snippet: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Snippet '%s' saved to %s\n", name, targetPath)
	return nil
}

func runSnippetList(cmd *cobra.Command, args []string) error {
	dirs, err := getSnippetDirs()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tLOCATION\tSIZE")

	seen := make(map[string]bool)

	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			// Handle duplicate names (shadowing)
			if seen[f.Name()] {
				continue
			}
			seen[f.Name()] = true

			info, _ := f.Info()
			size := "0 B"
			if info != nil {
				size = fmt.Sprintf("%d B", info.Size())
			}

			// Show relative path if possible
			loc := dir
			if cwd, err := os.Getwd(); err == nil {
				if rel, err := filepath.Rel(cwd, dir); err == nil && !strings.HasPrefix(rel, "..") {
					loc = rel
				} else {
					// Check home
					home, _ := os.UserHomeDir()
					if strings.HasPrefix(dir, home) {
						loc = "~" + strings.TrimPrefix(dir, home)
					}
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\n", f.Name(), loc, size)
		}
	}
	w.Flush()
	return nil
}

func runSnippetShow(cmd *cobra.Command, args []string) error {
	_, content, err := resolveSnippet(args[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), content)
	return nil
}

func runSnippetRemove(cmd *cobra.Command, args []string) error {
	path, _, err := resolveSnippet(args[0])
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove snippet: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Removed snippet '%s'\n", args[0])
	return nil
}

func runSnippetApply(cmd *cobra.Command, args []string) error {
	name := args[0]
	_, content, err := resolveSnippet(name)
	if err != nil {
		return err
	}

	var targetFile string
	if len(args) > 1 {
		targetFile = args[1]
	}

	// AI Processing
	if snippetApplyAI {
		ctx := context.Background()
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		cwd, _ := os.Getwd()

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-snippet")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		// Prepare prompt
		var prompt string
		if targetFile != "" {
			// Read target file context
			targetContent, err := os.ReadFile(targetFile)
			if err != nil {
				// Maybe file doesn't exist yet?
				targetContent = []byte("(New File)")
			}

			prompt = fmt.Sprintf(`You are a coding assistant.
I want to insert the following snippet into the file '%s'.

Snippet:
'''
%s
'''

Target File Context:
'''
%s
'''

Task:
1. Adapt the snippet to the target file's context (e.g. adjust variable names, indentation, imports).
2. Fill in any placeholders (like {{...}} or <...>) in the snippet.
3. Return ONLY the adapted snippet content. Do not include the rest of the file.
`, targetFile, content, string(targetContent))

		} else {
			prompt = fmt.Sprintf(`You are a coding assistant.
Fill in any placeholders in the following snippet with reasonable defaults or generic values.
Return ONLY the code.

Snippet:
'''
%s
'''`, content)
		}

		fmt.Fprintln(cmd.ErrOrStderr(), "🧠 Adapting snippet with AI...")
		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return err
		}
		content = utils.CleanCodeBlock(resp)
	}

	// Output
	if targetFile != "" {
		f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		// Ensure newline before append
		stat, _ := f.Stat()
		if stat.Size() > 0 {
			f.WriteString("\n")
		}

		if _, err := f.WriteString(content); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Applied snippet '%s' to %s\n", name, targetFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), content)
	}

	return nil
}
