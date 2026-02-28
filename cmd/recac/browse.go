package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var browseExecCommand = exec.Command

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Open the repository's origin URL in the default web browser",
	Long:  `Open the repository's origin URL in the default web browser. Useful for quickly navigating to GitHub, GitLab, etc.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		gitClient := gitClientFactory()
		if !gitClient.RepoExists(cwd) {
			return fmt.Errorf("not a git repository")
		}

		remoteURL, err := gitClient.Run(cwd, "remote", "get-url", "origin")
		if err != nil || strings.TrimSpace(remoteURL) == "" {
			return fmt.Errorf("could not find remote 'origin' URL")
		}

		url := formatGitURLToHTTPS(strings.TrimSpace(remoteURL))

		if err := openBrowserURL(url); err != nil {
			return fmt.Errorf("failed to open browser: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Opened %s in the browser.\n", url)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(browseCmd)
}

func formatGitURLToHTTPS(rawURL string) string {
	url := rawURL

	// Convert SSH format `git@github.com:user/repo.git` to `https://github.com/user/repo.git`
	if strings.HasPrefix(url, "git@") {
		url = strings.Replace(url, ":", "/", 1)
		url = strings.Replace(url, "git@", "https://", 1)
	}

	// Remove .git suffix
	if strings.HasSuffix(url, ".git") {
		url = strings.TrimSuffix(url, ".git")
	}

	return url
}

func openBrowserURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = browseExecCommand("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = browseExecCommand("open", url)
	default: // linux, freebsd, openbsd, netbsd
		cmd = browseExecCommand("xdg-open", url)
	}

	return cmd.Start()
}
