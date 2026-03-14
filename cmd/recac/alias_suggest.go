package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	aliasSuggestHistoryFile string
	aliasSuggestShell       string
	aliasSuggestMinFreq     int
	aliasSuggestJSON        bool

	// Pre-compile regex for Zsh timestamp stripping
	// Zsh extended history: : 1678900000:0;command
	zshRe = regexp.MustCompile(`^: \d+:\d+;(.*)$`)
)

var aliasSuggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest aliases based on shell history",
	Long: `Analyzes your shell history (.bash_history or .zsh_history) to find frequently used long 'recac' commands.
Uses AI to suggest short, memorable aliases for them.`,
	RunE: runAliasSuggest,
}

func init() {
	aliasCmd.AddCommand(aliasSuggestCmd)
	aliasSuggestCmd.Flags().StringVar(&aliasSuggestHistoryFile, "history-file", "", "Path to shell history file (default: auto-detect)")
	aliasSuggestCmd.Flags().StringVar(&aliasSuggestShell, "shell", "auto", "Shell type (bash, zsh, auto)")
	aliasSuggestCmd.Flags().IntVar(&aliasSuggestMinFreq, "min-freq", 3, "Minimum frequency to consider")
	aliasSuggestCmd.Flags().BoolVar(&aliasSuggestJSON, "json", false, "Output suggestions as JSON")
}

type AliasSuggestion struct {
	Command   string `json:"command"`
	Frequency int    `json:"frequency"`
	Alias     string `json:"alias"`
	Reason    string `json:"reason,omitempty"`
}

func runAliasSuggest(cmd *cobra.Command, args []string) error {
	// 1. Detect Shell / History File
	shell := aliasSuggestShell
	histFile := aliasSuggestHistoryFile

	if shell == "auto" {
		shellEnv := os.Getenv("SHELL")
		if strings.Contains(shellEnv, "zsh") {
			shell = "zsh"
		} else {
			shell = "bash" // Default to bash
		}
	}

	if histFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home dir: %w", err)
		}
		if shell == "zsh" {
			histFile = filepath.Join(home, ".zsh_history")
		} else {
			histFile = filepath.Join(home, ".bash_history")
		}
	}

	// 2. Parse History
	commands, err := parseHistory(shell, histFile)
	if err != nil {
		// Fallback to warning if file not found, maybe user provided wrong path
		if os.IsNotExist(err) {
			return fmt.Errorf("history file not found: %s", histFile)
		}
		return fmt.Errorf("failed to parse history: %w", err)
	}

	// 3. Analyze
	frequentCmds := analyzeHistory(commands, aliasSuggestMinFreq)
	if len(frequentCmds) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No frequent long 'recac' commands found.")
		return nil
	}

	// 4. AI Suggestions
	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-alias-suggest")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	suggestions, err := getAISuggestions(ctx, ag, frequentCmds)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 5. Output
	if aliasSuggestJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(suggestions)
	}

	printSuggestions(cmd, suggestions)

	// 6. Interactive Apply
	// Only if TTY
	stat, _ := os.Stdout.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		applySelectedSuggestions(cmd.InOrStdin(), cmd.OutOrStdout(), suggestions)
	}

	return nil
}

// parseHistory reads the history file and returns a list of "recac ..." commands.
func parseHistory(shell, path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var commands []string
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		cmd := line
		if shell == "zsh" {
			matches := zshRe.FindStringSubmatch(line)
			if len(matches) > 1 {
				cmd = matches[1]
			} else {
				// Non-extended zsh history or multiline?
				// Just treat as raw line if regex fails
			}
		}

		// Check if it's a recac command
		// Could be "recac" or "./recac" or "bin/recac"
		// We normalize simple cases.
		parts := strings.Fields(cmd)
		if len(parts) > 0 {
			bin := filepath.Base(parts[0])
			if bin == "recac" || bin == "recac.exe" {
				// Keep the arguments, strip the binary name to normalize
				// Actually, we want the full command relative to recac
				// e.g. "todo solve --file foo.go"
				if len(parts) > 1 {
					commands = append(commands, strings.Join(parts[1:], " "))
				}
			}
		}
	}

	return commands, scanner.Err()
}

type commandStat struct {
	Command string
	Count   int
}

// analyzeHistory filters and counts commands.
func analyzeHistory(commands []string, minFreq int) []commandStat {
	counts := make(map[string]int)

	for _, cmd := range commands {
		// Normalize: trim spaces
		cmd = strings.TrimSpace(cmd)
		// Filter short commands (arbitrary: length < 10)
		// e.g. "help", "status" are short.
		// "todo solve --file x" is long.
		if len(cmd) < 10 {
			continue
		}
		counts[cmd]++
	}

	var stats []commandStat
	for cmd, count := range counts {
		if count >= minFreq {
			stats = append(stats, commandStat{Command: cmd, Count: count})
		}
	}

	// Sort by count desc, then command asc
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Command < stats[j].Command
		}
		return stats[i].Count > stats[j].Count
	})

	// Limit to top 10
	if len(stats) > 10 {
		stats = stats[:10]
	}

	return stats
}

type Agent interface {
	Send(ctx context.Context, prompt string) (string, error)
}

func getAISuggestions(ctx context.Context, ag Agent, stats []commandStat) ([]AliasSuggestion, error) {
	// Construct prompt
	var sb strings.Builder
	sb.WriteString("I have a list of frequently used CLI commands. Please suggest short, memorable aliases for them.\n")
	sb.WriteString("The aliases should be for the 'recac' CLI tool.\n\n")
	sb.WriteString("Commands:\n")
	for _, s := range stats {
		sb.WriteString(fmt.Sprintf("- %s (used %d times)\n", s.Command, s.Count))
	}
	sb.WriteString("\nReturn a JSON list of objects with 'command', 'alias', and 'reason'.\n")
	sb.WriteString("Example: [{\"command\": \"todo solve --file foo.go\", \"alias\": \"fix-foo\", \"reason\": \"Frequent fix\"}]\n")
	sb.WriteString("Do NOT use markdown code blocks.")

	resp, err := ag.Send(ctx, sb.String())
	if err != nil {
		return nil, err
	}

	clean := utils.CleanJSONBlock(resp)
	var suggestions []AliasSuggestion
	if err := json.Unmarshal([]byte(clean), &suggestions); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Merge count info back
	// Map command -> count
	countMap := make(map[string]int)
	for _, s := range stats {
		countMap[s.Command] = s.Count
	}

	for i := range suggestions {
		suggestions[i].Frequency = countMap[suggestions[i].Command]
	}

	return suggestions, nil
}

func printSuggestions(cmd *cobra.Command, suggestions []AliasSuggestion) {
	fmt.Fprintln(cmd.OutOrStdout(), "\n💡 Alias Suggestions:")
	fmt.Fprintln(cmd.OutOrStdout(), "=====================")
	for i, s := range suggestions {
		fmt.Fprintf(cmd.OutOrStdout(), "%d. recac %s  ->  recac %s\n   (used %d times) - %s\n\n",
			i+1, s.Command, s.Alias, s.Frequency, s.Reason)
	}
}

func applySelectedSuggestions(in io.Reader, out io.Writer, suggestions []AliasSuggestion) {
	fmt.Fprint(out, "Enter the numbers of aliases to apply (comma separated, e.g. 1,3) or 'all': ")

	reader := bufio.NewReader(in)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return
	}

	var selected []AliasSuggestion

	if strings.ToLower(input) == "all" {
		selected = suggestions
	} else {
		parts := strings.Split(input, ",")
		for _, part := range parts {
			var idx int
			if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &idx); err == nil {
				if idx >= 1 && idx <= len(suggestions) {
					selected = append(selected, suggestions[idx-1])
				}
			}
		}
	}

	if len(selected) == 0 {
		fmt.Fprintln(out, "No aliases selected.")
		return
	}

	aliases := viper.GetStringMapString("aliases")
	if aliases == nil {
		aliases = make(map[string]string)
	}

	for _, s := range selected {
		aliases[s.Alias] = s.Command
		fmt.Fprintf(out, "Applied: %s='%s'\n", s.Alias, s.Command)
	}

	viper.Set("aliases", aliases)
	if err := viper.WriteConfig(); err != nil {
		// Try safe write
		if err := viper.SafeWriteConfig(); err != nil {
			// We might not have access to stderr here, so just print to out or ignore for TUI simplicity
			// Or we could pass stderr as well. For now, writing to out is acceptable for interactive prompts.
			fmt.Fprintf(out, "Failed to write config: %v\n", err)
			return
		}
	}
	fmt.Fprintln(out, "✅ Configuration saved.")
}
