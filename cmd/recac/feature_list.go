package main

import (
	"fmt"
	"os"
	"recac/internal/pkg/git"
	"strings"
	"time"

	"github.com/spf13/cobra"
)


var featureListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all features (branches)",
	Long:  `Lists all branches with the "feature/" prefix, along with their last commit details.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := listFeatures(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}
	},
}

func listFeatures() error {
	branches, err := git.ListBranches("feature/")
	if err != nil {
		return fmt.Errorf("failed to list feature branches: %w", err)
	}

	if len(branches) == 0 {
		fmt.Println("No features found.")
		return nil
	}

	fmt.Printf("%-30s %-20s %-25s %s\n", "FEATURE", "AUTHOR", "LAST COMMIT", "MESSAGE")
	fmt.Println(strings.Repeat("-", 100))

	for _, branch := range branches {
		commit, err := git.GetLastCommit(branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not get last commit for branch %s: %v\n", branch, err)
			continue
		}

		// Clean up branch name for display
		featureName := strings.TrimPrefix(branch, "refs/heads/")
		featureName = strings.TrimPrefix(featureName, "feature/")

		// Format commit time
		commitTime := commit.Author.When.Format(time.RFC822)

		// Truncate commit message
		shortMessage := strings.Split(commit.Message, "\n")[0]
		if len(shortMessage) > 50 {
			shortMessage = shortMessage[:47] + "..."
		}

		fmt.Printf("%-30s %-20s %-25s %s\n",
			featureName,
			commit.Author.Name,
			commitTime,
			shortMessage,
		)
	}

	return nil
}
