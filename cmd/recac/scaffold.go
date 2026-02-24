package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Scaffold new components using AI",
	Long:  `Generate boilerplate code for new CLI commands, APIs, and other components using AI assistance.`,
}

var scaffoldCommandCmd = &cobra.Command{
	Use:   "command [name]",
	Short: "Scaffold a new CLI command",
	Long:  `Generates a new Cobra command file in cmd/recac/ (if exists) or current directory with the specified name and description.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runScaffoldCommand,
}

func init() {
	rootCmd.AddCommand(scaffoldCmd)
	scaffoldCmd.AddCommand(scaffoldCommandCmd)

	scaffoldCommandCmd.Flags().StringP("description", "d", "", "Description of what the command should do")
}

func runScaffoldCommand(cmd *cobra.Command, args []string) error {
	name := args[0]
	// Sanitize name to prevent path traversal
	baseName := filepath.Base(name)
	if baseName == "." || baseName == "/" {
		return fmt.Errorf("invalid command name: %s", name)
	}

	desc, _ := cmd.Flags().GetString("description")
	if desc == "" {
		desc = baseName
	}

	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-scaffold")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Determine target directory
	targetDir := "."
	// If running from repo root, prefer cmd/recac
	if _, err := os.Stat("cmd/recac"); err == nil {
		targetDir = "cmd/recac"
	}

	// 1. Generate Command File
	prompt := fmt.Sprintf(`Generate a Go file for a new Cobra command in the 'recac' CLI.
The command name is "%s" and its purpose is: "%s".
The file will be placed in 'cmd/recac/%s.go'.

Requirements:
- Package: 'main'
- Global variable: 'var %sCmd = &cobra.Command{...}'
- Use: "%s"
- Short description: "%s"
- RunE function that implements the logic.
- An 'init()' function that adds '%sCmd' to 'rootCmd'.
- Add flags if relevant to the description.
- Import necessary packages (including "github.com/spf13/cobra" and others).
- Follow Go best practices.

Output ONLY the Go code for the file.`, baseName, desc, baseName, baseName, baseName, desc, baseName)

	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating command '%s'...\n", baseName)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	code := utils.CleanCodeBlock(resp)

	filename := filepath.Join(targetDir, fmt.Sprintf("%s.go", baseName))
	// Check if file exists?
	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("file %s already exists", filename)
	}

	if err := os.WriteFile(filename, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Created %s\n", filename)

	// 2. Generate Test File
	testName := capitalize(baseName)
	testPrompt := fmt.Sprintf(`Generate a Go test file for the command '%s'.
The command file content is:
'''go
%s
'''

Requirements:
- Package: 'main'
- Test function: 'Test%sCommand(t *testing.T)'
- Use 'bytes.Buffer' to capture output.
- Set 'cmd.SetOut(&buf)' and 'cmd.SetErr(&buf)'.
- Invoke 'cmd.Execute()'.
- Assert success and output contains expected strings.
- Output ONLY the Go code for the test file.`, baseName, code, testName)

	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating tests for '%s'...\n", baseName)

	testResp, err := ag.Send(ctx, testPrompt)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to generate tests: %v\n", err)
		return nil
	}

	testCode := utils.CleanCodeBlock(testResp)
	testFilename := filepath.Join(targetDir, fmt.Sprintf("%s_test.go", baseName))

	if err := os.WriteFile(testFilename, []byte(testCode), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", testFilename, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Created %s\n", testFilename)

	return nil
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
