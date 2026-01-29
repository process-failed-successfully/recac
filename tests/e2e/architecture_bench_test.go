//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/architecture"
	"recac/internal/cmdutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestArchitectureGenerationBenchmark runs the Architect Agent multiple times
// and reports the pass/fail rate of the Validator.
// Usage: go test -v -tags=e2e ./tests/e2e/architecture_bench_test.go -args -provider=openrouter -model=...
func TestArchitectureGenerationBenchmark(t *testing.T) {
	// Configuration from Env or Defaults
	provider := os.Getenv("RECAC_PROVIDER")
	if provider == "" {
		provider = "openrouter" // Default
	}
	model := os.Getenv("RECAC_MODEL")
	if model == "" {
		model = "nvidia/nemotron-3-nano-30b-a3b:free" // Default
	}
	runs := 5 // Number of iterations

	t.Logf("Benchmarking Architecture Generation")
	t.Logf("Provider: %s", provider)
	t.Logf("Model:    %s", model)
	t.Logf("Runs:     %d", runs)

	ctx := context.Background()
	ag, err := cmdutils.GetAgentClient(ctx, provider, model, ".", "recac-bench")
	require.NoError(t, err, "Failed to create agent client")

	// Simple App Spec
	spec := `
System: Order Processing
Features:
1. API to create an order (POST /orders).
2. Save order to Database.
3. Publish "OrderCreated" event.
4. Worker listens to "OrderCreated" and sends an Email.
`

	passCount := 0
	
	for i := 0; i < runs; i++ {
		t.Run(fmt.Sprintf("Run_%d", i+1), func(t *testing.T) {
			// 1. Generate
			prompt, err := prompts.GetPrompt(prompts.ArchitectAgent, map[string]string{"spec": spec})
			require.NoError(t, err)

			start := time.Now()
			resp, err := ag.Send(ctx, prompt)
			require.NoError(t, err)
			duration := time.Since(start)

			t.Logf("Generation took %v", duration)

			// 2. Parse (Simulate the logic in architect.go)
			// We duplicate a bit of logic here to keep test self-contained or we could refactor.
			// For this test, I'll do basic JSON extraction.
			// ... (JSON extraction logic similar to architect.go) ...
			
			// For the sake of this test file, I'll rely on the fact that if the JSON is malformed, it's a failure.
			// But to properly use the Validator, I need to materialize the files to a temp dir.
			tempDir := t.TempDir()
			
			// Mocking the extraction/parsing part for brevity in this thought, 
			// but in real code I need to actually parse it.
			// Let's assume a helper function or copy the logic.
			files, err := parseResponse(resp)
			if err != nil {
				t.Logf("Failed to parse JSON: %v", err)
				t.Fail()
				return
			}

			// Write files
			for path, content := range files {
				fullPath := filepath.Join(tempDir, path)
				os.MkdirAll(filepath.Dir(fullPath), 0755)
				os.WriteFile(fullPath, []byte(content), 0644)
			}

			// 3. Validate
			archPath := filepath.Join(tempDir, "architecture.yaml")
			archData, err := os.ReadFile(archPath)
			if err != nil {
				t.Logf("architecture.yaml missing")
				t.Fail()
				return
			}

			var arch architecture.SystemArchitecture
			if err := yaml.Unmarshal(archData, &arch); err != nil {
				t.Logf("YAML Unmarshal failed: %v", err)
				t.Fail()
				return
			}

			validator := architecture.NewValidator(&RealDirFS{Base: tempDir})
			if err := validator.Validate(&arch); err != nil {
				t.Logf("Validation Failed: %v", err)
				t.Fail()
			} else {
				passCount++
			}
		})
	}

	passRate := float64(passCount) / float64(runs) * 100.0
	t.Logf("Final Results: %d/%d Passed (%.1f%%)", passCount, runs, passRate)
	
	if passRate < 80.0 {
		t.Logf("Warning: Pass rate below 80%%")
		// t.Fail() // Optional: fail the suite if quality is too low
	}
}

// Helpers

func parseResponse(jsonStr string) (map[string]string, error) {
	// ... (Same extraction logic as architect.go) ...
	// Since I can't import main package functions easily, I'll copy-paste or assume a shared util location.
	// For now, minimal implementation:

	if start := strings.Index(jsonStr, "```json"); start != -1 {
		jsonStr = jsonStr[start+7:]
		if end := strings.Index(jsonStr, "```"); end != -1 {
			jsonStr = jsonStr[:end]
		}
	} else if start := strings.Index(jsonStr, "{"); start != -1 {
		jsonStr = jsonStr[start:]
		if end := strings.LastIndex(jsonStr, "}"); end != -1 {
			jsonStr = jsonStr[:end+1]
		}
	}
	jsonStr = strings.TrimSpace(jsonStr)

	var files map[string]string
	err := json.Unmarshal([]byte(jsonStr), &files)
	return files, err
}

type RealDirFS struct {
	Base string
}

func (b *RealDirFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(b.Base, name))
}
