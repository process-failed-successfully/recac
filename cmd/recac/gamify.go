package main

import (
	"fmt"
	"recac/internal/gamify"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var gamifyCmd = &cobra.Command{
	Use:   "gamify [path]",
	Short: "Gamify your git history",
	Long: `Analyze the git repository and generate a leaderboard of contributors based on XP, Badges, and Achievements.

XP is awarded for:
- Commits
- Lines of code (capped)
- Bug fixes
- Documentation updates
- Test coverage improvements
`,
	RunE: runGamify,
}

func init() {
	rootCmd.AddCommand(gamifyCmd)
}

func runGamify(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// 1. Get Client
	// gitClientFactory returns IGitClient (local interface)
	// We need to adapt it or verify it matches gamify.GitClient
	client := gitClientFactory()

	if !client.RepoExists(path) {
		return fmt.Errorf("not a git repository: %s", path)
	}

	// 2. Analyze
	// Go interface satisfaction is implicit. If client implements Log, it works.
	lb, err := gamify.AnalyzeRepo(client, path)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// 3. Render Output
	fmt.Fprintln(cmd.OutOrStdout(), "\n🏆 GIT LEADERBOARD 🏆")
	fmt.Fprintln(cmd.OutOrStdout(), "======================")

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RANK\tPLAYER\tXP\tCOMMITS\tFIXES\tBADGES")
	fmt.Fprintln(w, "----\t------\t--\t-------\t-----\t------")

	for i, p := range lb.Players {
		badges := ""
		if len(p.Badges) > 0 {
			if len(p.Badges) > 3 {
				badges = fmt.Sprintf("%s +%d more", p.Badges[0], len(p.Badges)-1)
			} else {
				badges = fmt.Sprintf("%v", p.Badges)
			}
		}

		rank := i + 1
		medal := ""
		switch rank {
		case 1:
			medal = "🥇 "
		case 2:
			medal = "🥈 "
		case 3:
			medal = "🥉 "
		}

		fmt.Fprintf(w, "%d\t%s%s\t%d\t%d\t%d\t%s\n", rank, medal, p.Name, p.XP, p.Commits, p.BugFixes, badges)
	}
	w.Flush()
	fmt.Fprintln(cmd.OutOrStdout(), "")

	return nil
}
