package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/architecture"
	"recac/internal/cmdutils"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var getAgentClientFunc = cmdutils.GetAgentClient

var architectCmd = &cobra.Command{
	Use:   "architect",
	Short: "Generate and validate system architecture from spec",
	Long:  "Reads app_spec.txt, uses AI to generate architecture.yaml and contracts, then validates them.",
	RunE:  runArchitectCmd,
}

func init() {
	rootCmd.AddCommand(architectCmd)
	architectCmd.Flags().String("spec", "app_spec.txt", "Path to application specification file")
	architectCmd.Flags().String("out", ".recac/architecture", "Output directory for generated artifacts")
}

func runArchitectCmd(cmd *cobra.Command, args []string) error {
	specPath, _ := cmd.Flags().GetString("spec")
	outDir, _ := cmd.Flags().GetString("out")

	ctx := context.Background()

	// 1. Read Spec
	specContent, err := readFileFunc(specPath)
	if err != nil {
		return fmt.Errorf("error reading spec: %w", err)
	}

	// 2. Init Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := getAgentClientFunc(ctx, provider, model, ".", "recac-architect")
	if err != nil {
		return fmt.Errorf("error initializing agent: %w", err)
	}

	// 3. Generate
	fmt.Println("Architecting system...")
	files, err := generateArchitecture(ctx, ag, string(specContent))
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	// 4. Write Files
	if err := mkdirAllFunc(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for path, content := range files {
		fullPath := filepath.Join(outDir, path)
		if err := mkdirAllFunc(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create dir for %s: %w", path, err)
		}
		if err := writeFileFunc(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
		fmt.Printf("Wrote %s\n", path)
	}

	// 5. Validate
	fmt.Println("Validating architecture...")
	archPath := filepath.Join(outDir, "architecture.yaml")
	archData, err := readFileFunc(archPath)
	if err != nil {
		return fmt.Errorf("missing architecture.yaml: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(archData, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	// Use a validator that knows about the output directory base path
	validator := architecture.NewValidator(&BasePathFS{Base: outDir})
	if err := validator.Validate(&arch); err != nil {
		return fmt.Errorf("VALIDATION FAILED:\n%w", err)
	}

	fmt.Println("SUCCESS: Architecture is valid.")
	return nil
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
	jsonStr := utils.CleanJSONBlock(resp)

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
	return osStatFunc(filepath.Join(b.Base, name))
}
