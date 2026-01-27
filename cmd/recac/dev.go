package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"
)

// devExecCommand allows mocking exec.Command in tests
var devExecCommand = exec.Command

var (
	devCmdFlag     string
	devWatchDir    string
	devExtensions  string
	devRecursive   bool
	devDebounce    time.Duration
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Watch mode for continuous development",
	Long:  `Watches for file changes and runs a command (test, build, lint).
Auto-detects the project type (Go, Node, Make) if no command is provided.`,
	RunE: runDev,
}

func init() {
	rootCmd.AddCommand(devCmd)
	devCmd.Flags().StringVarP(&devCmdFlag, "cmd", "c", "", "Command to run on change")
	devCmd.Flags().StringVarP(&devWatchDir, "watch", "w", ".", "Directory to watch")
	devCmd.Flags().StringVarP(&devExtensions, "ext", "e", "", "Extensions to trigger on (comma separated)")
	devCmd.Flags().BoolVarP(&devRecursive, "recursive", "r", true, "Watch directories recursively")
	devCmd.Flags().DurationVarP(&devDebounce, "debounce", "d", 500*time.Millisecond, "Debounce duration")
}

func runDev(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. Determine Command
	runCommand := devCmdFlag
	if runCommand == "" {
		runCommand = detectDevCommand(devWatchDir)
		if runCommand == "" {
			return fmt.Errorf("could not auto-detect command. Please provide one with --cmd")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Auto-detected command: %s\n", runCommand)
	}

	// 2. Determine Extensions
	exts := parseExtensions(devExtensions, runCommand)
	fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Watching extensions: %v\n", exts)

	// 3. Setup Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// 4. Add Paths
	if devRecursive {
		if err := devAddRecursiveWatch(watcher, devWatchDir); err != nil {
			return err
		}
	} else {
		if err := watcher.Add(devWatchDir); err != nil {
			return err
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "👀 Watching %s for changes...\n", devWatchDir)

	// 5. Watch Loop
	var timer *time.Timer
	var mu sync.Mutex

	// Channel to signal execution
	trigger := make(chan struct{}, 1)

	// Initial run
	go func() { trigger <- struct{}{} }()

	// Event Loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Filter events
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Rename == fsnotify.Rename {
					// Check extension
					if shouldTrigger(event.Name, exts) {
						mu.Lock()
						if timer != nil {
							timer.Stop()
						}
						timer = time.AfterFunc(devDebounce, func() {
							trigger <- struct{}{}
						})
						mu.Unlock()
					}

					// If new directory created, add to watcher
					if devRecursive && event.Op&fsnotify.Create == fsnotify.Create {
						fi, err := os.Stat(event.Name)
						if err == nil && fi.IsDir() {
							watcher.Add(event.Name)
						}
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Watcher error: %v\n", err)
			}
		}
	}()

	// Execution Loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-trigger:
				executeDevCommand(cmd, runCommand)
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func detectDevCommand(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go test ./..."
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "npm test"
	}
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); err == nil {
		return "make"
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return "pytest"
	}
	return ""
}

func parseExtensions(flagExt, cmd string) []string {
	if flagExt != "" {
		parts := strings.Split(flagExt, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
			if !strings.HasPrefix(parts[i], ".") {
				parts[i] = "." + parts[i]
			}
		}
		return parts
	}

	// Infer from command or project type
	if strings.Contains(cmd, "go ") {
		return []string{".go", ".mod"}
	}
	if strings.Contains(cmd, "npm") || strings.Contains(cmd, "node") {
		return []string{".js", ".ts", ".json"}
	}
	if strings.Contains(cmd, "pytest") || strings.Contains(cmd, "python") {
		return []string{".py"}
	}
	if strings.Contains(cmd, "make") {
		return []string{".go", ".c", ".cpp", ".h", ".rs"} // Generic mix
	}

	return []string{} // Watch all? No, safer to default to common code files if unknown
}

func shouldTrigger(path string, exts []string) bool {
	if len(exts) == 0 {
		return true
	}
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func devAddRecursiveWatch(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			// Skip hidden directories (like .git, .recac)
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			// Skip dependency and build directories
			if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "target" || name == "bin" {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}

func executeDevCommand(cobraCmd *cobra.Command, cmdStr string) {
	fmt.Fprintln(cobraCmd.OutOrStdout(), "\n🔄 Running...")
	start := time.Now()

	// Split command string
	parts, err := shellquote.Split(cmdStr)
	if err != nil {
		fmt.Fprintf(cobraCmd.ErrOrStderr(), "\n❌ Failed to parse command: %v\n", err)
		return
	}
	if len(parts) == 0 {
		return
	}

	head := parts[0]
	args := parts[1:]

	cmd := devExecCommand(head, args...)
	cmd.Stdout = cobraCmd.OutOrStdout()
	cmd.Stderr = cobraCmd.ErrOrStderr()
	// Use a pipe or buffer if we want to capture output for analysis, but direct to stdout is fine for dev mode

	// Clear screen? Maybe not, it can be annoying.

	err = cmd.Run()
	duration := time.Since(start).Round(time.Millisecond)

	if err != nil {
		fmt.Fprintf(cobraCmd.OutOrStdout(), "\n❌ Failed (%s)\n", duration)
	} else {
		fmt.Fprintf(cobraCmd.OutOrStdout(), "\n✅ Passed (%s)\n", duration)
	}
	fmt.Fprintln(cobraCmd.OutOrStdout(), "--------------------------------------------------")
}

// Needed for mocking Stdin/Stdout in executeRunCmd but not dev command usually
// But we might want to ensure IO is not blocking.
func copyIO(w io.Writer, r io.Reader) {
	io.Copy(w, r)
}
