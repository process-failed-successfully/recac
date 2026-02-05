package main

import (
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	announcePlatform string
	announceTone     string
	announceOutput   string
)

var announceCmd = &cobra.Command{
	Use:   "announce",
	Short: "Generate a social media announcement for the latest changes",
	Long: `Generates a social media post (Twitter, LinkedIn, Slack, etc.) based on the project's recent changes.
It reads from CHANGELOG.md if available, or falls back to git logs since the last tag.`,
	RunE: runAnnounce,
}

func init() {
	rootCmd.AddCommand(announceCmd)
	announceCmd.Flags().StringVarP(&announcePlatform, "platform", "p", "twitter", "Target platform (twitter, linkedin, slack, blog)")
	announceCmd.Flags().StringVarP(&announceTone, "tone", "t", "excited", "Tone of the announcement (excited, professional, technical, funny)")
	announceCmd.Flags().StringVarP(&announceOutput, "output", "o", "", "Write output to a file instead of stdout")
}

func runAnnounce(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Get Content (Changelog or Git Log)
	var content string
	changelogPath := "CHANGELOG.md"
	if _, err := os.Stat(changelogPath); err == nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "Found CHANGELOG.md, reading latest entry...")
		data, err := os.ReadFile(changelogPath)
		if err != nil {
			return fmt.Errorf("failed to read CHANGELOG.md: %w", err)
		}
		// Heuristic: Get top 20 lines or try to parse latest version?
		// For simplicity, let's take the first 2000 chars which likely covers the latest release.
		sData := string(data)
		runes := []rune(sData)
		if len(runes) > 2000 {
			content = string(runes[:2000]) + "\n... (truncated)"
		} else {
			content = sData
		}
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "No CHANGELOG.md found, falling back to git logs...")
		gitClient := gitClientFactory()
		if !gitClient.RepoExists(cwd) {
			return fmt.Errorf("not a git repository")
		}

		// Try to find last tag
		lastTag, err := gitClient.LatestTag(cwd)
		logArgs := []string{"--pretty=format:%s", "--no-merges", "-n", "20"}
		if err == nil && lastTag != "" {
			logArgs = []string{"--pretty=format:%s", "--no-merges", fmt.Sprintf("%s..HEAD", lastTag)}
		}

		logs, err := gitClient.Log(cwd, logArgs...)
		if err != nil {
			return fmt.Errorf("failed to get git logs: %w", err)
		}
		if len(logs) == 0 {
			return fmt.Errorf("no changes found to announce")
		}
		content = strings.Join(logs, "\n")
	}

	// 2. Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-announce")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are a Developer Marketing Expert.
Task: Write a %s announcement for the latest update of this project.
Tone: %s

Input Data (Changelog/Logs):
'''
%s
'''

Guidelines:
- Highlight the most important features.
- Keep it concise appropriate for %s.
- Use emojis if the tone is 'excited' or 'funny'.
- Include relevant hashtags (e.g. #opensource #dev) if platform is Twitter/LinkedIn.
- Do NOT output markdown code blocks unless requested. Just the raw text for the post.
`, announcePlatform, announceTone, content, announcePlatform)

	fmt.Fprintln(cmd.ErrOrStderr(), "Generating announcement...")

	// 4. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate announcement: %w", err)
	}

	resp = utils.CleanCodeBlock(resp)

	// 5. Output
	if announceOutput != "" {
		if err := os.WriteFile(announceOutput, []byte(resp), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Announcement written to %s\n", announceOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), resp)
	}

	return nil
}
