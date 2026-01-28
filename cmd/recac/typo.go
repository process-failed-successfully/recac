package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	typoFix   bool
	typoLimit int
)

var typoCmd = &cobra.Command{
	Use:   "typo [path]",
	Short: "Check for typos in comments and strings using AI",
	Long: `Scans the codebase for potential typos in comments and string literals.
Uses the configured AI agent to verify typos and suggest corrections.
Can interactively fix them.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTypo,
}

func init() {
	rootCmd.AddCommand(typoCmd)
	typoCmd.Flags().BoolVarP(&typoFix, "fix", "f", false, "Interactively fix typos")
	typoCmd.Flags().IntVarP(&typoLimit, "limit", "l", 50, "Maximum number of files to scan")
}

func runTypo(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Scanning %s for typos...\n", root)

	// 1. Scan files
	files, err := scanFilesForTypo(root, typoLimit)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No suitable files found to scan.")
		return nil
	}

	// 2. Extract Candidates
	candidates, fileMap := extractTypoCandidates(files)
	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No suspicious words found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d unique candidate words in %d files. Checking with AI...\n", len(candidates), len(files))

	// 3. AI Check
	typos, err := checkTyposWithAI(cmd.Context(), candidates)
	if err != nil {
		return err
	}

	if len(typos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No typos found!")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "❌ Found %d typos:\n", len(typos))

	// 4. Report / Fix
	for word, suggestion := range typos {
		// Find where this word occurs
		filesWithTypo := findFilesWithWord(fileMap, word)
		fmt.Fprintf(cmd.OutOrStdout(), "  • '%s' -> '%s' (found in %d files)\n", word, suggestion, len(filesWithTypo))

		if typoFix {
			for _, file := range filesWithTypo {
				msg := fmt.Sprintf("Fix in %s?", file)
				var confirm bool
				prompt := &survey.Confirm{
					Message: msg,
					Default: true,
				}
				if err := askOneFunc(prompt, &confirm); err != nil {
					return err
				}
				if confirm {
					if err := replaceInFile(file, word, suggestion); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "    Failed to fix %s: %v\n", file, err)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "    ✅ Fixed in %s\n", file)
					}
				}
			}
		}
	}

	if !typoFix {
		fmt.Fprintln(cmd.OutOrStdout(), "\nRun with --fix to interactively correct them.")
	}

	return nil
}

func scanFilesForTypo(root string, limit int) ([]string, error) {
	var files []string
	ignored := DefaultIgnoreMap()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if ignored[info.Name()] || strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Only check text files
		ext := filepath.Ext(path)
		if utils.IsBinaryExt(ext) {
			return nil
		}
		if strings.Contains(path, "lock") || strings.Contains(path, "sum") {
			return nil
		}

		files = append(files, path)
		if len(files) >= limit {
			// Stop walking if limit reached? No, walk is depth first.
			// Ideally we want to scan *important* files first?
			// Let's just hard stop for this MVP to avoid scanning huge repos entirely
			// return filepath.SkipAll // SkipAll is go 1.23+? No, SkipDir.
			// We can return a special error to stop.
			// But filepath.Walk doesn't support easy "Stop everything" except returning error.
			// Let's just collect all and then slice? No, that reads directories.
			// For now, let's just collect up to limit * 2 and then slice.
			if len(files) > limit {
				return fmt.Errorf("limit reached") // Hacky way to stop
			}
		}
		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return nil, err
	}
	if len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}

// extractTypoCandidates returns a list of unique words and a map of word -> files containing it
func extractTypoCandidates(files []string) ([]string, map[string][]string) {
	candidates := make(map[string]bool)
	fileMap := make(map[string][]string)

	// Regex for "words": only alpha, min 4 chars.
	re := regexp.MustCompile(`[a-zA-Z]{4,}`)

	// Allowlist (very basic)
	allowlist := map[string]bool{
		"func": true, "return": true, "package": true, "import": true, "string": true,
		"interface": true, "struct": true, "type": true, "const": true, "range": true,
		"error": true, "false": true, "true": true, "nil": true, "append": true,
		"len": true, "cap": true, "make": true, "new": true, "panic": true,
		"recover": true, "close": true, "copy": true, "print": true, "println": true,
		"todo": true, "http": true, "json": true, "yaml": true, "html": true,
		"context": true, "cobra": true, "viper": true, "recac": true, "github": true,
		"file": true, "path": true, "read": true, "write": true, "open": true,
		"stat": true, "exec": true, "cmd": true, "args": true,
		"flag": true, "flags": true, "usage": true, "short": true, "long": true,
		"run": true, "init": true, "main": true, "test": true, "bench": true,
		"example": true, "param": true, "value": true, "data": true, "result": true,
		"output": true, "input": true, "buffer": true, "bytes": true,
		"int": true, "int64": true, "float64": true, "bool": true, "map": true,
		"chan": true, "select": true, "case": true, "default": true, "defer": true,
		"go": true, "goto": true, "switch": true, "break": true, "continue": true,
		"fallthrough": true, "var": true,
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		matches := re.FindAllString(string(content), -1)
		for _, m := range matches {
			lower := strings.ToLower(m)
			if len(lower) < 4 {
				continue
			}
			if allowlist[lower] {
				continue
			}

			// Add to candidates list (unique by lower case, but store original casing found first?)
			// The issue is if we have "Reciever" and "reciever", we want to catch both?
			// The current logic sends words to AI.
			// If we send "Reciever", AI says "Receiver".
			// If we send "reciever", AI says "receiver".
			// We should probably track unique words *exactly as they appear* to be safe for replacement.

			if !candidates[m] {
				candidates[m] = true
			}

			// Add to file map
			list := fileMap[m]
			found := false
			for _, f := range list {
				if f == file {
					found = true
					break
				}
			}
			if !found {
				fileMap[m] = append(fileMap[m], file)
			}
		}
	}

	var result []string
	for k := range fileMap {
		result = append(result, k)
	}
	return result, fileMap
}

func checkTyposWithAI(ctx context.Context, words []string) (map[string]string, error) {
	// Limit batch size?
	// If too many words, AI might hallucinate or truncate.
	// Let's take top 50 suspicious ones (rarest?) or just first 50.
	if len(words) > 50 {
		words = words[:50]
	}

	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-typo")
	if err != nil {
		return nil, err
	}

	wordsJSON, _ := json.Marshal(words)

	prompt := fmt.Sprintf(`You are a spell checker for a software project.
Review the following list of words found in the codebase.
Identify which ones are likely typos (misspelled English words or common dev terms).
Ignore proper nouns, project-specific jargon, or valid variable abbreviations.

Words:
%s

Return a JSON object where the key is the typo and the value is the correct spelling.
Example: {"reciever": "receiver", "funtion": "function"}
Return an empty object {} if no typos are found.
`, string(wordsJSON))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	cleanResp := utils.CleanJSONBlock(resp)
	var typos map[string]string
	if err := json.Unmarshal([]byte(cleanResp), &typos); err != nil {
		// Try to parse partial or just log warning?
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// Filter out if correction is same as original (AI quirk)
	finalTypos := make(map[string]string)
	for k, v := range typos {
		if k != v && v != "" {
			finalTypos[k] = v
		}
	}

	return finalTypos, nil
}

func findFilesWithWord(fileMap map[string][]string, word string) []string {
	return fileMap[word]
}

func replaceInFile(path, old, newWord string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Simple replace?
	// Risky: "mode" -> "node" might replace "model" -> "nodel"
	// Use regex word boundary
	re, err := regexp.Compile(`\b` + regexp.QuoteMeta(old) + `\b`)
	if err != nil {
		return err
	}

	newContent := re.ReplaceAll(content, []byte(newWord))

	return os.WriteFile(path, newContent, 0644)
}
