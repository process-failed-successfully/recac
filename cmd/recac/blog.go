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
	blogSince  string
	blogOutput string
	blogStyle  string
	blogFiles  []string
)

var blogCmd = &cobra.Command{
	Use:   "blog",
	Short: "Generate a technical blog post from git history",
	Long: `Generates a draft for a technical blog post summarizing recent changes.
It analyzes the git history and project context to write an engaging post in the specified style.

Styles:
- announcement: Focus on new features and releases.
- tutorial: Explain how to use the new changes.
- deep-dive: Technical explanation of the implementation.`,
	RunE: runBlog,
}

func init() {
	// Check if rootCmd is nil to avoid panics in tests where it might not be initialized
	if rootCmd != nil {
		rootCmd.AddCommand(blogCmd)
	}
	blogCmd.Flags().StringVar(&blogSince, "since", "1 week ago", "Time window to analyze")
	blogCmd.Flags().StringVarP(&blogOutput, "output", "o", "blog_post.md", "Output file path")
	blogCmd.Flags().StringVarP(&blogStyle, "style", "s", "announcement", "Style of the blog post (announcement, tutorial, deep-dive)")
	blogCmd.Flags().StringSliceVarP(&blogFiles, "files", "f", nil, "Specific files to include in context")
}

func runBlog(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather Git History
	client := gitClientFactory()
	if !client.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	logArgs := []string{
		"--pretty=format:%h|%an|%s",
		"--since=" + blogSince,
		"--no-merges",
	}

	logs, err := client.Log(cwd, logArgs...)
	if err != nil {
		return fmt.Errorf("failed to fetch git logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Printf("No commits found since %s.\n", blogSince)
		return nil
	}

	// 2. Prepare Context
	var contextBuilder strings.Builder
	contextBuilder.WriteString(fmt.Sprintf("Commits since %s:\n", blogSince))
	for _, line := range logs {
		contextBuilder.WriteString("- " + line + "\n")
	}
	contextBuilder.WriteString("\n")

	// Add README context
	if content, err := os.ReadFile("README.md"); err == nil {
		contextBuilder.WriteString("Project Context (README):\n")
		// Truncate if too long
		if len(content) > 2000 {
			contextBuilder.WriteString(string(content[:2000]) + "\n...(truncated)\n")
		} else {
			contextBuilder.WriteString(string(content) + "\n")
		}
		contextBuilder.WriteString("\n")
	}

	// Add specific files context
	if len(blogFiles) > 0 {
		contextBuilder.WriteString("Relevant Files:\n")
		for _, f := range blogFiles {
			content, err := os.ReadFile(f)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read %s: %v\n", f, err)
				continue
			}
			contextBuilder.WriteString(fmt.Sprintf("--- %s ---\n", f))
			if len(content) > 3000 {
				contextBuilder.WriteString(string(content[:3000]) + "\n...(truncated)\n")
			} else {
				contextBuilder.WriteString(string(content) + "\n")
			}
			contextBuilder.WriteString("\n")
		}
	}

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are a Developer Advocate and Technical Writer.
Write a blog post based on the recent changes in this project.

Style: %s
Target Audience: Developers and technical users.

Instructions:
- Title should be catchy and relevant.
- Structure with clear headings.
- Highlight key features or fixes from the git log.
- Use code snippets (if implied by commit messages or file content) where appropriate to illustrate.
- Be engaging and professional.

Input Data:
%s

Return ONLY the Markdown content for the blog post.
`, blogStyle, contextBuilder.String())

	// 4. Call Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-blog")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✍️  Drafting blog post (%s style)...\n", blogStyle)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 5. Save Output
	cleaned := utils.CleanCodeBlock(resp)

	if err := os.WriteFile(blogOutput, []byte(cleaned), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Blog post saved to %s\n", blogOutput)
	return nil
}
