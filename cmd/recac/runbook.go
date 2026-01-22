package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
)

// runbookExecCommand allows mocking in tests
var runbookExecCommand = exec.Command

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
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var blocks []Block
	scanner := bufio.NewScanner(f)

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

	for scanner.Scan() {
		line := scanner.Text()

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

	return blocks, scanner.Err()
}

func promptUser(cmd *cobra.Command, reader *bufio.Reader) (string, error) {
	for {
		fmt.Fprint(cmd.OutOrStdout(), "Execute this block? [y]es, [n]o, [q]uit: ")
		input, err := reader.ReadString('\n')

		cleanInput := strings.TrimSpace(strings.ToLower(input))
		if cleanInput == "y" || cleanInput == "yes" {
			return "y", nil
		}
		if cleanInput == "n" || cleanInput == "no" {
			return "n", nil
		}
		if cleanInput == "q" || cleanInput == "quit" {
			return "q", nil
		}

		if err != nil {
			// EOF or error, default to quit
			return "q", nil
		}
	}
}

func askToContinue(cmd *cobra.Command, reader *bufio.Reader) bool {
	fmt.Fprint(cmd.OutOrStdout(), "Command failed. Continue anyway? [y/N]: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func executeBlock(code string, env map[string]string, tmpDir string, cmd *cobra.Command) (map[string]string, error) {
	// 1. Prepare command
	// We run: sh -c "<code>; env > env.new"
	// We handle set -e to stop on error within the block
	outFile := filepath.Join(tmpDir, "env_out.txt")
	// Make sure outFile doesn't exist
	os.Remove(outFile)

	// Use sh or bash
	shell := "bash"
	if _, err := exec.LookPath("bash"); err != nil {
		shell = "sh"
	}

	// We wrap the user code to ensure we capture env even if it's multiple lines
	wrappedCode := fmt.Sprintf("set -e\n%s\nenv > '%s'", code, outFile)

	runCmd := runbookExecCommand(shell, "-c", wrappedCode)
	// We stream stdout/stderr
	runCmd.Stdout = cmd.OutOrStdout()
	runCmd.Stderr = cmd.ErrOrStderr()

	var envList []string
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	runCmd.Env = envList

	if err := runCmd.Run(); err != nil {
		return nil, err
	}

	// 2. Read new env
	outData, err := os.ReadFile(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output env: %w", err)
	}

	newEnv := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(outData))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			newEnv[parts[0]] = parts[1]
		}
	}

	return newEnv, nil
}
