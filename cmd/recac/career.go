package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	careerAuthor string
	careerSince  string
	careerOutput string
)

var careerCmd = &cobra.Command{
	Use:   "career",
	Short: "Generate career achievements and resume points from git history",
	Long: `Analyzes your git contributions to generate a "Brag Document" or resume bullet points.
It scans your commits, identifies impact (technologies used, lines changed), and uses AI
to summarize your achievements in STAR (Situation, Task, Action, Result) format.`,
	RunE: runCareer,
}

func init() {
	rootCmd.AddCommand(careerCmd)
	careerCmd.Flags().StringVar(&careerAuthor, "author", "", "Git author to analyze (default: current user)")
	careerCmd.Flags().StringVar(&careerSince, "since", "30d", "Time window to analyze (e.g. 30d, 1y)")
	careerCmd.Flags().StringVarP(&careerOutput, "output", "o", "", "Output file path")
}

func runCareer(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	git := gitClientFactory()

	// 1. Determine Author
	author := careerAuthor
	if author == "" {
		// Try to get current user.name or user.email
		name, err := git.Run(cwd, "config", "user.name")
		if err == nil {
			author = strings.TrimSpace(name)
		}
		if author == "" {
			email, err := git.Run(cwd, "config", "user.email")
			if err == nil {
				author = strings.TrimSpace(email)
			}
		}
		if author == "" {
			return fmt.Errorf("could not determine git author. Please use --author")
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Analyzing contributions for author: %s\n", author)

	// 2. Fetch Commits (Messages)
	// git log --author=<author> --since=<since> --pretty=format:"%h|%ad|%s"
	sinceArg := careerSince
	// Ensure since format is compatible with git log. Git supports "30 days ago", "1 year ago", etc.
	// But our flag says "30d". Let's assume git handles it or we parse it.
	// Git log --since="30d" doesn't work directly, it needs "30 days ago".
	// Let's try to parse it if it looks like a duration.
	if d, err := time.ParseDuration(careerSince); err == nil {
		sinceArg = time.Now().Add(-d).Format(time.RFC3339)
	} else {
		// If it's just "30d" (which ParseDuration might fail on if no unit?), Go's ParseDuration handles "h", "m", "s", "ns", "us", "ms".
		// It does NOT handle "d", "w", "y".
		// We can use a helper or just pass it to git if it supports it? Git supports "2 weeks ago".
		// If user passes "30d", we should probably convert to "30 days ago".
		if strings.HasSuffix(careerSince, "d") {
			days := strings.TrimSuffix(careerSince, "d")
			sinceArg = fmt.Sprintf("%s days ago", days)
		} else if strings.HasSuffix(careerSince, "w") {
			weeks := strings.TrimSuffix(careerSince, "w")
			sinceArg = fmt.Sprintf("%s weeks ago", weeks)
		} else if strings.HasSuffix(careerSince, "y") {
			years := strings.TrimSuffix(careerSince, "y")
			sinceArg = fmt.Sprintf("%s years ago", years)
		}
	}

	commits, err := git.Log(cwd, "--author="+author, "--since="+sinceArg, "--pretty=format:%h|%ad|%s")
	if err != nil {
		return fmt.Errorf("failed to fetch commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No commits found in the specified period.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d commits since %s.\n", len(commits), sinceArg)

	// 3. Fetch Files Changed (for Skills)
	// git log --author=<author> --since=<since> --name-only --format=""
	files, err := git.Log(cwd, "--author="+author, "--since="+sinceArg, "--name-only", "--format=")
	if err != nil {
		// Non-fatal, just less info
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to fetch file stats: %v\n", err)
	}

	// Analyze Files for Skills
	skills := make(map[string]int)
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		ext := filepath.Ext(f)
		if ext != "" {
			skills[ext]++
		}
		// Check for specific filenames like Dockerfile
		if strings.Contains(strings.ToLower(f), "dockerfile") {
			skills["Docker"]++
		}
		if strings.Contains(strings.ToLower(f), "makefile") {
			skills["Make"]++
		}
		if strings.Contains(strings.ToLower(f), ".github/workflows") {
			skills["CI/CD"]++
		}
	}

	// 4. Construct Prompt
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Generate a 'Brag Document' or Resume Achievement list for the developer '%s'.\n", author))
	sb.WriteString("Focus on impact, technical skills, and problem-solving.\n")
	sb.WriteString("Use the STAR format (Situation, Task, Action, Result) where possible, but keep it concise (bullet points).\n\n")

	sb.WriteString("### Tech Stack Inferred:\n")
	var techList []string
	for k, v := range skills {
		if v > 0 {
			techList = append(techList, k)
		}
	}
	sort.Strings(techList)
	sb.WriteString(strings.Join(techList, ", ") + "\n\n")

	sb.WriteString("### Commit History (Recent first):\n")
	// Limit commits to avoid context overflow (e.g. 50?)
	maxCommits := 50
	if len(commits) > maxCommits {
		sb.WriteString(fmt.Sprintf("(Showing last %d of %d commits)\n", maxCommits, len(commits)))
		commits = commits[:maxCommits]
	}
	for _, c := range commits {
		sb.WriteString(c + "\n")
	}

	sb.WriteString("\nInstructions:\n")
	sb.WriteString("- Group related commits into single achievements.\n")
	sb.WriteString("- Highlight key technologies used.\n")
	sb.WriteString("- Mention refactoring, performance improvements, or new features.\n")
	sb.WriteString("- Output in Markdown.\n")

	// 5. Call AI
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-career")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Analyzing career impact...")

	resp, err := ag.Send(ctx, sb.String())
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 6. Output
	if careerOutput != "" {
		if err := os.WriteFile(careerOutput, []byte(resp), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Career summary saved to %s\n", careerOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\n"+resp)
	}

	return nil
}
