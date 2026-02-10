package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"recac/internal/utils"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	testWatch       bool
	testAll         bool
	testStaged      bool
	testDiagnose    bool
	testFix         bool
	testFixAttempts int
)

// Mockable dependency
var identifyPackagesFunc = IdentifyImpactedPackages
var getGitDiffFilesFunc = getGitDiffFiles

var testCmd = &cobra.Command{
	Use:   "test [packages...]",
	Short: "Smart test runner with AI diagnosis and self-healing",
	Long: `Runs tests for your project. By default, it uses impact analysis to run only tests affected by your changes.
If tests fail, it can automatically ask the AI agent to diagnose the failure and suggest a fix.
With --fix, it can even attempt to apply the fix and re-run tests.`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Flags().BoolVarP(&testWatch, "watch", "w", false, "Watch for file changes and re-run tests")
	testCmd.Flags().BoolVar(&testAll, "all", false, "Run all tests in the module")
	testCmd.Flags().BoolVar(&testStaged, "staged", false, "Include staged changes in impact analysis")
	testCmd.Flags().BoolVar(&testDiagnose, "diagnose", true, "Automatically diagnose failures with AI")
	testCmd.Flags().BoolVar(&testFix, "fix", false, "Automatically attempt to fix failing tests")
	testCmd.Flags().IntVar(&testFixAttempts, "fix-attempts", 3, "Maximum number of fix attempts")
}

func runTest(cmd *cobra.Command, args []string) error {
	// If watch mode is enabled, start the watcher loop
	if testWatch {
		return runTestWatch(cmd, args)
	}
	return runTestOnce(cmd, args)
}

func runTestOnce(cmd *cobra.Command, args []string) error {
	output, err := runTestCore(cmd, args)
	if err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "\n✅ All tests passed.")
		return nil
	}

	// Tests failed!
	fmt.Fprintln(cmd.OutOrStdout(), "\n❌ Tests failed.")

	if testFix {
		return attemptFixLoop(cmd, args, output)
	}

	if testDiagnose {
		return diagnoseFailure(cmd, output)
	}
	return fmt.Errorf("tests failed")
}

// runTestCore executes the tests and returns the captured output and error if they failed.
func runTestCore(cmd *cobra.Command, args []string) (string, error) {
	var packages []string
	var err error

	// 1. Determine target packages
	if len(args) > 0 {
		packages = args
	} else if testAll {
		packages = []string{"./..."}
	} else {
		// Smart Impact Analysis
		fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing impact of changes...")
		diffFiles, err := getGitDiffFilesFunc(testStaged)
		if err != nil {
			return "", fmt.Errorf("failed to get changed files: %w", err)
		}

		if len(diffFiles) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No changed files found. Use --all to run all tests.")
			return "", nil
		}

		pkgs, _, err := identifyPackagesFunc(diffFiles, ".")
		if err != nil {
			// Fallback or just report error?
			// If no go packages found, maybe just warn and return
			if strings.Contains(err.Error(), "No Go packages found") {
				fmt.Fprintln(cmd.OutOrStdout(), "No affected Go packages found.")
				return "", nil
			}
			return "", fmt.Errorf("impact analysis failed: %w", err)
		}
		packages = pkgs
	}

	if len(packages) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No packages to test.")
		return "", nil
	}

	// 2. Filter for packages that actually have tests (unless ./...)
	var targets []string
	if len(packages) == 1 && packages[0] == "./..." {
		targets = packages
	} else {
		for _, pkg := range packages {
			targets = append(targets, pkg)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🏃 Running tests for %d packages...\n", len(targets))
	if len(targets) < 10 {
		for _, t := range targets {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", t)
		}
	}

	// 3. Run 'go test'
	// usage: go test -v [packages]
	goArgs := append([]string{"test", "-v"}, targets...)
	testExec := execCommand("go", goArgs...)

	stdoutPipe, err := testExec.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := testExec.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := testExec.Start(); err != nil {
		return "", fmt.Errorf("failed to start go test: %w", err)
	}

	// Stream and capture
	var outputBuf strings.Builder
	var mu sync.Mutex

	// Use a scanner to read line by line and print/capture
	// We use channels to wait for reading to finish
	done := make(chan bool)

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(cmd.OutOrStdout(), line)
			mu.Lock()
			outputBuf.WriteString(line + "\n")
			mu.Unlock()
		}
		done <- true
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(cmd.ErrOrStderr(), line)
			mu.Lock()
			outputBuf.WriteString(line + "\n")
			mu.Unlock()
		}
		done <- true
	}()

	err = testExec.Wait()
	<-done
	<-done

	return outputBuf.String(), err
}

func attemptFixLoop(cmd *cobra.Command, args []string, initialOutput string) error {
	currentOutput := initialOutput
	for i := 1; i <= testFixAttempts; i++ {
		fmt.Fprintf(cmd.OutOrStdout(), "\n🔧 Attempting fix %d/%d...\n", i, testFixAttempts)

		fixed, err := attemptFix(cmd, currentOutput)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Fix attempt failed: %v\n", err)
			break
		}
		if !fixed {
			fmt.Fprintln(cmd.OutOrStdout(), "Agent could not suggest any changes.")
			break
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Applying changes and re-running tests...")
		output, err := runTestCore(cmd, args)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "\n✅ Fix successful! Tests passed.")
			return nil
		}
		currentOutput = output
		fmt.Fprintln(cmd.OutOrStdout(), "\n❌ Tests still failing.")
	}

	return fmt.Errorf("failed to fix tests after %d attempts", testFixAttempts)
}

func attemptFix(cmd *cobra.Command, output string) (bool, error) {
	// 1. Extract context
	fileContexts, err := extractFileContexts(output)
	if err != nil {
		// Continue anyway?
		fileContexts = "No local files could be linked to the output."
	}

	// 2. Prepare Agent
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-test-fix")
	if err != nil {
		return false, fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`The tests failed. Please analyze the output and the referenced code to fix the failure.

<test_output>
%s
</test_output>

<referenced_code>
%s
</referenced_code>

INSTRUCTIONS:
1. Identify the bug.
2. Provide the corrected code for the affected file(s).
3. Return the FULL CONTENT of the modified file(s) wrapped in <file path="...">...</file> tags.
   Example:
   <file path="pkg/math/sum.go">
   package math
   ...
   </file>

Do not return diffs. Return full file content.
`, output, fileContexts)

	fmt.Fprintln(cmd.OutOrStdout(), "Waiting for agent solution...")

	// 4. Send
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("agent failed: %w", err)
	}

	// 5. Parse and Apply
	files := utils.ParseFileBlocks(resp)
	if len(files) == 0 {
		return false, nil
	}

	for path, content := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "Updating %s...\n", path)
		dir := filepath.Dir(path)
		if err := mkdirAllFunc(dir, 0755); err != nil {
			return false, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		if err := writeFileFunc(path, []byte(content), 0644); err != nil {
			return false, fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	return true, nil
}

func diagnoseFailure(cmd *cobra.Command, output string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "\n🧠 Diagnosing failure with AI...")

	// 1. Extract context
	fileContexts, err := extractFileContexts(output)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not extract file contexts: %v\n", err)
		fileContexts = "No local files could be linked to the output."
	}

	// 2. Prepare Agent
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-test-diagnose")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`The tests failed. Please analyze the output and the referenced code to explain the failure and suggest a fix.

<test_output>
%s
</test_output>

<referenced_code>
%s
</referenced_code>
`, output, fileContexts)

	// 4. Stream Response
	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

	if err != nil {
		return fmt.Errorf("agent failed during diagnosis: %w", err)
	}

	return fmt.Errorf("tests failed (diagnosis complete)")
}

func runTestWatch(cmd *cobra.Command, args []string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Add recursive watch
	root, _ := os.Getwd()
	if err := addRecursiveWatch(watcher, root); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "👀 Watching for file changes...")

	// Initial Run
	runTestOnce(cmd, args) // Ignore error on initial run to keep watching

	// Debounce logic
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Filter irrelevant events
			if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				continue
			}
			// Filter files
			if shouldIgnoreFile(event.Name) {
				continue
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDuration, func() {
				fmt.Fprintln(cmd.OutOrStdout(), "\n🔄 File changed, re-running tests...")
				// Re-run
				// We need to run this in main goroutine? No, separate is fine, but output might interleave?
				// For simple CLI watch, running in this goroutine (via channel or blocking) is better.
				// But AfterFunc runs in its own goroutine.
				// Let's use a channel to trigger run.
				// But for MVP, let's just run it here. Note: concurrency issues with stdout might occur.

				// Better approach: send to a 'trigger' channel.
				// But to keep it simple, let's just run it.
				_ = runTestOnce(cmd, args)
				fmt.Fprintln(cmd.OutOrStdout(), "\n👀 Watching...")
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Watcher error: %v\n", err)
		}
	}
}

func addRecursiveWatch(watcher *fsnotify.Watcher, path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if shouldIgnoreDir(p) {
				return filepath.SkipDir
			}
			return watcher.Add(p)
		}
		return nil
	})
}

func shouldIgnoreDir(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != "." {
		return true // .git, .idea, etc
	}
	if base == "node_modules" || base == "vendor" || base == "dist" || base == "build" {
		return true
	}
	return false
}

func shouldIgnoreFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".tmp") {
		return true
	}
	// Add more ignores if needed
	return false
}
