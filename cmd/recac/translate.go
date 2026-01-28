package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	translateTarget string
	translateOutput string
	translateForce  bool
)

var translateCmd = &cobra.Command{
	Use:   "translate [file]",
	Short: "Translate code from one language to another using AI",
	Long:  `Translate a source code file to a specified target language using the configured AI agent.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTranslate,
}

func init() {
	rootCmd.AddCommand(translateCmd)
	translateCmd.Flags().StringVarP(&translateTarget, "target", "t", "", "Target language (e.g., go, python, rust) (required)")
	translateCmd.MarkFlagRequired("target")
	translateCmd.Flags().StringVarP(&translateOutput, "output", "o", "", "Output file path")
	translateCmd.Flags().BoolVarP(&translateForce, "force", "f", false, "Overwrite existing output file")
}

func runTranslate(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	// 1. Read Input
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file %s: %w", inputFile, err)
	}

	// 2. Determine Output Path
	outputPath := translateOutput
	if outputPath == "" {
		ext := getExtensionForLanguage(translateTarget)
		base := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
		outputPath = filepath.Join(filepath.Dir(inputFile), base+ext)
	}

	// 3. Check for Overwrite
	if _, err := os.Stat(outputPath); err == nil && !translateForce {
		return fmt.Errorf("output file %s already exists. Use --force to overwrite", outputPath)
	}

	// 4. Initialize Agent
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	projectName := filepath.Base(cwd)
	ag, err := agentClientFactory(ctx, provider, model, cwd, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 5. Construct Prompt
	prompt := fmt.Sprintf(`Translate the following code to %s.
Maintain the logic, comments, and structure where appropriate.
Use idiomatic %s conventions.
Return ONLY the translated code. Do not include markdown code blocks or explanations.

Source File: %s
Code:
'''
%s
'''`, translateTarget, translateTarget, inputFile, string(content))

	fmt.Fprintf(cmd.OutOrStdout(), "Translating %s to %s...\n", inputFile, translateTarget)

	// 6. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to translate code: %w", err)
	}

	// 7. Clean Output
	translatedCode := utils.CleanCodeBlock(resp)

	// 8. Write Output
	if err := os.WriteFile(outputPath, []byte(translatedCode), 0644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Translation saved to %s\n", outputPath)
	return nil
}

func getExtensionForLanguage(lang string) string {
	lang = strings.ToLower(lang)
	switch lang {
	case "go", "golang":
		return ".go"
	case "python", "py":
		return ".py"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "rust", "rs":
		return ".rs"
	case "java":
		return ".java"
	case "c":
		return ".c"
	case "cpp", "c++":
		return ".cpp"
	case "csharp", "c#":
		return ".cs"
	case "ruby", "rb":
		return ".rb"
	case "php":
		return ".php"
	case "swift":
		return ".swift"
	case "kotlin", "kt":
		return ".kt"
	case "scala":
		return ".scala"
	case "haskell", "hs":
		return ".hs"
	case "lua":
		return ".lua"
	case "shell", "bash", "sh":
		return ".sh"
	case "yaml", "yml":
		return ".yaml"
	case "json":
		return ".json"
	case "xml":
		return ".xml"
	case "html":
		return ".html"
	case "css":
		return ".css"
	case "sql":
		return ".sql"
	default:
		// Default fallback
		return "_translated.txt"
	}
}
