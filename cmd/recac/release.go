package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	releaseDryRun bool
	releaseBump   string
	releasePush   bool
)

func NewReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Automate the release process",
		Long: `Automates the release process by:
1. Determining the next version based on commits (Conventional Commits).
2. Generating a changelog using AI.
3. Updating the version file.
4. Updating CHANGELOG.md.
5. Creating a git commit and tag.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			// 1. Git Check
			gitClient := gitClientFactory()
			if !gitClient.RepoExists(cwd) {
				return fmt.Errorf("not a git repository")
			}

			// 2. Current Version
			versionFile := filepath.Join(cwd, "cmd/recac/version.go")
			if _, err := os.Stat(versionFile); os.IsNotExist(err) {
				// Fallback or error? Let's error for now as it's specific to this repo structure
				return fmt.Errorf("version file not found at %s", versionFile)
			}

			currentVersion, err := getCurrentVersion(versionFile)
			if err != nil {
				return fmt.Errorf("failed to get current version: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", currentVersion)

			// 3. Last Tag & Commits
			lastTag, err := getLastTag(cwd)
			if err != nil {
				return fmt.Errorf("failed to get last tag: %w", err)
			}
			if lastTag != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Last tag: %s\n", lastTag)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "No tags found. Analyzing all commits.")
			}

			commits, err := getCommitsSince(gitClient, cwd, lastTag)
			if err != nil {
				return fmt.Errorf("failed to get commits: %w", err)
			}
			if len(commits) == 0 {
				return fmt.Errorf("no commits found since last release")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d commits.\n", len(commits))

			// 4. Next Version
			nextVersion, err := getNextVersion(currentVersion, commits, releaseBump)
			if err != nil {
				return fmt.Errorf("failed to calculate next version: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Next version: %s\n", nextVersion)

			if releaseDryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "[Dry Run] Skipping changes...")
				return nil
			}

			// 5. Generate Changelog
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			// If no provider set, maybe skip AI or error?
			// We'll try to get agent.
			var changelogBody string
			if provider != "" {
				ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-release")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create agent: %v. Using simple log dump.\n", err)
					changelogBody = strings.Join(commits, "\n- ")
					changelogBody = "- " + changelogBody
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Generating changelog with AI...")
					prompt := fmt.Sprintf(`You are a release assistant.
Generate a concise Changelog for version v%s based on:
%s

Group by type (Features, Fixes, etc).
Output Markdown ONLY.`, nextVersion, strings.Join(commits, "\n"))

					changelogBody, err = ag.Send(ctx, prompt)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: AI generation failed: %v. Using simple log dump.\n", err)
						changelogBody = "- " + strings.Join(commits, "\n- ")
					}
					changelogBody = utils.CleanCodeBlock(changelogBody)
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "No AI provider configured. Using simple commit log.")
				changelogBody = "- " + strings.Join(commits, "\n- ")
			}

			// 6. File Updates
			if err := updateVersionFile(versionFile, nextVersion); err != nil {
				return fmt.Errorf("failed to update version file: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Updated version.go")

			changelogFile := filepath.Join(cwd, "CHANGELOG.md")
			if err := updateChangelog(changelogFile, nextVersion, changelogBody); err != nil {
				return fmt.Errorf("failed to update CHANGELOG.md: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Updated CHANGELOG.md")

			// 7. Git Operations
			tag := fmt.Sprintf("v%s", nextVersion)
			msg := fmt.Sprintf("chore(release): %s", tag)

			if err := gitClient.Commit(cwd, msg); err != nil {
				return fmt.Errorf("failed to commit: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Committed changes.")

			// Tag
			tagCmd := exec.Command("git", "tag", tag)
			tagCmd.Dir = cwd
			if err := tagCmd.Run(); err != nil {
				return fmt.Errorf("failed to tag: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created tag %s\n", tag)

			if releasePush {
				fmt.Fprintln(cmd.OutOrStdout(), "Pushing to remote...")
				pushCmd := exec.Command("git", "push", "origin", "HEAD")
				pushCmd.Dir = cwd
				if err := pushCmd.Run(); err != nil {
					return fmt.Errorf("failed to push commits: %w", err)
				}
				// Push tags
				pushTagCmd := exec.Command("git", "push", "origin", tag)
				pushTagCmd.Dir = cwd
				if err := pushTagCmd.Run(); err != nil {
					return fmt.Errorf("failed to push tag: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Pushed to remote.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&releaseDryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().StringVar(&releaseBump, "bump", "", "Manual override for bump type (major, minor, patch)")
	cmd.Flags().BoolVar(&releasePush, "push", false, "Push changes and tags to remote")

	return cmd
}

var releaseCmd = NewReleaseCmd()

func init() {
	rootCmd.AddCommand(releaseCmd)
}

// Helpers

func getCurrentVersion(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read version file: %w", err)
	}

	// Look for version = "vX.Y.Z"
	re := regexp.MustCompile(`version\s*=\s*"v?([^"]+)"`)
	matches := re.FindSubmatch(content)
	if len(matches) < 2 {
		return "", fmt.Errorf("version string not found in %s", path)
	}
	return string(matches[1]), nil
}

func getNextVersion(current string, commits []string, bumpOverride string) (string, error) {
	// Parse current version
	parts := strings.Split(current, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver format: %s", current)
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])

	if bumpOverride != "" {
		switch bumpOverride {
		case "major":
			return fmt.Sprintf("%d.0.0", major+1), nil
		case "minor":
			return fmt.Sprintf("%d.%d.0", major, minor+1), nil
		case "patch":
			return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
		default:
			return "", fmt.Errorf("invalid bump type: %s", bumpOverride)
		}
	}

	bumpMajor := false
	bumpMinor := false

	for _, msg := range commits {
		if strings.Contains(msg, "BREAKING CHANGE") || strings.Contains(msg, "BREAKING-CHANGE") || strings.Contains(msg, "!:") {
			bumpMajor = true
			break // Major trumps all
		}
		if strings.HasPrefix(msg, "feat") {
			bumpMinor = true
		}
	}

	if bumpMajor {
		major++
		minor = 0
		patch = 0
	} else if bumpMinor {
		minor++
		patch = 0
	} else {
		patch++
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

func getLastTag(dir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// If no tags found, git describe fails. Return empty string.
		return "", nil
	}
	return strings.TrimSpace(out.String()), nil
}

func getCommitsSince(gitClient IGitClient, dir, tag string) ([]string, error) {
	var args []string
	if tag != "" {
		args = []string{tag + "..HEAD"}
	} else {
		args = []string{} // All commits
	}
	// Use %s to get subject only for analysis
	args = append(args, "--pretty=format:%s", "--no-merges")

	logs, err := gitClient.Log(dir, args...)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func updateVersionFile(path, newVersion string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`version\s*=\s*"v?([^"]+)"`)
	newContent := re.ReplaceAll(content, []byte(fmt.Sprintf(`version = "v%s"`, newVersion)))

	// Also update date
	reDate := regexp.MustCompile(`date\s*=\s*"([^"]+)"`)
	newContent = reDate.ReplaceAll(newContent, []byte(fmt.Sprintf(`date = "%s"`, time.Now().Format("2006-01-02"))))

	return os.WriteFile(path, newContent, 0644)
}

func updateChangelog(path, version, content string) error {
	// Read existing
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	header := fmt.Sprintf("# v%s (%s)\n\n", version, time.Now().Format("2006-01-02"))
	newEntry := header + content + "\n\n"

	final := []byte(newEntry)
	if len(existing) > 0 {
		final = append(final, existing...)
	}
	return os.WriteFile(path, final, 0644)
}
