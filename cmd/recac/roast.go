package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// roastAgentFactory allows mocking the agent creation specifically for the roast command
var roastAgentFactory = agentClientFactory

var roastCmd = &cobra.Command{
	Use:   "roast [file]",
	Short: "Get a savage code review from an AI Senior Engineer",
	Long: `Selects a file (or picks one at random) and asks the AI to "roast" it.
The AI adopts the persona of a grumpy but brilliant Senior Staff Engineer who has zero patience for bad code.
Expect brutal honesty, sarcasm, and emojis.`,
	RunE: runRoast,
}

func init() {
	rootCmd.AddCommand(roastCmd)
}

func runRoast(cmd *cobra.Command, args []string) error {
	var targetFile string

	if len(args) > 0 {
		targetFile = args[0]
		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", targetFile)
		}
	} else {
		// Pick a random file
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		files, err := listSourceFiles(cwd)
		if err != nil {
			return fmt.Errorf("failed to scan files: %w", err)
		}

		if len(files) == 0 {
			return fmt.Errorf("no source files found in %s", cwd)
		}

		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		targetFile = files[r.Intn(len(files))]
		fmt.Fprintf(cmd.OutOrStdout(), "🎲 Randomly selected: %s\n", targetFile)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// 2. Prepare Agent
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := roastAgentFactory(ctx, provider, model, cwd, "recac-roast")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are a Senior Staff Engineer who has seen it all. You are grumpy, sarcastic, and have zero patience for bad code.
Review the following code file: %s

Your task is to "roast" this code.
- Point out logic errors, bad naming, complexity, and style issues.
- Be savage but constructive. Explain WHY it's bad.
- Use emojis.
- Keep it under 200 words.
- Format your response as a bulleted list of grievances.

Code:
'''
%s
'''`, targetFile, string(content))

	fmt.Fprintln(cmd.OutOrStdout(), "🔥 Heating up the grill...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Output
	printRoast(cmd, targetFile, resp)

	return nil
}

func listSourceFiles(root string) ([]string, error) {
	var files []string
	ignoreMap := DefaultIgnoreMap()

	// Extensions to consider
	exts := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".py": true,
		".rs": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".java": true, ".rb": true, ".php": true, ".sh": true,
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if ignoreMap[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Size() > 100*1024 { // Skip large files > 100KB
			return nil
		}

		ext := filepath.Ext(path)
		if exts[strings.ToLower(ext)] {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func printRoast(cmd *cobra.Command, filename string, roast string) {
	// Style
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")). // Red
		Padding(1, 2).
		Margin(1)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")). // Orange
		Bold(true).
		Underline(true)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")) // White

	title := fmt.Sprintf("🔥 ROAST OF %s 🔥", filepath.Base(filename))
	rendered := borderStyle.Render(
		fmt.Sprintf("%s\n\n%s", titleStyle.Render(title), contentStyle.Render(roast)),
	)

	fmt.Fprintln(cmd.OutOrStdout(), rendered)
}
