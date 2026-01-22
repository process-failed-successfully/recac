package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	releaseDryRun bool
	releaseMajor  bool
	releaseMinor  bool
	releasePatch  bool
	releasePush   bool
)

var releaseCmd = &cobra.Command{
	Use:   "release [version]",
	Short: "Automate semantic versioning and release",
	Long: `Automates the release process:
1. Calculates the next version based on Conventional Commits (or manual override).
2. Updates version files (VERSION, package.json).
3. Updates CHANGELOG.md (using AI summary).
4. Commits, tags, and pushes.

Usage:
  recac release           # Auto-detect next version
  recac release --minor   # Force minor version bump
  recac release v1.2.3    # Set specific version
  recac release --dry-run # Preview changes
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRelease,
}

func init() {
	rootCmd.AddCommand(releaseCmd)
	releaseCmd.Flags().BoolVar(&releaseDryRun, "dry-run", false, "Preview changes without modifying files or git")
	releaseCmd.Flags().BoolVar(&releaseMajor, "major", false, "Force major version bump")
	releaseCmd.Flags().BoolVar(&releaseMinor, "minor", false, "Force minor version bump")
	releaseCmd.Flags().BoolVar(&releasePatch, "patch", false, "Force patch version bump")
	releaseCmd.Flags().BoolVar(&releasePush, "push", true, "Push commits and tags to remote")
}

func runRelease(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	gitClient := gitClientFactory()
	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// 1. Get current version (from tag or file)
	currentTag, err := getLatestTag(gitClient, cwd)
	if err != nil {
		// If no tags, assume v0.0.0
		currentTag = "v0.0.0"
		fmt.Fprintf(cmd.OutOrStdout(), "No existing tags found. Starting from %s\n", currentTag)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Current latest tag: %s\n", currentTag)
	}

	// 2. Determine Next Version
	var nextVersion string
	if len(args) > 0 {
		nextVersion = args[0]
		if !strings.HasPrefix(nextVersion, "v") {
			nextVersion = "v" + nextVersion
		}
	} else {
		// Calculate based on commits
		rangeArg := fmt.Sprintf("%s..HEAD", currentTag)
		if currentTag == "v0.0.0" {
			// Check if we can just log everything
			rangeArg = "HEAD"
		}

		commits, err := getCommitLogs(cwd, rangeArg)
		if err != nil {
			// Fallback: maybe the range is invalid because v0.0.0 doesn't exist as a tag ref?
			// If currentTag is v0.0.0, we probably meant "all history".
			// "HEAD" should work.
			// If it failed, let's try without range if it wasn't HEAD?
			if currentTag == "v0.0.0" && rangeArg != "HEAD" {
				commits, err = getCommitLogs(cwd, "HEAD")
			}
			if err != nil {
				return fmt.Errorf("failed to get git log: %w", err)
			}
		}

		bump, reason := calculateBump(commits)
		if releaseMajor {
			bump = "major"
			reason = "forced via flag"
		} else if releaseMinor {
			bump = "minor"
			reason = "forced via flag"
		} else if releasePatch {
			bump = "patch"
			reason = "forced via flag"
		}

		nextVersion, err = bumpVersion(currentTag, bump)
		if err != nil {
			return fmt.Errorf("failed to calculate next version: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Next version: %s (%s - %s)\n", nextVersion, bump, reason)
	}

	if releaseDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "Dry run enabled. Skipping file updates and git operations.")
		return nil
	}

	// 3. Generate Changelog
	fmt.Fprintln(cmd.OutOrStdout(), "Generating changelog...")
	changelogEntry, err := generateChangelogEntry(ctx, gitClient, cwd, currentTag, nextVersion)
	if err != nil {
		return fmt.Errorf("failed to generate changelog: %w", err)
	}

	// 4. Apply Changes
	if err := updateVersionFiles(cwd, strings.TrimPrefix(nextVersion, "v")); err != nil {
		return fmt.Errorf("failed to update version files: %w", err)
	}

	if err := prependChangelog(cwd, changelogEntry); err != nil {
		return fmt.Errorf("failed to update CHANGELOG.md: %w", err)
	}

	// 5. Commit and Tag
	fmt.Fprintln(cmd.OutOrStdout(), "Committing and tagging...")
	if err := gitClient.Commit(cwd, fmt.Sprintf("chore(release): %s", nextVersion)); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	if err := gitClient.Tag(cwd, nextVersion, fmt.Sprintf("Release %s", nextVersion)); err != nil {
		return fmt.Errorf("git tag failed: %w", err)
	}

	if releasePush {
		fmt.Fprintln(cmd.OutOrStdout(), "Pushing to remote...")
		currentBranch, _ := gitClient.CurrentBranch(cwd)
		if err := gitClient.Push(cwd, currentBranch); err != nil {
			return fmt.Errorf("git push failed: %w", err)
		}
		if err := gitClient.PushTags(cwd); err != nil {
			return fmt.Errorf("git push --tags failed: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🚀 Released %s successfully!\n", nextVersion)
	return nil
}

func getLatestTag(gitClient IGitClient, cwd string) (string, error) {
	// Use git describe to find the latest reachable tag
	// git describe --tags --abbrev=0
	cmd := execCommand("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getCommitLogs(cwd, commitRange string) ([]string, error) {
	sep := "--RECAC-COMMIT-SEP--"
	// --pretty=format:%B gets the raw body
	args := []string{"log", fmt.Sprintf("--pretty=format:%%B%s", sep)}
	if commitRange != "" {
		args = append(args, commitRange)
	}

	cmd := execCommand("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Split by separator
	content := string(out)
	parts := strings.Split(content, sep)
	var logs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			logs = append(logs, p)
		}
	}
	return logs, nil
}

func calculateBump(commits []string) (string, string) {
	bump := "patch" // default
	reason := "default"

	for _, msg := range commits {
		// Look for BREAKING CHANGE: or BREAKING CHANGE in footer/body
		// Conventional commits: "BREAKING CHANGE: ..." or "feat!: ..."

		// Case insensitive check for breaking change text
		lowerMsg := strings.ToLower(msg)
		if strings.Contains(lowerMsg, "breaking change") {
			return "major", "breaking change detected"
		}

		// Check for !: in the first line (subject)
		lines := strings.Split(msg, "\n")
		if len(lines) > 0 {
			subject := lines[0]
			if strings.Contains(subject, "!: ") {
				return "major", "breaking change detected (!:)"
			}
			if strings.HasPrefix(subject, "feat") {
				bump = "minor"
				reason = "new feature detected"
			}
		}
	}
	return bump, reason
}

func bumpVersion(version, bump string) (string, error) {
	// Parse v1.2.3 or 1.2.3
	re := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	matches := re.FindStringSubmatch(version)
	if matches == nil {
		return "", fmt.Errorf("invalid semantic version format: %s", version)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	switch bump {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func updateVersionFiles(cwd, version string) error {
	// 1. VERSION file
	versionFile := filepath.Join(cwd, "VERSION")
	if _, err := os.Stat(versionFile); err == nil {
		if err := os.WriteFile(versionFile, []byte(version), 0644); err != nil {
			return err
		}
		fmt.Println("Updated VERSION")
	}

	// 2. package.json
	packageJson := filepath.Join(cwd, "package.json")
	if _, err := os.Stat(packageJson); err == nil {
		content, err := os.ReadFile(packageJson)
		if err == nil {
			// Simple string replacement for "version": "..."
			// This is risky but effective for simple cases.
			// Better to use json unmarshal/marshal to be safe.
			var data map[string]interface{}
			if err := json.Unmarshal(content, &data); err == nil {
				data["version"] = version
				newContent, err := json.MarshalIndent(data, "", "  ")
				if err == nil {
					os.WriteFile(packageJson, newContent, 0644)
					fmt.Println("Updated package.json")
				}
			}
		}
	}

	return nil
}

func generateChangelogEntry(ctx context.Context, gitClient IGitClient, cwd, from, to string) (string, error) {
	// Use existing changelog logic via Agent if possible, or build custom prompt
	logs, err := gitClient.Log(cwd, fmt.Sprintf("%s..HEAD", from), "--pretty=format:%h %an: %s", "--no-merges")
	if err != nil {
		if from == "v0.0.0" {
			logs, _ = gitClient.Log(cwd, "--pretty=format:%h %an: %s", "--no-merges")
		} else {
			return "", err
		}
	}

	if len(logs) == 0 {
		return fmt.Sprintf("## %s\n\nNo changes detected.\n", to), nil
	}

	// Use Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-release-changelog")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`Generate a Changelog entry for version %s.
Previous version was %s.

Commits:
%s

Format as Markdown:
## [Version] - Date
### Type (Features, Bug Fixes, etc)
- Description

Only output the markdown for this version entry.`, to, from, strings.Join(logs, "\n"))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return "", err
	}

	// Strip markdown blocks if present
	lines := strings.Split(resp, "\n")
	var cleaned []string
	for _, l := range lines {
		if strings.TrimSpace(l) == "```markdown" || strings.TrimSpace(l) == "```" {
			continue
		}
		cleaned = append(cleaned, l)
	}
	return strings.Join(cleaned, "\n"), nil
}

func prependChangelog(cwd, entry string) error {
	path := filepath.Join(cwd, "CHANGELOG.md")
	var existing string
	if b, err := os.ReadFile(path); err == nil {
		existing = string(b)
	}

	newContent := entry + "\n\n" + existing
	return os.WriteFile(path, []byte(newContent), 0644)
}
