package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	linksFix      bool
	linksExternal bool
	// Pre-compile regex for link finding and replacement
	linkRegex      = regexp.MustCompile(`\[.*?\]\((.*?)\)`)
	linkReplaceRegex = regexp.MustCompile(`(\[.*?\]\()(.*?)(\))`)

	// httpHeadFunc allows mocking HTTP requests for testing
	httpHeadFunc = func(url string) (*http.Response, error) {
		client := http.Client{
			Timeout: 5 * time.Second,
		}
		return client.Head(url)
	}
)

var linksCmd = &cobra.Command{
	Use:   "links [path]",
	Short: "Check and fix broken links in Markdown files",
	Long: `Scans Markdown files for broken links.
Can check local file references and optionally external URLs.
With --fix, it attempts to automatically resolve broken local links by searching for the file in the project.`,
	RunE: runLinks,
}

func init() {
	rootCmd.AddCommand(linksCmd)
	linksCmd.Flags().BoolVar(&linksFix, "fix", false, "Attempt to fix broken local links automatically")
	linksCmd.Flags().BoolVar(&linksExternal, "external", false, "Check external HTTP/HTTPS links (slow)")
}

type BrokenLink struct {
	File        string
	Link        string
	Target      string
	IsExternal  bool
	Replacement string
}

func runLinks(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Scanning %s for broken links...\n", root)

	brokenLinks, err := scanLinks(root, linksExternal)
	if err != nil {
		return err
	}

	if len(brokenLinks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No broken links found!")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d broken links:\n", len(brokenLinks))

	for _, bl := range brokenLinks {
		icon := "📁"
		if bl.IsExternal {
			icon = "🌐"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s: %s -> %s\n", icon, bl.File, bl.Link, bl.Target)
	}

	if linksFix {
		fixedCount := 0
		for _, bl := range brokenLinks {
			if bl.IsExternal {
				continue
			}

			// Try to find the file
			targetName := filepath.Base(bl.Target)
			candidates, err := findFilesByName(root, targetName)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "    Warning: failed to search for %s: %v\n", targetName, err)
				continue
			}

			if len(candidates) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "    ❌ Could not find %s in project.\n", targetName)
				continue
			}

			// Pick the best candidate (simplest: first one, or shortest path)
			// Ideally we might prefer one that is closer in the tree?
			// Let's take the first one for now.
			newTarget := candidates[0]

			// Calculate relative path from bl.File (dir) to newTarget
			blDir := filepath.Dir(bl.File)
			relPath, err := filepath.Rel(blDir, newTarget)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "    Warning: failed to calc rel path: %v\n", err)
				continue
			}

			// Force forward slashes for markdown
			relPath = filepath.ToSlash(relPath)

			// Apply fix
			if err := updateLinkInFile(bl.File, bl.Target, relPath); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "    ❌ Failed to update %s: %v\n", bl.File, err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "    ✅ Fixed: %s -> %s\n", bl.Target, relPath)
				fixedCount++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Fixed %d links.\n", fixedCount)

		remaining := len(brokenLinks) - fixedCount
		if remaining > 0 {
			return fmt.Errorf("fixed %d links, but %d remain broken", fixedCount, remaining)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nRun with --fix to attempt automatic repair of local links.")
		return fmt.Errorf("found %d broken links", len(brokenLinks))
	}

	return nil
}

func scanLinks(root string, checkExternal bool) ([]BrokenLink, error) {
	var broken []BrokenLink

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == ".recac" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		matches := linkRegex.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			fullLink := m[0]
			target := m[1]
			// Strip title if present: [text](url "title")
			if idx := strings.Index(target, " \""); idx != -1 {
				target = target[:idx]
			}
			target = strings.TrimSpace(target)

			// Handle anchors
			targetPath := target
			if idx := strings.Index(target, "#"); idx != -1 {
				targetPath = target[:idx]
			}

			if targetPath == "" {
				// Internal link to section in same file (#section) - valid if file exists (it does)
				continue
			}

			isExternal := strings.HasPrefix(targetPath, "http://") || strings.HasPrefix(targetPath, "https://")

			if isExternal {
				if checkExternal {
					if !checkURL(targetPath) {
						broken = append(broken, BrokenLink{
							File:       path,
							Link:       fullLink,
							Target:     target,
							IsExternal: true,
						})
					}
				}
			} else {
				// Local file check
				// Target is relative to the markdown file location
				dir := filepath.Dir(path)
				absTarget := filepath.Join(dir, targetPath)

				if _, err := os.Stat(absTarget); err != nil {
					broken = append(broken, BrokenLink{
						File:       path,
						Link:       fullLink,
						Target:     targetPath, // Store pure path for fixing
						IsExternal: false,
					})
				}
			}
		}

		return nil
	})

	return broken, err
}

func checkURL(url string) bool {
	resp, err := httpHeadFunc(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func findFilesByName(root, name string) ([]string, error) {
	var candidates []string
	ignored := DefaultIgnoreMap()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignored[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == name {
			candidates = append(candidates, path)
		}
		return nil
	})
	return candidates, err
}

func updateLinkInFile(filePath, oldTarget, newTarget string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Simple string replacement?
	// Risky if same link appears multiple times but only one is broken (unlikely for exact same string).
	// Also we need to be careful about what we replace.
	// We scan again and replace only the broken matches?
	// But `updateLinkInFile` is called for a specific BrokenLink which we identified.
	// The `oldTarget` passed here is the path part.
	// We should replace `(oldTarget)` with `(newTarget)`.
	// What if `oldTarget` is used in text?
	// We should use regex to replace only inside `](...)`.

	// Construct regex: `\]\(` + regexp.QuoteMeta(oldTarget) + `\)`
	// But wait, `oldTarget` might have regex chars. QuoteMeta handles that.
	// Also need to handle potential title? `oldTarget` we passed didn't include title.
	// If the original link had a title, `oldTarget` matches the path part.
	// `(... oldTarget ...)` ?
	// Let's assume for now we just replace `(oldTarget)` or `(oldTarget ` (start of url).

	// Actually, `strings.ReplaceAll` of `(oldTarget)` to `(newTarget)` is decent 99% of time.
	// But let's be safer.
	// Replace `]({oldTarget})` -> `]({newTarget})`
	// Replace `]({oldTarget} ` -> `]({newTarget} ` (if title)
	// Replace `]({oldTarget}#` -> `]({newTarget}#` (if anchor)

	newContent := string(content)

	// Actually, easier approach: Use regex to find all links, check if target matches oldTarget, and replace.
	// Group 1: [text](
	// Group 2: url + optional title
	// Group 3: )

	newContent = linkReplaceRegex.ReplaceAllStringFunc(newContent, func(match string) string {
		sub := linkReplaceRegex.FindStringSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		prefix := sub[1]
		inner := sub[2]
		suffix := sub[3]

		// Inner might contain title: "url title"
		urlPart := inner
		titlePart := ""
		if idx := strings.Index(inner, " \""); idx != -1 {
			urlPart = inner[:idx]
			titlePart = inner[idx:]
		}

		// urlPart might contain anchor: "url#anchor"
		pathPart := urlPart
		anchorPart := ""
		if idx := strings.Index(urlPart, "#"); idx != -1 {
			pathPart = urlPart[:idx]
			anchorPart = urlPart[idx:]
		}

		if pathPart == oldTarget {
			// Found it! Replace pathPart with newTarget
			return prefix + newTarget + anchorPart + titlePart + suffix
		}

		return match
	})

	return writeFileFunc(filePath, []byte(newContent), 0644)
}
