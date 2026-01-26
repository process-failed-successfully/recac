package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"recac/internal/utils"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	envFile     string
	envExample  string
	envForce    bool
	envDetailed bool
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage local environment variables",
	Long:  `Manage your .env files: check for missing keys, sync with example.env, and generate new configurations using AI.`,
}

var envCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for missing keys in .env compared to example.env",
	RunE:  runEnvCheck,
}

var envSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Update example.env with keys from .env (sanitizing values)",
	RunE:  runEnvSync,
}

var envGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a .env file from example.env using AI to suggest defaults",
	RunE:  runEnvGenerate,
}

func init() {
	rootCmd.AddCommand(envCmd)

	envCmd.PersistentFlags().StringVarP(&envFile, "file", "f", ".env", "Path to the environment file")
	envCmd.PersistentFlags().StringVarP(&envExample, "example", "e", "example.env", "Path to the example/template file")

	envCheckCmd.Flags().BoolVarP(&envDetailed, "detailed", "d", false, "Show detailed comparison")
	envCmd.AddCommand(envCheckCmd)

	envCmd.AddCommand(envSyncCmd)

	envGenerateCmd.Flags().BoolVar(&envForce, "force", false, "Overwrite existing .env file")
	envCmd.AddCommand(envGenerateCmd)
}

func runEnvCheck(cmd *cobra.Command, args []string) error {
	// Load files
	envMap, err := godotenv.Read(envFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", envFile, err)
	}

	exampleMap, err := godotenv.Read(envExample)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", envExample, err)
	}

	var missingKeys []string
	for key := range exampleMap {
		if _, exists := envMap[key]; !exists {
			missingKeys = append(missingKeys, key)
		}
	}

	var extraKeys []string
	if envDetailed {
		for key := range envMap {
			if _, exists := exampleMap[key]; !exists {
				extraKeys = append(extraKeys, key)
			}
		}
	}

	sort.Strings(missingKeys)
	sort.Strings(extraKeys)

	if len(missingKeys) == 0 {
		fmt.Printf("✅ %s is in sync with %s (all required keys present)\n", envFile, envExample)
	} else {
		fmt.Printf("❌ %s is missing %d keys from %s:\n", envFile, len(missingKeys), envExample)
		for _, key := range missingKeys {
			fmt.Printf("  - %s\n", key)
		}
	}

	if envDetailed && len(extraKeys) > 0 {
		fmt.Printf("\nℹ️  %s has %d extra keys not in %s:\n", envFile, len(extraKeys), envExample)
		for _, key := range extraKeys {
			fmt.Printf("  - %s\n", key)
		}
	}

	if len(missingKeys) > 0 {
		return fmt.Errorf("environment check failed")
	}

	return nil
}

func runEnvSync(cmd *cobra.Command, args []string) error {
	envMap, err := godotenv.Read(envFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", envFile, err)
	}

	// Read existing example to preserve comments if possible?
	// godotenv.Read returns a map, stripping comments.
	// To do this properly requires a parser that preserves comments.
	// But for MVP, we just rewrite it or append.
	// Let's Read the example file to check which keys already exist.
	exampleMap, _ := godotenv.Read(envExample)

	var newKeys []string
	for key := range envMap {
		if _, exists := exampleMap[key]; !exists {
			newKeys = append(newKeys, key)
		}
	}

	if len(newKeys) == 0 {
		fmt.Println("Example file is already up to date.")
		return nil
	}

	sort.Strings(newKeys)

	// Append to example file
	f, err := os.OpenFile(envExample, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", envExample, err)
	}
	defer f.Close()

	// Ensure we start on a new line
	stat, _ := f.Stat()
	if stat.Size() > 0 {
		buf := make([]byte, 1)
		_, err := f.ReadAt(buf, stat.Size()-1)
		if err == nil && buf[0] != '\n' {
			f.WriteString("\n")
		}
	}

	for _, key := range newKeys {
		// We sanitize the value.
		// For now, just empty string or placeholder.
		val := ""
		if strings.Contains(strings.ToLower(key), "key") || strings.Contains(strings.ToLower(key), "token") || strings.Contains(strings.ToLower(key), "secret") || strings.Contains(strings.ToLower(key), "password") {
			val = "your_" + strings.ToLower(key)
		}

		_, err := f.WriteString(fmt.Sprintf("%s=%s\n", key, val))
		if err != nil {
			return err
		}
		fmt.Printf("Added %s to %s\n", key, envExample)
	}

	return nil
}

func runEnvGenerate(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(envFile); err == nil && !envForce {
		return fmt.Errorf("%s already exists. Use --force to overwrite", envFile)
	}

	// Read example
	content, err := os.ReadFile(envExample)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", envExample, err)
	}

	// Agent Setup
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-env")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Println("🤖 Analyzing project to generate environment configuration...")

	prompt := fmt.Sprintf(`You are a DevOps expert.
Generate a valid .env file based on the following template (example.env).
Fill in sensible defaults where possible based on standard conventions (e.g. for localhost, standard ports).
For secrets (API keys, passwords), use placeholders like "INSERT_HERE".
Do NOT invent new keys that are not in the template unless necessary for a standard setup.
Keep existing comments if possible or add helpful ones.

Template:
'''
%s
'''

Output ONLY the content of the new .env file.`, string(content))

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	cleanResp := utils.CleanCodeBlock(resp)

	if err := os.WriteFile(envFile, []byte(cleanResp), 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", envFile, err)
	}

	fmt.Printf("✅ Generated %s from %s\n", envFile, envExample)
	return nil
}
