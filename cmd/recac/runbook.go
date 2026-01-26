package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
)

var runbookCmd = &cobra.Command{
	Use:   "runbook [file]",
	Short: "Execute a markdown file as an interactive runbook",
	Long: `Parses a Markdown file, identifies bash/sh code blocks, and executes them interactively.
Preserves environment variables between blocks, allowing for stateful workflows.`,
	Args: cobra.ExactArgs(1),
	RunE: runRunbook,
}

func init() {
	rootCmd.AddCommand(runbookCmd)
	// We could add flags like --auto-approve or --dry-run later
}

type Block struct {
	Type    string // "text" or "code"
	Content string
	Lang    string // "bash", "sh", etc.
}

func runRunbook(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	blocks, err := parseRunbook(filePath)
	if err != nil {
		return err
	}

	// Initialize environment with current process environment
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	// Create a temporary directory for environment files
	tmpDir, err := os.MkdirTemp("", "recac-runbook-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	inputReader := bufio.NewReader(cmd.InOrStdin())

	fmt.Fprintf(cmd.OutOrStdout(), "📖 Running runbook: %s\n", filePath)

	for i, block := range blocks {
		if block.Type == "text" {
			// Render markdown
			out, err := renderer.Render(block.Content)
			if err != nil {
				// Fallback to plain text
				fmt.Fprint(cmd.OutOrStdout(), block.Content)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), out)
			}
		} else if block.Type == "code" {
			// Check if it's a runnable language
			lang := strings.ToLower(block.Lang)
			if lang != "bash" && lang != "sh" && lang != "shell" {
				// Just display, don't run
				out, err := renderer.Render(fmt.Sprintf("```%s\n%s\n```", block.Lang, block.Content))
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "```%s\n%s\n```\n", block.Lang, block.Content)
				} else {
					fmt.Fprint(cmd.OutOrStdout(), out)
				}
				continue
			}

			// It is a runnable block
			// Display the code
			// We manually format it to indicate it's the next step
			fmt.Fprintln(cmd.OutOrStdout(), "\n👉  Step", i+1, "found:")
			// We can use glamour to render just the code block for syntax highlighting
			out, err := renderer.Render(fmt.Sprintf("```%s\n%s\n```", block.Lang, block.Content))
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "```%s\n%s\n```\n", block.Lang, block.Content)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), out)
			}

			// Prompt user
			choice, err := promptUser(cmd, inputReader)
			if err != nil {
				return err
			}

			switch choice {
			case "y":
				newEnv, err := executeBlock(block.Content, env, tmpDir, cmd)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "❌ Command failed: %v\n", err)
					// Ask to continue?
					if !askToContinue(cmd, inputReader) {
						return fmt.Errorf("runbook stopped")
					}
				} else {
					env = newEnv
					fmt.Fprintln(cmd.OutOrStdout(), "✅ Step completed.")
				}
			case "n":
				fmt.Fprintln(cmd.OutOrStdout(), "⏭️  Skipping block.")
			case "q":
				fmt.Fprintln(cmd.OutOrStdout(), "👋 Exiting runbook.")
				return nil
			}
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n🎉 Runbook completed.")
	return nil
}

func parseRunbook(path string) ([]Block, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	var blocks []Block

	var currentBuffer bytes.Buffer
	inCodeBlock := false
	codeLang := ""

	flushBuffer := func(t, l string) {
		if currentBuffer.Len() > 0 {
			blocks = append(blocks, Block{
				Type:    t,
				Content: currentBuffer.String(),
				Lang:    l,
			})
			currentBuffer.Reset()
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCodeBlock {
				// End of code block
				// Do not include the closing fence in the content
				flushBuffer("code", codeLang)
				inCodeBlock = false
				codeLang = ""
			} else {
				// Start of code block
				// Flush previous text
				flushBuffer("text", "")
				inCodeBlock = true
				codeLang = strings.TrimPrefix(strings.TrimSpace(line), "```")
			}
		} else {
			currentBuffer.WriteString(line)
			currentBuffer.WriteString("\n")
		}
	}

	// Flush remaining
	if currentBuffer.Len() > 0 {
		if inCodeBlock {
			// Unclosed code block, treat as code I guess? Or error?
			// Let's treat as code to be robust
			flushBuffer("code", codeLang)
		} else {
			flushBuffer("text", "")
		}
	}

	return blocks, nil
}

