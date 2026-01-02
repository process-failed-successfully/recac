//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TestResult struct {
	Name    string
	Passed  bool
	Error   string
	Elapsed time.Duration
}

func main() {
	provider := os.Getenv("RECAC_PROVIDER")
	if provider == "" {
		provider = "openrouter"
	}
	model := os.Getenv("RECAC_MODEL")
	if model == "" {
		model = "mistralai/devstral-2512:free"
	}

	fmt.Printf("Starting Project Verification Suite...\n")
	fmt.Printf("Provider: %s\n", provider)
	fmt.Printf("Model:    %s\n\n", model)

	projects, err := os.ReadDir("scripts/e2e_projects")
	if err != nil {
		fmt.Printf("Failed to read e2e_projects: %v\n", err)
		os.Exit(1)
	}

	results := []TestResult{}
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		res := runTest(p.Name(), provider, model)
		results = append(results, res)
	}

	fmt.Printf("\n--- Verification Summary ---\n")
	allPassed := true
	for _, res := range results {
		status := "PASS"
		if !res.Passed {
			status = "FAIL"
			allPassed = false
		}
		fmt.Printf("[%s] %-20s (%v)\n", status, res.Name, res.Elapsed.Round(time.Second))
		if res.Error != "" {
			fmt.Printf("     Error: %s\n", res.Error)
		}
	}

	if !allPassed {
		os.Exit(1)
	}
}

func runTest(name, provider, model string) TestResult {
	fmt.Printf("Running Test: %s...\n", name)
	start := time.Now()

	tmpDir, err := os.MkdirTemp("", "recac-e2e-"+name)
	if err != nil {
		return TestResult{Name: name, Passed: false, Error: "failed to create temp dir: " + err.Error()}
	}
	// defer os.RemoveAll(tmpDir)

	// Copy app_spec.txt
	specSrc := filepath.Join("scripts/e2e_projects", name, "app_spec.txt")
	specDest := filepath.Join(tmpDir, "app_spec.txt")
	content, err := os.ReadFile(specSrc)
	if err != nil {
		return TestResult{Name: name, Passed: false, Error: "failed to read spec: " + err.Error()}
	}
	if err := os.WriteFile(specDest, content, 0644); err != nil {
		return TestResult{Name: name, Passed: false, Error: "failed to write spec: " + err.Error()}
	}

	// Load .env if it exists
	envVars := os.Environ()
	if envBytes, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(envBytes), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Simple key=value or key="value"
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, "\"")
				envVars = append(envVars, key+"="+value)
			}
		}
	}

	// Run recac
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/recac", "start",
		"--path", tmpDir,
		"--provider", provider,
		"--model", model,
		"--max-iterations", "30",
		"--allow-dirty",
		"--stream",
	)
	cmd.Env = envVars

	// We want to see output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return TestResult{Name: name, Passed: false, Error: "recac execution failed: " + err.Error(), Elapsed: time.Since(start)}
	}

	// Verification logic based on project name
	err = verify(name, tmpDir)
	if err != nil {
		return TestResult{Name: name, Passed: false, Error: "verification failed: " + err.Error(), Elapsed: time.Since(start)}
	}

	return TestResult{Name: name, Passed: true, Elapsed: time.Since(start)}
}

func verify(name, path string) error {
	switch name {
	case "calculator-go":
		if _, err := os.Stat(filepath.Join(path, "main.go")); os.IsNotExist(err) {
			return fmt.Errorf("main.go missing")
		}
	case "sort-python":
		if _, err := os.Stat(filepath.Join(path, "sort.py")); os.IsNotExist(err) {
			return fmt.Errorf("sort.py missing")
		}
	case "webserver-node":
		if _, err := os.Stat(filepath.Join(path, "package.json")); os.IsNotExist(err) {
			return fmt.Errorf("package.json missing")
		}
	case "sysutil-bash":
		if _, err := os.Stat(filepath.Join(path, "setup.sh")); os.IsNotExist(err) {
			return fmt.Errorf("setup.sh missing")
		}
	case "git-workflow":
		gitDir := filepath.Join(path, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			return fmt.Errorf(".git directory missing")
		}
	}
	return nil
}
