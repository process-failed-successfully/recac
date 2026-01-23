package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var resolveCmd = &cobra.Command{
	Use:   "resolve [file]",
	Short: "Intelligently resolve git merge conflicts using AI",
	Long: `Scans for files with merge conflict markers and uses AI to resolve them.
If a file is specified, it resolves conflicts in that file.
If no file is specified, it scans the current directory for conflicted files.`,
	RunE: runResolve,
}

func init() {
	rootCmd.AddCommand(resolveCmd)
	resolveCmd.Flags().Bool("auto", false, "Automatically apply AI resolutions without confirmation")
}

func runResolve(cmd *cobra.Command, args []string) error {
	auto, _ := cmd.Flags().GetBool("auto")
	ctx := context.Background()

	var files []string
	if len(args) > 0 {
		files = args
	} else {
		// Find files with conflicts
		fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning for conflicts...")
		var err error
		files, err = findConflictedFiles()
		if err != nil {
			return fmt.Errorf("failed to scan for conflicts: %w", err)
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No conflicted files found! 🎉")
		return nil
	}

	for _, file := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "👉 Resolving conflicts in %s...\n", file)
		if err := resolveFileConflicts(ctx, cmd, file, auto); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to resolve %s: %v\n", file, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Resolved %s\n", file)
	}

	return nil
}

func findConflictedFiles() ([]string, error) {
	// Use git grep to find files with conflict markers
	// git grep -l "<<<<<<<"
	c := execCommand("git", "grep", "-l", "<<<<<<<")
	out, err := c.Output()
	if err != nil {
		// grep returns exit code 1 if no matches
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, l := range lines {
		if l != "" {
			result = append(result, l)
		}
	}
	return result, nil
}

func resolveFileConflicts(ctx context.Context, cmd *cobra.Command, filePath string, auto bool) error {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(contentBytes)

	// Regex to find conflict blocks
	// Matches <<<<<<< ... >>>>>>> (including newline)
	// We use a package-level variable or compile here.
	// Since this is not a hot loop (user interaction involves network), compiling here is fine,
	// but for cleanliness:
	re := regexp.MustCompile(`<<<<<<<[^\n]*\n[\s\S]*?>>>>>>>[^\n]*\n?`)

	matches := re.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return fmt.Errorf("no conflict markers found in file")
	}

	var sb strings.Builder
	lastIndex := 0

	for _, match := range matches {
		start := match[0]
		end := match[1]

		sb.WriteString(content[lastIndex:start])

		conflictBlock := content[start:end]

		ours, theirs, err := parseConflictBlock(conflictBlock)
		if err != nil {
			return fmt.Errorf("failed to parse conflict block at offset %d: %w", start, err)
		}

		resolution, err := askAIResolve(ctx, cmd, ours, theirs)
		if err != nil {
			return err
		}

		if !auto {
			fmt.Fprintln(cmd.OutOrStdout(), "\n----------------------------------------")
			fmt.Fprintln(cmd.OutOrStdout(), "Conflict Resolution Proposed:")
			fmt.Fprintln(cmd.OutOrStdout(), "----------------------------------------")
			fmt.Fprintln(cmd.OutOrStdout(), resolution)
			fmt.Fprintln(cmd.OutOrStdout(), "----------------------------------------")

			if !confirm(cmd.OutOrStdout(), cmd.InOrStdin(), "Accept this resolution?") {
				fmt.Fprintln(cmd.OutOrStdout(), "Skipping this conflict (keeping original markers).")
				sb.WriteString(conflictBlock)
				lastIndex = end
				continue
			}
		}

		sb.WriteString(resolution)
		lastIndex = end
	}

	sb.WriteString(content[lastIndex:])

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

func parseConflictBlock(block string) (string, string, error) {
	lines := strings.Split(block, "\n")
	var ours []string
	var theirs []string

	state := 0 // 0: start, 1: ours, 2: base (skip), 3: theirs

	for _, line := range lines {
		if strings.HasPrefix(line, "<<<<<<<") {
			state = 1
			continue
		}
		if strings.HasPrefix(line, "|||||||") {
			state = 2
			continue
		}
		if strings.HasPrefix(line, "=======") {
			state = 3
			continue
		}
		if strings.HasPrefix(line, ">>>>>>>") {
			break
		}

		switch state {
		case 1:
			ours = append(ours, line)
		case 2:
			// skip base
		case 3:
			theirs = append(theirs, line)
		}
	}

	return strings.Join(ours, "\n"), strings.Join(theirs, "\n"), nil
}

func askAIResolve(ctx context.Context, cmd *cobra.Command, ours, theirs string) (string, error) {
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-resolve")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`You are an expert software engineer resolving a git merge conflict.
Below are the two conflicting versions of code.
Please verify the intent of both changes and combine them intelligently.
If they are mutually exclusive, choose the most logical one or merge them if possible.

Version A (Current/Ours):
'''
%s
'''

Version B (Incoming/Theirs):
'''
%s
'''

Return ONLY the resolved code. Do not include markdown formatting or explanation.
`, ours, theirs)

	fmt.Fprint(cmd.OutOrStdout(), "🤖 Asking AI to resolve conflict...\n")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return "", err
	}

	return utils.CleanCodeBlock(resp), nil
}

func confirm(w io.Writer, r io.Reader, question string) bool {
	fmt.Fprintf(w, "%s [y/N]: ", question)
	reader := bufio.NewReader(r)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
