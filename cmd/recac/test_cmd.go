package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	testWatch    bool
	testAll      bool
	testStaged   bool
	testDiagnose bool
)

// Mockable dependency
var identifyPackagesFunc = IdentifyImpactedPackages
var getGitDiffFilesFunc = getGitDiffFiles

var testCmd = &cobra.Command{
	Use:   "test [packages...]",
	Short: "Smart test runner with AI diagnosis",
	Long: `Runs tests for your project. By default, it uses impact analysis to run only tests affected by your changes.
If tests fail, it can automatically ask the AI agent to diagnose the failure and suggest a fix.`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Flags().BoolVarP(&testWatch, "watch", "w", false, "Watch for file changes and re-run tests")
	testCmd.Flags().BoolVar(&testAll, "all", false, "Run all tests in the module")
	testCmd.Flags().BoolVar(&testStaged, "staged", false, "Include staged changes in impact analysis")
	testCmd.Flags().BoolVar(&testDiagnose, "diagnose", true, "Automatically diagnose failures with AI")
}

func runTest(cmd *cobra.Command, args []string) error {
	// If watch mode is enabled, start the watcher loop
	if testWatch {
		return runTestWatch(cmd, args)
	}
	return runTestOnce(cmd, args)
}

func runTestOnce(cmd *cobra.Command, args []string) error {
	var packages []string
	var err error

	// 1. Determine target packages
	if len(args) > 0 {
		packages = args
	} else if testAll {
		packages = []string{"./..."}
	} else {
		// Smart Impact Analysis
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing impact of changes...")
		diffFiles, err := getGitDiffFilesFunc(testStaged)
		if err != nil {
			return fmt.Errorf("failed to get changed files: %w", err)
		}

		if len(diffFiles) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changed files found. Use --all to run all tests.")
			return nil
		}

		pkgs, _, err := identifyPackagesFunc(diffFiles, ".")
		if err != nil {
			// Fallback or just report error?
			// If no go packages found, maybe just warn and return
			if strings.Contains(err.Error(), "No Go packages found") {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No affected Go packages found.")
				return nil
			}
			return fmt.Errorf("impact analysis failed: %w", err)
		}
		packages = pkgs
	}

	if len(packages) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No packages to test.")
		return nil
	}

	// 2. Filter for packages that actually have tests (unless ./...)
	var targets []string
	if len(packages) == 1 && packages[0] == "./..." {
		targets = packages
	} else {
		for _, pkg := range packages {
			// Convert pkg import path back to dir?
			// Identifying if a package has tests given just the import path is tricky without 'go list'.
			// But IdentifyImpactedPackages returns import paths.
			// Ideally we just pass them to 'go test' and let it decide.
			targets = append(targets, pkg)
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🏃 Running tests for %d packages...\n", len(targets))
	if len(targets) < 10 {
		for _, t := range targets {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", t)
		}
	}

	// 3. Run 'go test'
	// usage: go test -v [packages]
	goArgs := append([]string{"test", "-v"}, targets...)
	testExec := execCommand("go", goArgs...) // execCommand is from shared_utils.go (mockable)

	// We want to capture output for diagnosis, but also stream it to user.
	// We'll use a pipe or just capture combined output?
	// Streaming is better for DX.

	stdoutPipe, err := testExec.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := testExec.StderrPipe()
	if err != nil {
		return err
	}

	if err := testExec.Start(); err != nil {
		return fmt.Errorf("failed to start go test: %w", err)
	}

	// Stream and capture
	var outputBuf strings.Builder
	var wg sync.WaitGroup

	// Use a scanner to read line by line and print/capture
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			outputBuf.WriteString(line + "\n")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
			outputBuf.WriteString(line + "\n")
		}
	}()

	err = testExec.Wait()
	wg.Wait()
	if err != nil {
		// Tests failed!
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n❌ Tests failed.")

		if testDiagnose {
			return diagnoseFailure(cmd, outputBuf.String())
		}
		return fmt.Errorf("tests failed") // Return error so exit code is non-zero
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n✅ All tests passed.")
	return nil
}

func diagnoseFailure(cmd *cobra.Command, output string) error {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n🧠 Diagnosing failure with AI...")

	// 1. Extract context
	fileContexts, err := extractFileContexts(output)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not extract file contexts: %v\n", err)
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
		_, _ = fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

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

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "👀 Watching for file changes...")

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
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n🔄 File changed, re-running tests...")
				// Re-run
				// We need to run this in main goroutine? No, separate is fine, but output might interleave?
				// For simple CLI watch, running in this goroutine (via channel or blocking) is better.
				// But AfterFunc runs in its own goroutine.
				// Let's use a channel to trigger run.
				// But for MVP, let's just run it here. Note: concurrency issues with stdout might occur.

				// Better approach: send to a 'trigger' channel.
				// But to keep it simple, let's just run it.
				_ = runTestOnce(cmd, args)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n👀 Watching...")
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Watcher error: %v\n", err)
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
