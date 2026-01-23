package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	i18nSource string
	i18nVerify bool
	i18nAuto   bool
)

var i18nCmd = &cobra.Command{
	Use:   "i18n [dir]",
	Short: "Manage and auto-translate i18n JSON files",
	Long: `Scans a directory for JSON translation files (e.g., en.json, es.json).
Identifies keys present in the source file (default: en.json) but missing in others.
Uses the configured AI agent to translate the missing keys and updates the files.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runI18n,
}

func init() {
	rootCmd.AddCommand(i18nCmd)
	i18nCmd.Flags().StringVarP(&i18nSource, "source", "s", "", "Source language file (e.g. en.json)")
	i18nCmd.Flags().BoolVar(&i18nVerify, "verify", false, "Only verify missing keys, do not translate (exit code 1 if missing)")
	i18nCmd.Flags().BoolVarP(&i18nAuto, "yes", "y", false, "Automatically accept translations without prompt")
}

func runI18n(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// 1. Scan for JSON files
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	// Filter out non-translation files (heuristic?)
	// For now, assume user points to a clean locales dir, or we filter by simple name structure.
	var locFiles []string
	for _, f := range files {
		base := filepath.Base(f)
		if base == "package.json" || base == "tsconfig.json" {
			continue
		}
		locFiles = append(locFiles, f)
	}

	if len(locFiles) < 2 {
		return fmt.Errorf("found %d JSON files in %s. Need at least 2 to compare (source and target)", len(locFiles), dir)
	}

	// 2. Determine Source
	sourceFile := ""
	if i18nSource != "" {
		// Verify it exists in the list
		found := false
		for _, f := range locFiles {
			if filepath.Base(f) == i18nSource || f == i18nSource {
				sourceFile = f
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("source file %s not found in scanned files", i18nSource)
		}
	} else {
		// Default to en.json or ask
		for _, f := range locFiles {
			if filepath.Base(f) == "en.json" {
				sourceFile = f
				break
			}
		}
		if sourceFile == "" {
			// Ask user
			options := make([]string, len(locFiles))
			for i, f := range locFiles {
				options[i] = filepath.Base(f)
			}
			var selection string
			err := askOneFunc(&survey.Select{
				Message: "Select source language file:",
				Options: options,
			}, &selection)
			if err != nil {
				return err
			}
			sourceFile = filepath.Join(dir, selection)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📖 Source: %s\n", filepath.Base(sourceFile))

	// 3. Load Source Keys
	sourceMap, err := loadJSON(sourceFile)
	if err != nil {
		return err
	}
	sourceFlat := flattenJSON(sourceMap, "")

	// 4. Check Targets
	var missingCount int
	for _, targetFile := range locFiles {
		if targetFile == sourceFile {
			continue
		}

		targetLang := strings.TrimSuffix(filepath.Base(targetFile), ".json")
		fmt.Fprintf(cmd.OutOrStdout(), "🔍 Checking %s...\n", filepath.Base(targetFile))

		targetMap, err := loadJSON(targetFile)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  Warning: failed to load %s: %v\n", targetFile, err)
			continue
		}
		targetFlat := flattenJSON(targetMap, "")

		// Find missing
		var missingKeys []string
		for k := range sourceFlat {
			if _, exists := targetFlat[k]; !exists {
				missingKeys = append(missingKeys, k)
			}
		}

		if len(missingKeys) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  ✅ Up to date.")
			continue
		}

		missingCount += len(missingKeys)
		fmt.Fprintf(cmd.OutOrStdout(), "  ❌ Missing %d keys.\n", len(missingKeys))

		if i18nVerify {
			continue
		}

		// Prepare for Translation
		sort.Strings(missingKeys)

		// Build map of missing keys -> source values
		toTranslate := make(map[string]interface{})
		for _, k := range missingKeys {
			toTranslate[k] = sourceFlat[k]
		}

		// Translate
		fmt.Fprintf(cmd.OutOrStdout(), "  🤖 Translating to %s...\n", targetLang)
		translated, err := translateKeys(cmd.Context(), toTranslate, filepath.Base(sourceFile), targetLang)
		if err != nil {
			return err
		}

		// Merge back
		// We unflatten the new keys into a map, then deep merge.
		newlyTranslatedMap := unflattenJSONMap(translated)
		finalMap := deepMerge(targetMap, newlyTranslatedMap)

		// Write
		if !i18nAuto {
			var confirm bool
			err := askOneFunc(&survey.Confirm{
				Message: fmt.Sprintf("Write %d new translations to %s?", len(translated), filepath.Base(targetFile)),
				Default: true,
			}, &confirm)
			if err != nil {
				return err
			}
			if !confirm {
				fmt.Fprintln(cmd.OutOrStdout(), "  Skipped.")
				continue
			}
		}

		if err := writeJSON(targetFile, finalMap); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetFile, err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "  💾 Saved.")
	}

	if i18nVerify && missingCount > 0 {
		return fmt.Errorf("verification failed: %d missing keys found", missingCount)
	}

	return nil
}

// Helpers

func loadJSON(path string) (map[string]interface{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeJSON(path string, data map[string]interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func flattenJSON(m map[string]interface{}, prefix string) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		if nested, ok := v.(map[string]interface{}); ok {
			flatNested := flattenJSON(nested, fullKey)
			for nk, nv := range flatNested {
				out[nk] = nv
			}
		} else {
			out[fullKey] = v
		}
	}
	return out
}

func unflattenJSONMap(flat map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range flat {
		parts := strings.Split(k, ".")
		current := out
		for i, part := range parts {
			if i == len(parts)-1 {
				current[part] = v
			} else {
				if _, exists := current[part]; !exists {
					current[part] = make(map[string]interface{})
				}
				if nextMap, ok := current[part].(map[string]interface{}); ok {
					current = nextMap
				} else {
					// Conflict: key is both value and object?
					// Overwrite or skip. For i18n usually structure is consistent.
					// Let's overwrite with map
					newMap := make(map[string]interface{})
					current[part] = newMap
					current = newMap
				}
			}
		}
	}
	return out
}

func deepMerge(base, overlay map[string]interface{}) map[string]interface{} {
	for k, v := range overlay {
		if subOverlay, ok := v.(map[string]interface{}); ok {
			if subBase, ok := base[k].(map[string]interface{}); ok {
				base[k] = deepMerge(subBase, subOverlay)
			} else {
				base[k] = v
			}
		} else {
			base[k] = v
		}
	}
	return base
}

func translateKeys(ctx context.Context, keys map[string]interface{}, sourceFile, targetLang string) (map[string]interface{}, error) {
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-i18n")
	if err != nil {
		return nil, err
	}

	keysJSON, _ := json.MarshalIndent(keys, "", "  ")

	prompt := fmt.Sprintf(`You are a professional translator and localization expert.
Translate the following JSON keys from the source file (%s) to %s.
Maintain all placeholders (e.g. {{name}}, {0}, %%s) exactly as they are.
Do not change the keys. Return ONLY the translated JSON object.

Source Keys:
%s`, sourceFile, targetLang, string(keysJSON))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	cleanResp := utils.CleanJSONBlock(resp)
	var translated map[string]interface{}
	if err := json.Unmarshal([]byte(cleanResp), &translated); err != nil {
		return nil, fmt.Errorf("failed to parse translation response: %w", err)
	}

	return translated, nil
}
