package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	i18nTargetLang string
	i18nOutput     string
	i18nForce      bool
)

var i18nCmd = &cobra.Command{
	Use:   "i18n",
	Short: "Internationalization and translation tools",
	Long:  `Utilities for managing localization files and translating content using AI.`,
}

var i18nTranslateCmd = &cobra.Command{
	Use:   "translate [source_file]",
	Short: "Translate a localization file to another language",
	Long: `Reads a source localization file (JSON) and translates it to the target language.
It intelligently merges with existing target files, only translating missing keys unless --force is used.`,
	Args: cobra.ExactArgs(1),
	RunE: runI18nTranslate,
}

func init() {
	rootCmd.AddCommand(i18nCmd)
	i18nCmd.AddCommand(i18nTranslateCmd)

	i18nTranslateCmd.Flags().StringVarP(&i18nTargetLang, "target", "t", "", "Target language code (e.g., 'es', 'fr') (required)")
	i18nTranslateCmd.MarkFlagRequired("target")
	i18nTranslateCmd.Flags().StringVarP(&i18nOutput, "output", "o", "", "Output file path (default: inferred from target)")
	i18nTranslateCmd.Flags().BoolVarP(&i18nForce, "force", "f", false, "Force re-translation of all keys")
}

func runI18nTranslate(cmd *cobra.Command, args []string) error {
	sourcePath := args[0]
	ctx := cmd.Context()

	// 1. Read Source
	sourceContent, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	var sourceMap map[string]interface{}
	if err := json.Unmarshal(sourceContent, &sourceMap); err != nil {
		return fmt.Errorf("failed to parse source JSON: %w", err)
	}

	// 2. Determine Output Path
	if i18nOutput == "" {
		// Infer from target lang. e.g., en.json -> es.json
		ext := filepath.Ext(sourcePath)
		dir := filepath.Dir(sourcePath)
		i18nOutput = filepath.Join(dir, i18nTargetLang+ext)
	}

	// 3. Read Existing Target (if any)
	existingMap := make(map[string]interface{})
	if _, err := os.Stat(i18nOutput); err == nil && !i18nForce {
		content, err := os.ReadFile(i18nOutput)
		if err == nil {
			// Ignore error if empty or invalid, just start fresh
			_ = json.Unmarshal(content, &existingMap)
		}
	}

	// 4. Identify Missing Keys
	keysToTranslate := make(map[string]interface{})

	// Recursive function to find missing keys
	var findMissing func(src, exist map[string]interface{}, target map[string]interface{})
	findMissing = func(src, exist map[string]interface{}, target map[string]interface{}) {
		for k, v := range src {
			if strVal, ok := v.(string); ok {
				// Leaf node
				if _, exists := exist[k]; !exists || i18nForce {
					target[k] = strVal
				}
			} else if mapVal, ok := v.(map[string]interface{}); ok {
				// Nested map
				existSub, _ := exist[k].(map[string]interface{})
				if existSub == nil {
					existSub = make(map[string]interface{})
				}
				targetSub := make(map[string]interface{})
				findMissing(mapVal, existSub, targetSub)
				if len(targetSub) > 0 {
					target[k] = targetSub
				}
			}
		}
	}

	findMissing(sourceMap, existingMap, keysToTranslate)

	if len(keysToTranslate) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No new keys to translate.")
		return nil
	}

	// 5. Translate
	fmt.Fprintf(cmd.OutOrStdout(), "Translating %d new keys/sections to %s...\n", countKeys(keysToTranslate), i18nTargetLang)

	keysJSON, _ := json.MarshalIndent(keysToTranslate, "", "  ")

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-i18n")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Translate the values in the following JSON to %s.
Keep the keys exactly the same.
Do not translate keys, only values.
Return valid JSON.

Input:
'''json
%s
'''`, i18nTargetLang, string(keysJSON))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	// Clean response
	resp = utils.CleanJSONBlock(resp)

	var translatedMap map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &translatedMap); err != nil {
		return fmt.Errorf("failed to parse agent response: %w", err)
	}

	// 6. Merge Back
	mergeMaps(existingMap, translatedMap)

	// 7. Write Output
	finalJSON, _ := json.MarshalIndent(existingMap, "", "  ")
	if err := os.WriteFile(i18nOutput, finalJSON, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Translations saved to %s\n", i18nOutput)

	return nil
}

func countKeys(m map[string]interface{}) int {
	count := 0
	for _, v := range m {
		if _, ok := v.(map[string]interface{}); ok {
			count += countKeys(v.(map[string]interface{}))
		} else {
			count++
		}
	}
	return count
}

func mergeMaps(base, overlay map[string]interface{}) {
	for k, v := range overlay {
		if subMap, ok := v.(map[string]interface{}); ok {
			if _, exists := base[k]; !exists {
				base[k] = make(map[string]interface{})
			}
			if baseSub, ok := base[k].(map[string]interface{}); ok {
				mergeMaps(baseSub, subMap)
			} else {
				// Type mismatch or base was not a map, overwrite
				base[k] = v
			}
		} else {
			base[k] = v
		}
	}
}
