package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"recac/pkg/e2e/scenarios"
	"recac/pkg/e2e/state"

	"github.com/joho/godotenv"
)

func RunVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	var (
		stateFile string
		keepRepo  bool
	)
	fs.StringVar(&stateFile, "state-file", "e2e_state.json", "Path to state file")
	fs.BoolVar(&keepRepo, "keep-repo", false, "Keep the cloned repository for inspection")
	fs.Parse(args)

	_ = godotenv.Load()

	e2eCtx, err := state.Load(stateFile)
	if err != nil {
		return fmt.Errorf("failed to load state file: %w", err)
	}

	scenario, ok := scenarios.Registry[e2eCtx.ScenarioName]
	if !ok {
		return fmt.Errorf("unknown scenario: %s", e2eCtx.ScenarioName)
	}

	// Clone to temp dir
	tmpDir, err := os.MkdirTemp("", "e2e-check")
	if err != nil {
		return err
	}
	
	if keepRepo {
		log.Printf("Repository cloned to: %s", tmpDir)
	} else {
		defer os.RemoveAll(tmpDir)
	}

	token := os.Getenv("GITHUB_API_KEY")
	authRepo := e2eCtx.RepoURL
	if !strings.Contains(authRepo, "@") && token != "" {
		// Insert token into URL
		authRepo = strings.Replace(authRepo, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}

	log.Printf("Cloning repo to %s...", tmpDir)
	if err := runCommand("git", "clone", authRepo, tmpDir); err != nil {
		return fmt.Errorf("failed to clone: %w", err)
	}

	log.Printf("Running verification for scenario: %s", scenario.Name())
	return scenario.Verify(tmpDir, e2eCtx.TicketMap)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
