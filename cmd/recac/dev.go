package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// devExecCommand allows mocking exec.Command in tests
// We alias this to the UI factory so tests modifying this will affect the UI
var devExecCommand = exec.Command

var (
	devCmdFlag    string
	devWatchDir   string
	devExtensions string
	devRecursive  bool
	devDebounce   time.Duration
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Watch mode for continuous development",
	Long: `Watches for file changes and runs a command (test, build, lint).
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
	// Sync the mock if it was changed (by tests)
	ui.DevExecCmdFactory = devExecCommand

	// 1. Determine Command
	runCommand := devCmdFlag
	if runCommand == "" {
		runCommand = detectDevCommand(devWatchDir)
		if runCommand == "" {
			return fmt.Errorf("could not auto-detect command. Please provide one with --cmd")
		}
		fmt.Printf("ℹ️  Auto-detected command: %s\n", runCommand)
	}

	// 2. Determine Extensions
	exts := parseExtensions(devExtensions, runCommand)
	fmt.Printf("ℹ️  Watching extensions: %v\n", exts)

	// 3. Start TUI
	model, err := ui.NewDevDashboardModel(runCommand, devWatchDir, exts, devRecursive, devDebounce)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
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
