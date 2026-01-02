//go:build ignore

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
	"time"

	"github.com/docker/docker/api/types"
)

// MockAgent implements agent.Agent
type MockAgent struct{}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	fmt.Println("    [MockAgent] Received prompt length:", len(prompt))
	if strings.Contains(prompt, "INITIALIZER") {
		return "Plan: Create a hello world file.\nCommand: echo 'Hello Real World' > hello.txt", nil
	}
	return "Mock response", nil
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk("Mock response")
	}
	return "Mock response", nil
}

func main() {
	fmt.Println("Starting End-to-End Smoke Test...")

	// 1. Setup Workspace
	tmpDir, err := os.MkdirTemp("", "recac-smoke-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)
	fmt.Printf("Workspace: %s\n", tmpDir)

	// Create Spec
	specFile := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specFile, []byte("Create a hello world file."), 0644)

	// 2. Init Dependencies
	// Use Mock Docker Client to avoid needing actual Docker daemon in this environment
	dClient, mockAPI := docker.NewMockClient()

	// Configure Mock Docker to succeed
	mockAPI.PingFunc = func(ctx context.Context) (types.Ping, error) { return types.Ping{}, nil }

	// Determine Agent
	agentType := os.Getenv("AGENT")
	if agentType == "" {
		agentType = "mock"
	}

	var agentClient agent.Agent

	if agentType == "mock" {
		fmt.Println("Using Mock Agent")
		agentClient = &MockAgent{}
	} else {
		fmt.Printf("Using Real Agent: %s\n", agentType)

		// Map generic env vars to API key
		var apiKey string
		var model string // Default model?

		switch agentType {
		case "gemini", "gemini-cli":
			apiKey = os.Getenv("GEMINI_API_KEY")
			model = "gemini-1.5-flash" // Default to flash which is widely available
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
			model = "gpt-4-turbo"
		case "anthropic":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		case "ollama":
			model = "llama3"
		}

		// Just create it, if it fails validation later that's fine (smoke test purpose)
		var err error
		agentClient, err = agent.NewAgent(agentType, apiKey, model, "", "smoke-test-project")
		if err != nil {
			fmt.Printf("Failed to create agent %s: %v\n", agentType, err)
			os.Exit(1)
		}
	}

	// 3. Init Session
	session := runner.NewSession(dClient, agentClient, tmpDir, "recac-agent:latest", "smoke-test-project", 1)
	session.MaxIterations = 2 // Short run

	// 4. Run
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := session.Start(ctx); err != nil {
		fmt.Printf("Session Start Failed: %v\n", err)
		os.Exit(1)
	}

	if err := session.RunLoop(ctx); err != nil {
		// Context deadline exceeded is expected if we just run out of time/iterations
		if err != context.DeadlineExceeded {
			fmt.Printf("RunLoop Failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Smoke Test Complete.")
}
