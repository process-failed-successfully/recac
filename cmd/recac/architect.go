package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var architectCmd = &cobra.Command{
	Use:   "architect",
	Short: "Generate and validate system architecture from spec",
	Long:  "Reads app_spec.txt, uses AI to generate architecture.yaml and contracts, then validates them.",
	Run:   runArchitectCmd,
}

func init() {
	rootCmd.AddCommand(architectCmd)
	architectCmd.Flags().String("spec", "app_spec.txt", "Path to application specification file")
	architectCmd.Flags().String("out", ".recac/architecture", "Output directory for generated artifacts")
}

func runArchitectCmd(cmd *cobra.Command, args []string) {
	specPath, _ := cmd.Flags().GetString("spec")
	outDir, _ := cmd.Flags().GetString("out")
	
	ctx := context.Background()

	// 1. Read Spec
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading spec: %v\n", err)
		os.Exit(1)
	}

	// 2. Init Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, ".", "recac-architect")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing agent: %v\n", err)
		os.Exit(1)
	}

	// 3. Generate
	fmt.Println("Architecting system...")
	files, err := generateArchitecture(ctx, ag, string(specContent))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Generation failed: %v\n", err)
		os.Exit(1)
	}

	// 4. Write Files
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	for path, content := range files {
		fullPath := filepath.Join(outDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create dir for %s: %v\n", path, err)
			os.Exit(1)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", path)
	}

	// 5. Validate
	fmt.Println("Validating architecture...")
	archPath := filepath.Join(outDir, "architecture.yaml")
	archData, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Missing architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(archData, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse architecture.yaml: %v\n", err)
		os.Exit(1)
	}

	// Use a validator that knows about the output directory base path
	validator := architecture.NewValidator(&BasePathFS{Base: outDir})
	if err := validator.Validate(&arch); err != nil {
		fmt.Fprintf(os.Stderr, "VALIDATION FAILED:\n%v\n", err)
		os.Exit(1)
	}

	fmt.Println("SUCCESS: Architecture is valid.")
}

// generateArchitecture calls the agent and parses the JSON response
func generateArchitecture(ctx context.Context, ag agent.Agent, spec string) (map[string]string, error) {
	prompt, err := prompts.GetPrompt(prompts.ArchitectAgent, map[string]string{"spec": spec})
	if err != nil {
		return nil, err
	}

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Extract JSON
	jsonStr := resp
	if start := strings.Index(jsonStr, "```json"); start != -1 {
		jsonStr = jsonStr[start+7:]
		if end := strings.Index(jsonStr, "```"); end != -1 {
			jsonStr = jsonStr[:end]
		}
	} else if start := strings.Index(jsonStr, "{"); start != -1 {
		// Fallback: try to find the first curly brace
		jsonStr = jsonStr[start:]
		if end := strings.LastIndex(jsonStr, "}"); end != -1 {
			jsonStr = jsonStr[:end+1]
		}
	}
	jsonStr = strings.TrimSpace(jsonStr)

	var files map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &files); err != nil {
		return nil, fmt.Errorf("json parse error: %v\nResponse: %s", err, resp)
	}

	return files, nil
}

// BasePathFS wraps os calls to be relative to a base directory
type BasePathFS struct {
	Base string
}

func (b *BasePathFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(b.Base, name))
}
