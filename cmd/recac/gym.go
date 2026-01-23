package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/runner"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// GymChallenge defines a coding challenge for the agent.
type GymChallenge struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Language    string `yaml:"language" json:"language"`
	Tests       string `yaml:"tests" json:"tests"`
	TestFile    string `yaml:"test_file" json:"test_file"` // Name of the test file to create (e.g. test_primes.py)
	Timeout     int    `yaml:"timeout" json:"timeout"`     // Timeout in seconds
}

// GymResult holds the result of a gym session.
type GymResult struct {
	Challenge string
	Passed    bool
	Output    string
	Duration  time.Duration
	Cost      float64
}

// Factory variables for testing
var (
	gymDockerClientFactory = func(project string) (runner.DockerClient, error) {
		return docker.NewClient(project)
	}
	gymAgentFactory   = agent.NewAgent
	gymSessionFactory = runner.NewSession
)

var gymCmd = &cobra.Command{
	Use:   "gym [path]",
	Short: "Train and evaluate the agent on coding challenges",
	Long: `Runs the autonomous agent against a suite of coding challenges defined in YAML/JSON.
Useful for regression testing prompts, evaluating models, and measuring improvement.

Example:
  recac gym ./gym/challenges.yaml`,
	RunE: runGym,
}

func init() {
	if rootCmd != nil {
		rootCmd.AddCommand(gymCmd)
	}
}

func runGym(cmd *cobra.Command, args []string) error {
	path := "./gym/challenges.yaml"
	if len(args) > 0 {
		path = args[0]
	}

	challenges, err := loadChallenges(path)
	if err != nil {
		return fmt.Errorf("failed to load challenges: %w", err)
	}

	fmt.Printf("Loaded %d challenges from %s\n", len(challenges), path)

	var results []GymResult

	for _, challenge := range challenges {
		fmt.Printf("\nRunning challenge: %s\n", challenge.Name)
		res, err := runGymSession(cmd.Context(), challenge)
		if err != nil {
			fmt.Printf("Error running challenge %s: %v\n", challenge.Name, err)
			results = append(results, GymResult{
				Challenge: challenge.Name,
				Passed:    false,
				Output:    err.Error(),
			})
		} else {
			results = append(results, *res)
		}
	}

	printGymReport(cmd, results)
	return nil
}

func loadChallenges(path string) ([]GymChallenge, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		// TODO: Support directory loading
		return nil, fmt.Errorf("directory loading not implemented yet")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var challenges []GymChallenge
	// Try parsing as list first
	if err := yaml.Unmarshal(data, &challenges); err == nil {
		return challenges, nil
	}

	// Maybe it's a single object
	var single GymChallenge
	if err := yaml.Unmarshal(data, &single); err == nil {
		return []GymChallenge{single}, nil
	}

	return nil, fmt.Errorf("failed to parse challenges file")
}

func runGymSession(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
	start := time.Now()

	// 1. Create Temp Workspace
	workspace, err := os.MkdirTemp("", "recac-gym-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp workspace: %w", err)
	}
	defer os.RemoveAll(workspace) // Cleanup

	// 2. Initialize Docker Client
	// We use a unique project ID for isolation
	projectID := fmt.Sprintf("gym-%s-%d", strings.ReplaceAll(challenge.Name, " ", "-"), time.Now().Unix())

	d, err := gymDockerClientFactory(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to init docker: %w", err)
	}
	// Session uses the interface which doesn't have Close().
	// For real usage, d is *docker.Client which has Close(), but we cast to interface.
	// If we want to close, we need to type assert or update interface.
	// For CLI tool, OS cleanup is fine.

	// 3. Initialize Agent
	// Read from config or env
	provider := os.Getenv("RECAC_GYM_PROVIDER")
	if provider == "" {
		provider = "gemini" // Default cheap model for gym
	}
	model := os.Getenv("RECAC_GYM_MODEL")
	if model == "" {
		model = "gemini-1.5-flash-latest"
	}
	apiKey := os.Getenv("RECAC_GYM_API_KEY") // Optional override

	// Use factory
	a, err := gymAgentFactory(provider, apiKey, model, workspace, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to init agent: %w", err)
	}

	// 4. Initialize Session
	// Image: Use default or env
	image := os.Getenv("RECAC_AGENT_IMAGE")
	if image == "" {
		image = "recac-agent:latest" // Use local default
	}

	sess := gymSessionFactory(d, a, workspace, image, projectID, provider, model, 1)

	// Configure Session
	sess.MaxIterations = 10 // Limit iterations for gym
	if challenge.Timeout > 0 {
		// Enforced via context later
	}
	sess.SpecContent = challenge.Description

	// Write Tests to Workspace
	if challenge.Tests != "" && challenge.TestFile != "" {
		testPath := filepath.Join(workspace, challenge.TestFile)
		if err := os.WriteFile(testPath, []byte(challenge.Tests), 0644); err != nil {
			return nil, fmt.Errorf("failed to write test file: %w", err)
		}
	}

	// 5. Run Session
	// Use timeout context
	timeout := 5 * time.Minute
	if challenge.Timeout > 0 {
		timeout = time.Duration(challenge.Timeout) * time.Second
	}
	sessCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := sess.Start(sessCtx); err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}
	defer sess.Stop(context.Background())

	// Run Loop
	// We ignore "ErrMaxIterations" as it might just mean it tried its best.
	// We rely on the tests to verify.
	if err := sess.RunLoop(sessCtx); err != nil {
		if err != runner.ErrMaxIterations && err != context.DeadlineExceeded {
			fmt.Printf("Session error: %v\n", err)
		}
	}

	// 6. Verification
	// Run the test command
	// Assume python for now or infer from language
	var testCmd []string
	if challenge.Language == "python" {
		testCmd = []string{"python3", challenge.TestFile}
	} else if challenge.Language == "go" {
		testCmd = []string{"go", "test", "-v", challenge.TestFile}
	} else {
		// Fallback: try to execute the file directly
		testCmd = []string{"./" + challenge.TestFile}
	}

	output, err := d.Exec(ctx, sess.GetContainerID(), testCmd)
	passed := err == nil

	return &GymResult{
		Challenge: challenge.Name,
		Passed:    passed,
		Output:    output,
		Duration:  time.Since(start),
		Cost:      0.0, // TODO: Extract from agent state
	}, nil
}

func printGymReport(cmd *cobra.Command, results []GymResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "CHALLENGE\tRESULT\tDURATION\tOUTPUT (Last Line)")

	passedCount := 0
	for _, r := range results {
		status := "FAIL 🔴"
		if r.Passed {
			status = "PASS 🟢"
			passedCount++
		}

		// Get last line of output for brevity
		lines := strings.Split(strings.TrimSpace(r.Output), "\n")
		lastLine := ""
		if len(lines) > 0 {
			lastLine = lines[len(lines)-1]
			if len(lastLine) > 50 {
				lastLine = lastLine[:47] + "..."
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Challenge, status, r.Duration.Round(time.Second), lastLine)
	}
	w.Flush()

	fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: %d/%d passed (%.1f%%)\n", passedCount, len(results), float64(passedCount)/float64(len(results))*100)
}
