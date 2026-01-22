package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	fuzzFunc     string
	fuzzDuration string
	fuzzKeep     bool
)

var fuzzCmd = &cobra.Command{
	Use:   "fuzz [file]",
	Short: "Generate and run fuzz tests using AI",
	Long: `Generates a native Go fuzz test target for a function in the specified file using AI,
and then runs the fuzzer to find edge cases and crashes.`,
	Args: cobra.ExactArgs(1),
	RunE: runFuzz,
}

func init() {
	rootCmd.AddCommand(fuzzCmd)
	fuzzCmd.Flags().StringVarP(&fuzzFunc, "func", "f", "", "Name of the function to fuzz (if empty, tries to infer or prompt)")
	fuzzCmd.Flags().StringVarP(&fuzzDuration, "duration", "d", "10s", "Duration to run the fuzzer (e.g., 10s, 1m)")
	fuzzCmd.Flags().BoolVar(&fuzzKeep, "keep", true, "Keep the generated fuzz test file after running")
}

func runFuzz(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Identify package name
	pkgName := getPackageName(string(content))
	if pkgName == "" {
		return fmt.Errorf("could not determine package name from %s", filePath)
	}

	// 2. Identify function if not provided
	targetFunc := fuzzFunc
	if targetFunc == "" {
		// Simple regex to find exported functions
		// This is a heuristic. A real parser would be better but this suffices for CLI.
		re := regexp.MustCompile(`func ([A-Z][a-zA-Z0-9_]*)`)
		matches := re.FindAllStringSubmatch(string(content), -1)

		if len(matches) == 0 {
			return fmt.Errorf("no exported functions found in %s", filePath)
		}

		if len(matches) == 1 {
			targetFunc = matches[0][1]
			fmt.Fprintf(cmd.OutOrStdout(), "Auto-selected function: %s\n", targetFunc)
		} else {
			// If interactive, we could ask. For now, let's error and list.
			var funcs []string
			for _, m := range matches {
				funcs = append(funcs, m[1])
			}
			return fmt.Errorf("multiple exported functions found. Please specify one with --func:\n%s", strings.Join(funcs, ", "))
		}
	}

	// 3. Generate Fuzz Target
	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating fuzz target for '%s'...\n", targetFunc)

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-fuzz")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert Go Security Engineer.
Create a Go 1.18+ native fuzz test (using testing.F) for the function '%s' in the following code.
The package name MUST be '%s'.
Do not import "github.com/dvyukov/go-fuzz/go-fuzz-dep" or similar. Use only "testing".
Include a valid seed corpus using f.Add().
Wrap the output in a markdown code block.

File Content:
%s
`, targetFunc, pkgName, string(content))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	fuzzCode := utils.CleanCodeBlock(resp)
	if fuzzCode == "" {
		fuzzCode = extractFuzzCodeBlock(resp) // Fallback to local helper
	}

	// 4. Write to file
	// We construct a name like name_fuzz_test.go
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	fuzzFileName := filepath.Join(dir, fmt.Sprintf("%s_fuzz_test.go", nameWithoutExt))

	if err := os.WriteFile(fuzzFileName, []byte(fuzzCode), 0644); err != nil {
		return fmt.Errorf("failed to write fuzz file: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "📄 Created fuzz test: %s\n", fuzzFileName)

	// 5. Run Fuzzer
	// We need to know the name of the Fuzz function.
	// We can regex it from the generated code.
	reFuzzFunc := regexp.MustCompile(`func (Fuzz[a-zA-Z0-9_]*)`)
	match := reFuzzFunc.FindStringSubmatch(fuzzCode)
	if len(match) < 2 {
		return fmt.Errorf("could not find Fuzz function name in generated code")
	}
	fuzzTargetName := match[1]

	fmt.Fprintf(cmd.OutOrStdout(), "🔥 Running fuzzer %s for %s...\n", fuzzTargetName, fuzzDuration)

	// go test -fuzz=^FuzzName$ -fuzztime=10s .
	// We run in the directory of the file

	// Prepare command
	testArgs := []string{"test", "-fuzz", fmt.Sprintf("^%s$", fuzzTargetName), "-fuzztime", fuzzDuration}

	// If the file is in a subdirectory, we need to be careful.
	// If we run `go test` from root with `./pkg/...`, it might be weird for fuzzing specific target.
	// Better to change directory or use package path.
	// Changing directory is easier for `go test .`

	// Note: execCommand is global var, so we can't easily change Dir on it if we assume it's just a function returning *Cmd.
	// But it returns *exec.Cmd, so we can set Dir.

	c := execCommand("go", testArgs...)
	c.Dir = dir // Run in the package directory

	// Pipe output
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()

	if err := c.Run(); err != nil {
		// Fuzzing failed (found a crash) or other error
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n❌ Fuzzer found a crash or failed!\n")
			// We could offer to fix it here
			return fmt.Errorf("fuzzing failed")
		}
		return fmt.Errorf("failed to run go test: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n✅ Fuzzing complete. No crashes found.")

	// 6. Cleanup
	if !fuzzKeep {
		os.Remove(fuzzFileName)
		fmt.Fprintf(cmd.OutOrStdout(), "🗑️  Deleted %s\n", fuzzFileName)
	}

	return nil
}

func getPackageName(content string) string {
	re := regexp.MustCompile(`package\s+([a-zA-Z0-9_]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func extractFuzzCodeBlock(response string) string {
	// Simple extractor for ``` code blocks
	start := strings.Index(response, "```")
	if start == -1 {
		return response
	}

	// Skip the opening ``` and optional language identifier
	rest := response[start+3:]
	newline := strings.Index(rest, "\n")
	if newline != -1 {
		rest = rest[newline+1:]
	}

	end := strings.LastIndex(rest, "```")
	if end == -1 {
		return rest // No closing block, return everything from start
	}

	return rest[:end]
}
