package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// pairCmd represents the pair command
var pairCmd = &cobra.Command{
	Use:   "pair [path]",
	Short: "Real-time AI pair programmer",
	Long: `Starts a session that monitors your file edits and provides immediate AI feedback.
It watches for file changes in the specified directory (defaulting to current) and
uses the configured agent to review the code for bugs, style, and improvements.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPair,
}

var (
	pairDebounce time.Duration
)

func init() {
	rootCmd.AddCommand(pairCmd)
	pairCmd.Flags().DurationVar(&pairDebounce, "debounce", 2*time.Second, "Time to wait after last edit before analyzing")
}

// FileWatcher interface allows mocking fsnotify
type FileWatcher interface {
	Events() <-chan fsnotify.Event
	Errors() <-chan error
	Add(name string) error
	Close() error
	AddRecursive(root string) error
}

// FSNotifyWatcher wraps fsnotify.Watcher
type FSNotifyWatcher struct {
	watcher *fsnotify.Watcher
}

func NewFSNotifyWatcher() (*FSNotifyWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FSNotifyWatcher{watcher: w}, nil
}

func (w *FSNotifyWatcher) Events() <-chan fsnotify.Event {
	return w.watcher.Events
}

func (w *FSNotifyWatcher) Errors() <-chan error {
	return w.watcher.Errors
}

func (w *FSNotifyWatcher) Add(name string) error {
	return w.watcher.Add(name)
}

func (w *FSNotifyWatcher) Close() error {
	return w.watcher.Close()
}

func (w *FSNotifyWatcher) AddRecursive(root string) error {
	ignoreMap := DefaultIgnoreMap()

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreMap[d.Name()] {
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
				return filepath.SkipDir
			}
			return w.Add(path)
		}
		return nil
	})
}

// factory for creating watcher, allows mocking
var watcherFactory = func() (FileWatcher, error) {
	return NewFSNotifyWatcher()
}

func runPair(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	watcher, err := watcherFactory()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Initial add
	fmt.Fprintf(cmd.OutOrStdout(), "👀 Watching %s for changes...\n", absRoot)
	if err := watcher.AddRecursive(absRoot); err != nil {
		return fmt.Errorf("failed to watch path: %w", err)
	}

	// Map to track pending updates per file
	var mu sync.Mutex
	var outMu sync.Mutex
	var wg sync.WaitGroup
	timers := make(map[string]*time.Timer)

	// Use command context for cancellation
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	// Pre-create agent client to ensure config is valid
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-pair")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case event, ok := <-watcher.Events():
				if !ok {
					return
				}

				// Only care about Write events for analysis
				// Create events might need adding to watcher if it's a dir
				if event.Has(fsnotify.Create) {
					info, err := os.Stat(event.Name)
					if err == nil && info.IsDir() {
						watcher.Add(event.Name) // Watch new directory
					}
				}

				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
					continue
				}

				filename := event.Name
				// Check ignore
				base := filepath.Base(filename)
				if DefaultIgnoreMap()[base] {
					continue
				}
				if strings.HasPrefix(base, ".") {
					continue
				}

				// Debounce logic
				mu.Lock()
				if t, exists := timers[filename]; exists {
					t.Stop()
				}
				var t *time.Timer
				wg.Add(1)
				t = time.AfterFunc(pairDebounce, func() {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					default:
					}
					analyzeFile(ctx, cmd, ag, filename, &outMu)
					mu.Lock()
					// Only delete if it's still the same timer
					if timers[filename] == t {
						delete(timers, filename)
					}
					mu.Unlock()
				})
				timers[filename] = t
				mu.Unlock()

			case err, ok := <-watcher.Errors():
				if !ok {
					return
				}
				outMu.Lock()
				fmt.Fprintf(cmd.ErrOrStderr(), "Watcher error: %v\n", err)
				outMu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	wg.Wait()
	return nil
}

func analyzeFile(ctx context.Context, cmd *cobra.Command, ag agent.Agent, path string, outMu *sync.Mutex) {
	// Re-check file existence
	info, err := os.Stat(path)
	if err != nil {
		return // File might have been deleted
	}
	if info.IsDir() {
		return
	}

	// Check binary
	ext := strings.ToLower(filepath.Ext(path))
	if isBinaryExt(ext) {
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		outMu.Lock()
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to read %s: %v\n", path, err)
		outMu.Unlock()
		return
	}
	if isBinaryContent(content) {
		return
	}

	outMu.Lock()
	fmt.Fprintf(cmd.OutOrStdout(), "\n📝 Detected change in %s. Analyzing...\n", filepath.Base(path))
	outMu.Unlock()

	prompt := fmt.Sprintf(`Review the following code file: %s
Identify potential bugs, logic errors, security vulnerabilities, and major style issues.
Be concise. If the code looks good, just say "LGTM".

Code:
'''%s
%s
'''`, filepath.Base(path), strings.TrimPrefix(ext, "."), string(content))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		outMu.Lock()
		fmt.Fprintf(cmd.ErrOrStderr(), "Agent failed: %v\n", err)
		outMu.Unlock()
		return
	}

	outMu.Lock()
	defer outMu.Unlock()
	if strings.Contains(strings.ToUpper(resp), "LGTM") && len(resp) < 20 {
		fmt.Fprintf(cmd.OutOrStdout(), "✅ %s: LGTM\n", filepath.Base(path))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Feedback for %s:\n%s\n", filepath.Base(path), resp)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------")
}
