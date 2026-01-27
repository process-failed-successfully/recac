package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	convertFrom string
	convertTo   string
	convertOut  string
	convertAI   bool
)

var convertCmd = &cobra.Command{
	Use:   "convert [file]",
	Short: "Convert files between formats (JSON, YAML, TOML)",
	Long: `Convert data files from one format to another.
Supports deterministic conversion between JSON, YAML, and TOML.
Use --ai to perform smart conversions for other formats or unstructured data.

Examples:
  recac convert config.yaml --to json
  recac convert data.json --to toml
  recac convert legacy.conf --to json --ai`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConvert,
}

func init() {
	rootCmd.AddCommand(convertCmd)
	convertCmd.Flags().StringVarP(&convertFrom, "from", "f", "auto", "Source format (json, yaml, toml, auto)")
	convertCmd.Flags().StringVarP(&convertTo, "to", "t", "json", "Target format (json, yaml, toml)")
	convertCmd.Flags().StringVarP(&convertOut, "out", "o", "", "Output file path (default stdout)")
	convertCmd.Flags().BoolVar(&convertAI, "ai", false, "Use AI for conversion")
}

func runConvert(cmd *cobra.Command, args []string) error {
	// 1. Read Input
	var inputBytes []byte
	var err error
	var filename string

	if len(args) > 0 {
		filename = args[0]
		inputBytes, err = os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
	} else {
		// Read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			inputBytes, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			filename = "stdin"
		} else {
			return fmt.Errorf("please provide a file or pipe data to stdin")
		}
	}

	// 2. Determine Source Format
	srcFormat := convertFrom
	if srcFormat == "auto" {
		if filename != "stdin" {
			ext := strings.ToLower(filepath.Ext(filename))
			switch ext {
			case ".json":
				srcFormat = "json"
			case ".yaml", ".yml":
				srcFormat = "yaml"
			case ".toml":
				srcFormat = "toml"
			case ".xml":
				srcFormat = "xml"
			default:
				// If AI is enabled, unknown format is fine.
				// If not, we try to detect by content?
				srcFormat = "unknown"
			}
		}
	}

	// 3. Convert
	var outputBytes []byte

	if convertAI || srcFormat == "unknown" || srcFormat == "xml" {
		// Use AI
		// Note: We use AI for XML because generic XML->Map is tricky without schema
		outputBytes, err = convertWithAI(cmd.Context(), inputBytes, srcFormat, convertTo)
	} else {
		outputBytes, err = convertDeterministic(inputBytes, srcFormat, convertTo)
	}

	if err != nil {
		return err
	}

	// 4. Output
	if convertOut != "" {
		if err := os.WriteFile(convertOut, outputBytes, 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Converted to %s\n", convertOut)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), string(outputBytes))
	}

	return nil
}

func convertDeterministic(input []byte, from, to string) ([]byte, error) {
	var data interface{}

	// Unmarshal
	switch strings.ToLower(from) {
	case "json":
		if err := json.Unmarshal(input, &data); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(input, &data); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	case "toml":
		if err := toml.Unmarshal(input, &data); err != nil {
			return nil, fmt.Errorf("failed to parse TOML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported source format '%s' for deterministic conversion (try --ai)", from)
	}

	// Marshal
	switch strings.ToLower(to) {
	case "json":
		return json.MarshalIndent(data, "", "  ")
	case "yaml", "yml":
		return yaml.Marshal(data)
	case "toml":
		return toml.Marshal(data)
	default:
		return nil, fmt.Errorf("unsupported target format '%s'", to)
	}
}

func convertWithAI(ctx context.Context, input []byte, from, to string) ([]byte, error) {
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-convert")
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Convert the following data from %s to %s.
Return ONLY the raw %s content. Do not include markdown code blocks.

Data:
'''
%s
'''`, from, to, to, string(input))

	fmt.Fprintf(os.Stderr, "🤖 Converting using AI (%s -> %s)...\n", from, to)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed: %w", err)
	}

	cleaned := utils.CleanCodeBlock(resp)
	return []byte(cleaned), nil
}
