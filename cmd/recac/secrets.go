package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"recac/internal/secrets"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	secretsKeyFile string
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage encrypted secrets (like .env files)",
	Long:  `Securely manage secrets by encrypting configuration files (e.g. .env) using a master key.`,
}

var secretsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a new master key",
	RunE:  runSecretsInit,
}

var secretsEncryptCmd = &cobra.Command{
	Use:   "encrypt [file]",
	Short: "Encrypt a file",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSecretsEncrypt,
}

var secretsDecryptCmd = &cobra.Command{
	Use:   "decrypt [file]",
	Short: "Decrypt a file",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSecretsDecrypt,
}

var secretsRunCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "Run a command with decrypted environment variables",
	Long:  `Decrypts .env.enc and runs the provided command with the environment variables set.`,
	Example: `  recac secrets run -- npm start
  recac secrets run ./my-script.sh`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSecretsRun,
}

func init() {
	rootCmd.AddCommand(secretsCmd)
	secretsCmd.AddCommand(secretsInitCmd)
	secretsCmd.AddCommand(secretsEncryptCmd)
	secretsCmd.AddCommand(secretsDecryptCmd)
	secretsCmd.AddCommand(secretsRunCmd)

	secretsCmd.PersistentFlags().StringVar(&secretsKeyFile, "key-file", ".recac.key", "Path to the master key file")
	secretsDecryptCmd.Flags().StringP("output", "o", "", "Write output to file instead of stdout")
}

func runSecretsInit(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(secretsKeyFile); err == nil {
		return fmt.Errorf("key file %s already exists", secretsKeyFile)
	}

	key, err := secrets.GenerateKey()
	if err != nil {
		return err
	}

	if err := secrets.SaveKey(secretsKeyFile, key); err != nil {
		return err
	}

	fmt.Printf("✅ Generated master key: %s\n", secretsKeyFile)
	fmt.Println("⚠️  Make sure to add this file to .gitignore!")

	// Check if .gitignore exists
	if _, err := os.Stat(".gitignore"); err == nil {
		// Read it
		content, _ := os.ReadFile(".gitignore")
		if !strings.Contains(string(content), secretsKeyFile) {
			f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				defer f.Close()
				f.WriteString("\n" + secretsKeyFile + "\n")
				fmt.Println("✅ Added to .gitignore")
			}
		} else {
			fmt.Println("ℹ️  Already in .gitignore")
		}
	}

	return nil
}

func runSecretsEncrypt(cmd *cobra.Command, args []string) error {
	inputFile := ".env"
	if len(args) > 0 {
		inputFile = args[0]
	}

	outputFile := inputFile + ".enc"

	key, err := secrets.LoadKey(secretsKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load key from %s: %w", secretsKeyFile, err)
	}

	plaintext, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", inputFile, err)
	}

	ciphertext, err := secrets.Encrypt(plaintext, key)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	if err := os.WriteFile(outputFile, ciphertext, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputFile, err)
	}

	fmt.Printf("🔒 Encrypted %s -> %s\n", inputFile, outputFile)
	return nil
}

func runSecretsDecrypt(cmd *cobra.Command, args []string) error {
	inputFile := ".env.enc"
	if len(args) > 0 {
		inputFile = args[0]
	}

	outputFile, _ := cmd.Flags().GetString("output")

	key, err := secrets.LoadKey(secretsKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load key from %s: %w", secretsKeyFile, err)
	}

	ciphertext, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", inputFile, err)
	}

	plaintext, err := secrets.Decrypt(ciphertext, key)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, plaintext, 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Printf("🔓 Decrypted %s -> %s\n", inputFile, outputFile)
	} else {
		// Output to stdout
		fmt.Print(string(plaintext))
	}
	return nil
}

func runSecretsRun(cmd *cobra.Command, args []string) error {
	// 1. Decrypt .env.enc
	encryptedFile := ".env.enc"
	if _, err := os.Stat(encryptedFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found", encryptedFile)
	}

	key, err := secrets.LoadKey(secretsKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load key: %w", err)
	}

	ciphertext, err := os.ReadFile(encryptedFile)
	if err != nil {
		return err
	}

	plaintext, err := secrets.Decrypt(ciphertext, key)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// 2. Parse Env
	envMap, err := godotenv.Unmarshal(string(plaintext))
	if err != nil {
		return fmt.Errorf("failed to parse decrypted .env: %w", err)
	}

	envVars := os.Environ()
	for k, v := range envMap {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// 3. Run Command
	// Handle case where args[0] is "--" (handled by cobra usually but good to be safe)
	cmdArgs := args
	if cmdArgs[0] == "--" {
		cmdArgs = cmdArgs[1:]
	}
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command provided")
	}

	execCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	execCmd.Env = envVars
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		// Pass through exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}
