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
	translatePrompt string
	translateStdout bool
)

var translateCmd = &cobra.Command{
	Use:   "translate [files...]",
	Short: "Translate code to another language",
	Long: `Translate source code files from one programming language to another using the AI agent.
It attempts to preserve logic, comments, and structure while adapting to the idioms of the target language.`,
	Example: `  recac translate --target python ./pkg/utils/math.go
  recac translate --target rust --output ./src/lib.rs ./pkg/utils/math.go
  recac translate --target typescript ./*.js`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTranslate,
}

func init() {
	rootCmd.AddCommand(translateCmd)
	translateCmd.Flags().StringVarP(&translateTarget, "target", "t", "", "Target programming language (e.g., python, rust)")
	translateCmd.MarkFlagRequired("target")
	translateCmd.Flags().StringVarP(&translateOutput, "output", "o", "", "Output file path (only for single input file)")
	translateCmd.Flags().StringVarP(&translatePrompt, "prompt", "p", "", "Additional instructions for translation")
	translateCmd.Flags().BoolVar(&translateStdout, "stdout", false, "Print to stdout instead of saving to file")
}

func runTranslate(cmd *cobra.Command, args []string) error {
	// Validation
	if len(args) > 1 && translateOutput != "" {
		return fmt.Errorf("cannot use --output with multiple input files")
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Initialize Agent
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-translate")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	for _, path := range args {
		// 1. Read File
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// 2. Construct Prompt
		basePrompt := fmt.Sprintf(`You are an expert polyglot programmer.
Translate the following code to %s.
Use idiomatic %s code, patterns, and conventions.
Preserve comments and documentation where appropriate.
Return ONLY the code block. Do not include explanations.

Source File: %s
Code:
%s
`, translateTarget, translateTarget, filepath.Base(path), string(content))

		if translatePrompt != "" {
			basePrompt += fmt.Sprintf("\nAdditional Instructions:\n%s\n", translatePrompt)
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "🔄 Translating %s to %s...\n", path, translateTarget)

		// 3. Call Agent
		resp, err := ag.Send(ctx, basePrompt)
		if err != nil {
			return fmt.Errorf("agent failed to translate %s: %w", path, err)
		}

		// 4. Process Response
		translatedCode := utils.CleanCodeBlock(resp)

		// 5. Output
		if translateStdout {
			fmt.Fprintln(cmd.OutOrStdout(), translatedCode)
			continue
		}

		outPath := translateOutput
		if outPath == "" {
			outPath = determineOutputPath(path, translateTarget)
		}

		if err := os.WriteFile(outPath, []byte(translatedCode), 0644); err != nil {
			return fmt.Errorf("failed to write output to %s: %w", outPath, err)
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Saved to %s\n", outPath)
	}

	return nil
}

func determineOutputPath(inputPath, targetLang string) string {
	ext := getExtensionForLanguage(targetLang)
	base := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
	return base + ext
}

func getExtensionForLanguage(lang string) string {
	lang = strings.ToLower(lang)
	switch lang {
	case "python", "py":
		return ".py"
	case "go", "golang":
		return ".go"
	case "rust", "rs":
		return ".rs"
	case "javascript", "js", "node":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "java":
		return ".java"
	case "c":
		return ".c"
	case "cpp", "c++":
		return ".cpp"
	case "c#", "csharp", "cs":
		return ".cs"
	case "ruby", "rb":
		return ".rb"
	case "php":
		return ".php"
	case "html":
		return ".html"
	case "css":
		return ".css"
	case "swift":
		return ".swift"
	case "kotlin", "kt":
		return ".kt"
	case "scala":
		return ".scala"
	case "shell", "bash", "sh":
		return ".sh"
	default:
		// Fallback: try to use the language name itself if it looks like an extension, otherwise default to .txt
		if len(lang) < 5 {
			return "." + lang
		}
		return ".txt"
	}
}
